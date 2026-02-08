package bridge

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/user/opencode-telegram/internal/health"
	"github.com/user/opencode-telegram/internal/metrics"
	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
	"github.com/user/opencode-telegram/internal/telegram"
)

type TelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
	SendMessagePlain(ctx context.Context, text string) (int, error)
	SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error)
	EditMessage(ctx context.Context, messageID int, text string) error
	EditMessagePlain(ctx context.Context, messageID int, text string) error
	AnswerCallback(ctx context.Context, callbackID string) error
	SendTyping(ctx context.Context) error
}

type OpenCodeClient interface {
	CreateSession(title *string, parentID *string) (*opencode.Session, error)
	ListSessions() ([]opencode.Session, error)
	DeleteSession(sessionID string) error
	SendPrompt(sessionID, text string, agent *string) (*opencode.SendPromptResponse, error)
	SendPromptWithParts(sessionID string, parts []interface{}, agent *string) (*opencode.SendPromptResponse, error)
	TriggerPrompt(sessionID, text string, agent *string) error
	AbortSession(sessionID string) error
	Health() (map[string]interface{}, error)
	GetConfig() (map[string]interface{}, error)
	GetMessages(sessionID string, limit int) ([]opencode.Message, error)
	ReplyPermission(sessionID, permissionID string, response opencode.PermissionResponse) error
	ReplyQuestion(requestID string, answers []opencode.QuestionAnswer) error
}

type PermissionState struct {
	PermissionID string
	SessionID    string
	MessageID    int
}

type QuestionState struct {
	RequestID       string
	SessionID       string
	MessageID       int
	QuestionIndex   int
	QuestionInfo    opencode.QuestionInfo
	SelectedOptions map[int]bool // For multi-select tracking
	WaitingCustom   bool         // True when waiting for custom text input
}

type DebounceBuffer struct {
	messages     []string
	lastReceived time.Time
	timer        *time.Timer
	mu           sync.Mutex
}

type StreamBuffer struct {
	text          string
	lastEdit      time.Time
	thinkingMsgID int
	mu            sync.Mutex
}

type MessageBuffer struct {
	text     string
	lastEdit time.Time
	mu       sync.Mutex
}

type Bridge struct {
	ocClient        OpenCodeClient
	tgBot           TelegramBot
	chatID          string
	state           *state.AppState
	registry        *state.IDRegistry
	debounceMs      time.Duration
	debounceBuffers sync.Map

	thinkingMsgs  sync.Map
	streamBuffers sync.Map
	msgBuffers    sync.Map
	permissions   sync.Map
	questions     sync.Map
	lastUpdate    sync.Map
	updateMu      sync.Mutex
	idleProcessed sync.Map

	healthMonitor *health.HealthMonitor
}

func NewBridge(ocClient OpenCodeClient, tgBot TelegramBot, appState *state.AppState, registry *state.IDRegistry, debounceMs time.Duration) *Bridge {
	if debounceMs <= 0 || debounceMs > 3000*time.Millisecond {
		debounceMs = 1000 * time.Millisecond
	}

	var chatID string
	if bot, ok := tgBot.(*telegram.Bot); ok {
		chatID = fmt.Sprintf("%d", bot.ChatID())
	}

	return &Bridge{
		ocClient:   ocClient,
		tgBot:      tgBot,
		chatID:     chatID,
		state:      appState,
		registry:   registry,
		debounceMs: debounceMs,
	}
}

func (b *Bridge) getEffectiveAgent() string {
	return b.state.GetAgentForChat(b.chatID)
}

func (b *Bridge) SetHealthMonitor(monitor *health.HealthMonitor) {
	b.healthMonitor = monitor
}

func (b *Bridge) HandleUserMessage(ctx context.Context, text string) error {
	sessionID := b.state.GetCurrentSession()
	log.Printf("[BRIDGE] HandleUserMessage: currentSession=%q, statePtr=%p", sessionID, b.state)

	if sessionID == "" {
		log.Printf("[BRIDGE] No session found, creating new one...")
		title := "Telegram Chat"
		session, err := b.ocClient.CreateSession(&title, nil)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		sessionID = session.ID
		b.state.SetCurrentSession(sessionID)
		log.Printf("[BRIDGE] Created and set session: %s", sessionID)
	}

	// Check if we have a buffer for this session
	bufVal, ok := b.debounceBuffers.Load(sessionID)
	if ok {
		// Buffer exists - add message and extend timer
		buf := bufVal.(*DebounceBuffer)
		buf.mu.Lock()
		buf.messages = append(buf.messages, text)
		buf.lastReceived = time.Now()
		buf.mu.Unlock()

		// Stop and restart timer
		if buf.timer != nil {
			buf.timer.Stop()
		}
		buf.timer = time.AfterFunc(b.debounceMs, func() {
			b.flushDebounceBuffer(sessionID)
		})
		return nil
	}

	// Check if session is busy
	if b.state.GetSessionStatus(sessionID) == state.SessionBusy {
		_, err := b.tgBot.SendMessage(ctx, "⏳ Still processing your previous request...")
		return err
	}

	// Session is idle - create first buffer and start debouncing
	buf := &DebounceBuffer{
		messages:     []string{text},
		lastReceived: time.Now(),
	}
	b.debounceBuffers.Store(sessionID, buf)
	buf.timer = time.AfterFunc(b.debounceMs, func() {
		b.flushDebounceBuffer(sessionID)
	})

	return nil
}

func (b *Bridge) flushDebounceBuffer(sessionID string) {
	bufVal, ok := b.debounceBuffers.Load(sessionID)
	if !ok {
		return
	}
	defer b.debounceBuffers.Delete(sessionID)

	buf := bufVal.(*DebounceBuffer)
	buf.mu.Lock()
	messages := buf.messages
	buf.mu.Unlock()

	if len(messages) == 0 {
		return
	}

	// Merge messages with newline separator
	mergedText := strings.Join(messages, "\n")

	// Check session status before sending
	if b.state.GetSessionStatus(sessionID) == state.SessionBusy {
		return
	}

	b.state.SetSessionStatus(sessionID, state.SessionBusy)

	ctx := context.Background()
	thinkingMsgID, err := b.tgBot.SendMessage(ctx, "⏳ Processing...")
	if err != nil {
		b.state.SetSessionStatus(sessionID, state.SessionIdle)
		return
	}

	b.thinkingMsgs.Store(sessionID, thinkingMsgID)

	// Send initial typing indicator before launching async processing
	_ = b.tgBot.SendTyping(ctx)

	go b.sendPromptAsync(context.Background(), sessionID, mergedText, thinkingMsgID)
}

func (b *Bridge) sendPromptAsync(ctx context.Context, sessionID, text string, thinkingMsgID int) {
	agent := b.getEffectiveAgent()

	go func() {
		err := b.ocClient.TriggerPrompt(sessionID, text, &agent)
		if err != nil {
			errorMsg := fmt.Sprintf("❌ Error: %s", err.Error())
			if editErr := b.tgBot.EditMessagePlain(context.Background(), thinkingMsgID, errorMsg); editErr != nil {
				log.Printf("[ERROR] Failed to edit error message: %v", editErr)
				b.tgBot.SendMessagePlain(context.Background(), errorMsg)
			}
			b.state.SetSessionStatus(sessionID, state.SessionError)
			b.thinkingMsgs.Delete(sessionID)
		}
	}()

	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				status := b.state.GetSessionStatus(sessionID)
				if status != state.SessionBusy {
					return
				}
				_ = b.tgBot.SendTyping(context.Background())
			}
		}
	}()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (b *Bridge) extractResponseText(resp *opencode.SendPromptResponse) string {
	var textParts []string
	for _, part := range resp.Parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}

		if text, ok := partMap["text"].(string); ok {
			textParts = append(textParts, text)
		}
	}

	result := strings.Join(textParts, "\n")
	return result
}

func (b *Bridge) HandleSSEEvent(event opencode.Event) {
	start := time.Now()
	defer func() {
		metrics.ObserveSSEEventProcessing(event.Type, start)
	}()

	if b.healthMonitor != nil {
		b.healthMonitor.RecordEvent(event.Type)
	}

	switch event.Type {
	case "session.idle":
		b.handleSessionIdle(event)

	case "session.error":
		b.handleSessionError(event)

	case "question.asked":
		if qaEvent, ok := event.Properties.(*opencode.EventQuestionAsked); ok {
			if err := b.handleQuestionAsked(*qaEvent); err != nil {
				b.tgBot.SendMessage(context.Background(), fmt.Sprintf("❌ Error handling question: %v", err))
			}
		}

	case "permission.asked":
		b.handlePermissionAsked(event)

	case "message.part.updated":
		b.handleMessagePartUpdated(event)

	case "message.updated":
		b.handleMessageUpdated(event)
	}
}

func (b *Bridge) handleSessionIdle(event opencode.Event) {
	evtData, ok := event.Properties.(*opencode.EventSessionIdle)
	if !ok {
		return
	}

	sessionID := evtData.Properties.SessionID
	b.state.SetSessionStatus(sessionID, state.SessionIdle)

	if evtData.Properties.Content != nil && *evtData.Properties.Content != "" {
		content := *evtData.Properties.Content

		// Deduplication: check if we already processed this session recently
		cacheKey := fmt.Sprintf("idle:%s", sessionID)
		if _, exists := b.idleProcessed.LoadOrStore(cacheKey, time.Now()); exists {
			log.Printf("[INFO] handleSessionIdle: skipping duplicate idle event for session %s", sessionID)
			return
		}

		// Clear cache after 5 seconds
		time.AfterFunc(5*time.Second, func() {
			b.idleProcessed.Delete(cacheKey)
		})

		log.Printf("[INFO] handleSessionIdle: sending response for session %s, content length=%d", sessionID, len(content))
		go b.sendCompletedMessageFromWebhook(sessionID, content)
	}
}

func (b *Bridge) handleSessionError(event opencode.Event) {
	evtData, ok := event.Properties.(*opencode.EventSessionError)
	if !ok {
		return
	}

	sessionID := ""
	if evtData.Properties.SessionID != nil {
		sessionID = *evtData.Properties.SessionID
	}

	b.state.SetSessionStatus(sessionID, state.SessionError)
}

func (b *Bridge) handleMessageUpdated(event opencode.Event) {
	if event.Type != "message.updated" {
		return
	}

	msgEvent, ok := event.Properties.(*opencode.EventMessageUpdated)
	if !ok {
		log.Printf("[WARN] handleMessageUpdated: failed to cast event properties")
		return
	}

	var sessionID string
	if msgEvent.Properties.Info != nil {
		sessionID = msgEvent.Properties.Info.SessionID

		if msgEvent.Properties.Info.Time.Completed != nil {
			b.state.SetSessionStatus(sessionID, state.SessionIdle)
			log.Printf("[INFO] handleMessageUpdated: message complete for session %s", sessionID)

			cacheKey := fmt.Sprintf("msg:%s:%s", sessionID, msgEvent.Properties.Info.ID)
			if _, exists := b.idleProcessed.LoadOrStore(cacheKey, time.Now()); exists {
				log.Printf("[INFO] handleMessageUpdated: skipping duplicate message for session %s", sessionID)
				return
			}

			time.AfterFunc(5*time.Second, func() {
				b.idleProcessed.Delete(cacheKey)
			})

			go b.fetchAndSendCompletedMessage(sessionID)
		}
	}
}

func (b *Bridge) fetchAndSendCompletedMessage(sessionID string) {
	messages, err := b.ocClient.GetMessages(sessionID, 10)
	if err != nil {
		log.Printf("[ERROR] fetchAndSendCompletedMessage: failed to get messages for session %s: %v", sessionID, err)
		return
	}

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Info.Role == "assistant" {
			var textParts []string
			for _, part := range msg.Parts {
				if part.Type == "text" && part.Text != "" {
					textParts = append(textParts, part.Text)
				}
			}

			if len(textParts) > 0 {
				content := strings.Join(textParts, "\n")
				log.Printf("[INFO] fetchAndSendCompletedMessage: sending response for session %s, content length=%d", sessionID, len(content))
				b.sendCompletedMessageFromWebhook(sessionID, content)
				return
			}
		}
	}

	log.Printf("[WARN] fetchAndSendCompletedMessage: no assistant message found for session %s", sessionID)
}

func (b *Bridge) sendCompletedMessageFromWebhook(sessionID string, content string) {
	ctx := context.Background()

	thinkingMsgIDInterface, ok := b.thinkingMsgs.Load(sessionID)
	if !ok {
		log.Printf("[INFO] sendCompletedMessageFromWebhook: creating new message for session %s", sessionID)
		formattedText := telegram.FormatHTML(content)
		chunks := telegram.SplitMessage(formattedText, 4096)

		for _, chunk := range chunks {
			if _, err := b.tgBot.SendMessage(ctx, chunk); err != nil {
				log.Printf("[ERROR] sendCompletedMessageFromWebhook: send failed: %v", err)
			}
		}
		return
	}

	thinkingMsgID := thinkingMsgIDInterface.(int)

	formattedText := telegram.FormatHTML(content)
	chunks := telegram.SplitMessage(formattedText, 4096)

	if len(chunks) > 0 {
		if err := b.tgBot.EditMessage(ctx, thinkingMsgID, chunks[0]); err != nil {
			log.Printf("[ERROR] sendCompletedMessageFromWebhook: edit failed: %v", err)
		}

		for i := 1; i < len(chunks); i++ {
			if _, err := b.tgBot.SendMessage(ctx, chunks[i]); err != nil {
				log.Printf("[ERROR] sendCompletedMessageFromWebhook: send chunk %d failed: %v", i, err)
			}
		}
	}

	b.thinkingMsgs.Delete(sessionID)
	log.Printf("[INFO] sendCompletedMessageFromWebhook: sent final message for session %s, content length=%d", sessionID, len(content))
}

func (b *Bridge) sendCompletedMessage(sessionID string) {
	ctx := context.Background()

	thinkingMsgIDInterface, ok := b.thinkingMsgs.Load(sessionID)
	if !ok {
		log.Printf("[INFO] sendCompletedMessage: no thinking message for session %s", sessionID)
		return
	}
	thinkingMsgID := thinkingMsgIDInterface.(int)

	bufInterface, ok := b.msgBuffers.Load(sessionID)
	if !ok {
		log.Printf("[WARN] sendCompletedMessage: no buffer for session %s", sessionID)
		b.tgBot.EditMessage(ctx, thinkingMsgID, "✅ Response completed (no content)")
		return
	}

	buf := bufInterface.(*MessageBuffer)
	buf.mu.Lock()
	finalText := buf.text
	buf.mu.Unlock()

	if finalText == "" {
		finalText = "✅ Response completed"
	}

	formattedText := telegram.FormatHTML(finalText)
	chunks := telegram.SplitMessage(formattedText, 4096)

	if len(chunks) > 0 {
		if err := b.tgBot.EditMessage(ctx, thinkingMsgID, chunks[0]); err != nil {
			log.Printf("[ERROR] sendCompletedMessage: edit failed: %v", err)
		}

		for i := 1; i < len(chunks); i++ {
			if _, err := b.tgBot.SendMessage(ctx, chunks[i]); err != nil {
				log.Printf("[ERROR] sendCompletedMessage: send chunk %d failed: %v", i, err)
			}
		}
	}

	b.msgBuffers.Delete(sessionID)
	b.thinkingMsgs.Delete(sessionID)
	log.Printf("[INFO] sendCompletedMessage: sent final message for session %s", sessionID)
}

func (b *Bridge) handleMessagePartUpdated(event opencode.Event) {
	partEvent, ok := event.Properties.(*opencode.EventMessagePartUpdated)
	if !ok {
		log.Printf("[WARN] handleMessagePartUpdated: failed to cast event properties")
		return
	}

	if partEvent.Properties.Delta == nil {
		log.Printf("[DEBUG] handleMessagePartUpdated: delta is nil")
		return
	}
	delta := *partEvent.Properties.Delta

	partData, ok := partEvent.Properties.Part.(map[string]interface{})
	if !ok {
		log.Printf("[WARN] handleMessagePartUpdated: part is not a map")
		return
	}

	sessionID, ok := partData["sessionID"].(string)
	if !ok {
		log.Printf("[WARN] handleMessagePartUpdated: sessionID not found in part")
		return
	}

	thinkingMsgIDInterface, ok := b.thinkingMsgs.Load(sessionID)
	if !ok {
		log.Printf("[WARN] handleMessagePartUpdated: no thinking message for session %s", sessionID)
		return
	}

	thinkingMsgID, ok := thinkingMsgIDInterface.(int)
	if !ok {
		log.Printf("[WARN] handleMessagePartUpdated: thinking message ID is not int")
		return
	}

	log.Printf("[DEBUG] handleMessagePartUpdated: session=%s, delta_len=%d, thinkingMsgID=%d", sessionID, len(delta), thinkingMsgID)

	// Get or create stream buffer
	bufInterface, _ := b.streamBuffers.LoadOrStore(sessionID, &StreamBuffer{
		text:          "",
		lastEdit:      time.Time{},
		thinkingMsgID: thinkingMsgID,
	})

	buf, ok := bufInterface.(*StreamBuffer)
	if !ok {
		return
	}

	// Accumulate delta
	buf.mu.Lock()
	buf.text += delta

	// Check if we should edit the message
	shouldEdit := time.Since(buf.lastEdit) > 500*time.Millisecond ||
		strings.HasSuffix(buf.text, "\n\n")

	if shouldEdit {
		// Copy current text for async edit
		textToSend := buf.text
		buf.lastEdit = time.Now()
		buf.mu.Unlock()

		// Edit message asynchronously
		go func() {
			ctx := context.Background()
			formattedText := telegram.FormatHTML(textToSend)
			chunks := telegram.SplitMessage(formattedText, 4096)

			if len(chunks) > 0 {
				// Edit with first chunk, silently ignore "message is not modified" errors
				_ = b.tgBot.EditMessage(ctx, thinkingMsgID, chunks[0])
			}
		}()
	} else {
		buf.mu.Unlock()
	}
}

func (b *Bridge) Start(ctx context.Context, sseConsumer *opencode.SSEConsumer) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sseConsumer.Events():
				if !ok {
					return
				}
				b.HandleSSEEvent(event)
			}
		}
	}()
}

func (b *Bridge) handlePermissionAsked(event opencode.Event) {
	permEvent, ok := event.Properties.(*opencode.EventPermissionAsked)
	if !ok {
		return
	}

	props := permEvent.Properties

	shortKey := b.registry.Register(props.ID, "p", "")

	msgContent := fmt.Sprintf(
		"🔐 **Permission Request**\n\n"+
			"**Permission:** %s\n"+
			"**Patterns:** %s",
		props.Permission,
		strings.Join(props.Patterns, ", "),
	)

	if len(props.Metadata) > 0 {
		msgContent += "\n\n**Details:**"
		for key, value := range props.Metadata {
			msgContent += fmt.Sprintf("\n• %s: %v", key, value)
		}
	}

	keyboard := telegram.BuildPermissionKeyboard(shortKey)

	ctx := context.Background()
	msgID, err := b.tgBot.SendMessageWithKeyboard(ctx, msgContent, keyboard)
	if err != nil {
		return
	}

	b.permissions.Store(shortKey, PermissionState{
		PermissionID: props.ID,
		SessionID:    props.SessionID,
		MessageID:    msgID,
	})
}

func (b *Bridge) HandlePermissionCallback(ctx context.Context, shortKey string, response string) error {
	stateVal, ok := b.permissions.Load(shortKey)
	if !ok {
		return fmt.Errorf("permission state not found for key: %s", shortKey)
	}

	permState := stateVal.(PermissionState)

	var permResponse opencode.PermissionResponse
	switch response {
	case "once":
		permResponse = opencode.PermissionOnce
	case "always":
		permResponse = opencode.PermissionAlways
	case "reject":
		permResponse = opencode.PermissionReject
	default:
		return fmt.Errorf("invalid permission response: %s", response)
	}

	err := b.ocClient.ReplyPermission(permState.SessionID, permState.PermissionID, permResponse)
	if err != nil {
		return fmt.Errorf("reply permission: %w", err)
	}

	var statusMsg string
	switch permResponse {
	case opencode.PermissionOnce:
		statusMsg = "✅ Allowed (once)"
	case opencode.PermissionAlways:
		statusMsg = "✅ Allowed (always)"
	case opencode.PermissionReject:
		statusMsg = "❌ Rejected"
	}

	editedMsg := fmt.Sprintf("🔐 **Permission Request**\n\n%s", statusMsg)
	err = b.tgBot.EditMessage(ctx, permState.MessageID, editedMsg)
	if err != nil {
		return fmt.Errorf("edit message: %w", err)
	}

	b.permissions.Delete(shortKey)

	return nil
}

// HandlePhotoMessage handles photo messages with vision API integration
func (b *Bridge) HandlePhotoMessage(ctx context.Context, photos []models.PhotoSize, caption string, botToken string) error {
	sessionID := b.state.GetCurrentSession()

	if sessionID == "" {
		title := "Telegram Chat"
		session, err := b.ocClient.CreateSession(&title, nil)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		sessionID = session.ID
		b.state.SetCurrentSession(sessionID)
	}

	// Check if session is busy
	if b.state.GetSessionStatus(sessionID) == state.SessionBusy {
		_, err := b.tgBot.SendMessage(ctx, "⏳ Still processing your previous request...")
		return err
	}

	b.state.SetSessionStatus(sessionID, state.SessionBusy)

	thinkingMsgID, err := b.tgBot.SendMessage(ctx, "🖼️ Processing image...")
	if err != nil {
		b.state.SetSessionStatus(sessionID, state.SessionIdle)
		return err
	}

	b.thinkingMsgs.Store(sessionID, thinkingMsgID)
	_ = b.tgBot.SendTyping(ctx)

	go b.sendPhotoPromptAsync(context.Background(), sessionID, photos, caption, botToken, thinkingMsgID)
	return nil
}

func (b *Bridge) sendPhotoPromptAsync(ctx context.Context, sessionID string, photos []models.PhotoSize, caption string, botToken string, thinkingMsgID int) {
	agent := b.state.GetCurrentAgent()

	largestPhoto := telegram.GetLargestPhoto(photos)
	if largestPhoto == nil {
		errorMsg := "❌ Error: No valid photo found"
		if editErr := b.tgBot.EditMessagePlain(context.Background(), thinkingMsgID, errorMsg); editErr != nil {
			log.Printf("[ERROR] Failed to edit error message: %v", editErr)
			b.tgBot.SendMessagePlain(context.Background(), errorMsg)
		}
		b.state.SetSessionStatus(sessionID, state.SessionError)
		b.thinkingMsgs.Delete(sessionID)
		return
	}

	photoData, err := telegram.DownloadPhoto(ctx, botToken, largestPhoto.FileID)
	if err != nil {
		errorMsg := fmt.Sprintf("❌ Error downloading image: %s", err.Error())
		if editErr := b.tgBot.EditMessagePlain(context.Background(), thinkingMsgID, errorMsg); editErr != nil {
			log.Printf("[ERROR] Failed to edit error message: %v", editErr)
			b.tgBot.SendMessagePlain(context.Background(), errorMsg)
		}
		b.state.SetSessionStatus(sessionID, state.SessionError)
		b.thinkingMsgs.Delete(sessionID)
		return
	}

	base64Image := telegram.EncodeBase64(photoData)

	parts := []interface{}{
		opencode.ImagePartInput{
			Type:     "image",
			Image:    base64Image,
			MimeType: "image/jpeg",
		},
	}

	if caption != "" {
		parts = append(parts, opencode.TextPartInput{
			Type: "text",
			Text: caption,
		})
	}

	go func() {
		_, err := b.ocClient.SendPromptWithParts(sessionID, parts, &agent)
		if err != nil {
			errorMsg := fmt.Sprintf("❌ Error: %s", err.Error())
			if editErr := b.tgBot.EditMessagePlain(context.Background(), thinkingMsgID, errorMsg); editErr != nil {
				log.Printf("[ERROR] Failed to edit error message: %v", editErr)
				b.tgBot.SendMessagePlain(context.Background(), errorMsg)
			}
			b.state.SetSessionStatus(sessionID, state.SessionError)
			b.thinkingMsgs.Delete(sessionID)
		}
	}()

	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				status := b.state.GetSessionStatus(sessionID)
				if status != state.SessionBusy {
					return
				}
				_ = b.tgBot.SendTyping(context.Background())
			}
		}
	}()
}

// HandleUnsupportedMedia handles unsupported media types
func (b *Bridge) HandleUnsupportedMedia(ctx context.Context) error {
	_, err := b.tgBot.SendMessage(ctx, "⚠️ 目前僅支援圖片分析,其他媒體類型暫不支援")
	return err
}

func (b *Bridge) HandleReaction(ctx context.Context, messageID int, userID int64, newReaction []models.ReactionType) error {
	sessionID := b.state.GetCurrentSession()
	if sessionID == "" {
		return nil
	}

	reactionStr := ""
	if len(newReaction) > 0 {
		reaction := newReaction[0]
		if reaction.ReactionTypeEmoji != nil {
			reactionStr = reaction.ReactionTypeEmoji.Emoji
		}
	}

	if reactionStr == "" {
		return nil
	}

	notificationText := fmt.Sprintf("[User reacted with %s to your previous response]", reactionStr)
	agent := b.state.GetCurrentAgent()
	_, err := b.ocClient.SendPrompt(sessionID, notificationText, &agent)
	return err
}

func (b *Bridge) RegisterHandlers() {
	b.tgBot.(*telegram.Bot).RegisterTextHandler(func(ctx context.Context, text string) {
		if b.HandleQuestionCustomInput(ctx, text) {
			return
		}
		if err := b.HandleUserMessage(ctx, text); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	cmdHandler := NewCommandHandler(b.ocClient, b.tgBot, b.state)

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("newsession", func(ctx context.Context, args string) {
		var title *string
		if args != "" {
			title = &args
		}
		if err := cmdHandler.HandleNewSession(ctx, title); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("sessions", func(ctx context.Context, args string) {
		if err := cmdHandler.HandleListSessions(ctx); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("session", func(ctx context.Context, args string) {
		sessionID := strings.TrimSpace(args)
		if sessionID == "" {
			b.tgBot.SendMessage(ctx, "❌ Please provide a session ID: /session <id>")
			return
		}
		if err := cmdHandler.HandleSwitchSession(ctx, sessionID); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("selectsession", func(ctx context.Context, args string) {
		if err := cmdHandler.HandleSelectSession(ctx); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("abort", func(ctx context.Context, args string) {
		if err := cmdHandler.HandleAbortSession(ctx); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("deletesession", func(ctx context.Context, args string) {
		sessionID := strings.TrimSpace(args)
		if sessionID == "" {
			if err := cmdHandler.HandleDeleteSessionMenu(ctx); err != nil {
				b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
			}
			return
		}
		if err := cmdHandler.HandleDeleteSession(ctx, sessionID); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("deletesessions", func(ctx context.Context, args string) {
		if err := cmdHandler.HandleDeleteSessionMenu(ctx); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("status", func(ctx context.Context, args string) {
		if err := cmdHandler.HandleStatus(ctx); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("help", func(ctx context.Context, args string) {
		if err := cmdHandler.HandleHelp(ctx); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("switch", func(ctx context.Context, args string) {
		var agent *string
		if args != "" {
			agent = &args
		}
		agents, _ := b.ocClient.(interface{ GetAgents() ([]string, error) })
		if getter, ok := agents.(interface{ GetAgents() ([]string, error) }); ok {
			availableAgents, _ := getter.GetAgents()
			if agent != nil && *agent != "" {
				valid := false
				for _, a := range availableAgents {
					if a == *agent {
						valid = true
						break
					}
				}
				if valid {
					b.state.SetCurrentAgent(*agent)
					b.tgBot.SendMessage(ctx, fmt.Sprintf("🔄 Switched to %s", *agent))
				} else {
					msg := fmt.Sprintf("❌ Unknown agent: %s\n\nAvailable agents:\n", *agent)
					for _, a := range availableAgents {
						msg += fmt.Sprintf("• %s\n", a)
					}
					b.tgBot.SendMessage(ctx, msg)
				}
			} else {
				msg := "🤖 Select an OHO Agent:\n\n"
				for i, a := range availableAgents {
					msg += fmt.Sprintf("%d. %s\n", i+1, a)
				}
				b.tgBot.SendMessage(ctx, msg)
			}
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("agent:", func(ctx context.Context, callbackID string, data string) {
		agentName := strings.TrimPrefix(data, "agent:")
		b.state.SetCurrentAgent(agentName)
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	modelHandler := NewModelHandler(b.tgBot, b.state)
	b.tgBot.(*telegram.Bot).RegisterCommandHandler("model", func(ctx context.Context, args string) {
		if err := modelHandler.HandleModelCommand(ctx); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("mdl:", func(ctx context.Context, callbackID string, data string) {
		if err := modelHandler.HandleModelCallback(ctx, 0, data); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("sess:", func(ctx context.Context, callbackID string, data string) {
		sessionID := strings.TrimPrefix(data, "sess:")
		if err := cmdHandler.HandleSwitchSession(ctx, sessionID); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("sesspage:", func(ctx context.Context, callbackID string, data string) {
		pageStr := strings.TrimPrefix(data, "sesspage:")
		page := 0
		fmt.Sscanf(pageStr, "%d", &page)
		if err := cmdHandler.HandleSessionPageCallback(ctx, page); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("del:", func(ctx context.Context, callbackID string, data string) {
		sessionID := strings.TrimPrefix(data, "del:")
		if err := cmdHandler.HandleDeleteConfirmCallback(ctx, sessionID); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("delpage:", func(ctx context.Context, callbackID string, data string) {
		pageStr := strings.TrimPrefix(data, "delpage:")
		page := 0
		fmt.Sscanf(pageStr, "%d", &page)
		if err := cmdHandler.HandleDeleteSessionPageCallback(ctx, page); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("delconfirm:", func(ctx context.Context, callbackID string, data string) {
		sessionID := strings.TrimPrefix(data, "delconfirm:")
		if err := cmdHandler.HandleDeleteExecuteCallback(ctx, sessionID); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("delcancel", func(ctx context.Context, callbackID string, data string) {
		b.tgBot.SendMessage(ctx, "❌ Deletion cancelled")
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	b.tgBot.(*telegram.Bot).RegisterCallbackHandler("q:", func(ctx context.Context, callbackID string, data string) {
		parts := strings.SplitN(data, ":", 3)
		if len(parts) < 3 {
			return
		}
		shortKey := parts[1]
		action := strings.Join(parts[2:], ":")

		if err := b.HandleQuestionCallback(ctx, shortKey, action); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
		b.tgBot.AnswerCallback(ctx, callbackID)
	})

	b.tgBot.(*telegram.Bot).RegisterPhotoHandler(func(ctx context.Context, photos []models.PhotoSize, caption string, botToken string) {
		if err := b.HandlePhotoMessage(ctx, photos, caption, botToken); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	stickerHandler := NewStickerHandler(b.ocClient, b.tgBot, b.state)
	b.tgBot.(*telegram.Bot).RegisterStickerHandler(func(ctx context.Context, emoji string, setName string) {
		if err := stickerHandler.HandleSticker(ctx, emoji, setName); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterUnsupportedMediaHandler(func(ctx context.Context) {
		if err := b.HandleUnsupportedMedia(ctx); err != nil {
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		}
	})

	b.tgBot.(*telegram.Bot).RegisterReactionHandler(func(ctx context.Context, messageID int, userID int64, newReaction []models.ReactionType) {
		if err := b.HandleReaction(ctx, messageID, userID, newReaction); err != nil {
			fmt.Printf("[BRIDGE] Error handling reaction: %v\n", err)
		}
	})

	routingHandler := NewRoutingHandler(b.state, b.tgBot)
	b.tgBot.(*telegram.Bot).RegisterCommandHandler("route", func(ctx context.Context, args string) {
		routingHandler.HandleRouteCommand(ctx, b.chatID, args)
	})

}
