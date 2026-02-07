package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
	"github.com/user/opencode-telegram/internal/telegram"
)

type TelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
	SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error)
	EditMessage(ctx context.Context, messageID int, text string) error
	AnswerCallback(ctx context.Context, callbackID string) error
	SendTyping(ctx context.Context) error
}

type OpenCodeClient interface {
	CreateSession(title *string, parentID *string) (*opencode.Session, error)
	ListSessions() ([]opencode.Session, error)
	SendPrompt(sessionID, text string, agent *string) (*opencode.SendPromptResponse, error)
	AbortSession(sessionID string) error
	Health() (map[string]interface{}, error)
	ReplyPermission(sessionID, permissionID string, response opencode.PermissionResponse) error
}

type PermissionState struct {
	PermissionID string
	SessionID    string
	MessageID    int
}

type Bridge struct {
	ocClient OpenCodeClient
	tgBot    TelegramBot
	state    *state.AppState
	registry *state.IDRegistry

	thinkingMsgs sync.Map
	permissions  sync.Map
	lastUpdate   sync.Map
	updateMu     sync.Mutex
}

func NewBridge(ocClient OpenCodeClient, tgBot TelegramBot, appState *state.AppState, registry *state.IDRegistry) *Bridge {
	return &Bridge{
		ocClient: ocClient,
		tgBot:    tgBot,
		state:    appState,
		registry: registry,
	}
}

func (b *Bridge) HandleUserMessage(ctx context.Context, text string) error {
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

	if b.state.GetSessionStatus(sessionID) == state.SessionBusy {
		_, err := b.tgBot.SendMessage(ctx, "⏳ Still processing your previous request...")
		return err
	}

	b.state.SetSessionStatus(sessionID, state.SessionBusy)

	thinkingMsgID, err := b.tgBot.SendMessage(ctx, "⏳ Processing...")
	if err != nil {
		b.state.SetSessionStatus(sessionID, state.SessionIdle)
		return fmt.Errorf("send thinking message: %w", err)
	}

	b.thinkingMsgs.Store(sessionID, thinkingMsgID)

	// Send initial typing indicator before launching async processing
	_ = b.tgBot.SendTyping(ctx)

	go b.sendPromptAsync(context.Background(), sessionID, text, thinkingMsgID)

	return nil
}

func (b *Bridge) sendPromptAsync(ctx context.Context, sessionID, text string, thinkingMsgID int) {
	agent := b.state.GetCurrentAgent()

	// Create done channel to signal typing goroutine to stop
	done := make(chan struct{})
	defer close(done)

	// Launch typing indicator goroutine that refreshes every 4 seconds
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = b.tgBot.SendTyping(context.Background())
			case <-done:
				return
			}
		}
	}()

	resp, err := b.ocClient.SendPrompt(sessionID, text, &agent)
	if err != nil {
		errorMsg := fmt.Sprintf("❌ Error: %s", err.Error())
		b.tgBot.EditMessage(ctx, thinkingMsgID, errorMsg)
		b.state.SetSessionStatus(sessionID, state.SessionError)
		return
	}

	responseText := b.extractResponseText(resp)

	chunks := telegram.SplitMessage(responseText, 4096)

	if len(chunks) > 0 {
		if err := b.tgBot.EditMessage(ctx, thinkingMsgID, chunks[0]); err != nil {
			b.tgBot.SendMessage(ctx, chunks[0])
		}

		for i := 1; i < len(chunks); i++ {
			b.tgBot.SendMessage(ctx, chunks[i])
		}
	}

	b.thinkingMsgs.Delete(sessionID)
	b.state.SetSessionStatus(sessionID, state.SessionIdle)
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
	switch event.Type {
	case "session.idle":
		b.handleSessionIdle(event)

	case "session.error":
		b.handleSessionError(event)

	case "question.asked":
		_ = event

	case "permission.asked":
		b.handlePermissionAsked(event)

	case "message.updated", "message.part.updated":
		b.handleMessageUpdated(event)
	}
}

func (b *Bridge) handleSessionIdle(event opencode.Event) {
	props, ok := event.Properties.(*struct {
		SessionID string `json:"sessionID"`
	})
	if !ok {
		return
	}

	b.state.SetSessionStatus(props.SessionID, state.SessionIdle)
}

func (b *Bridge) handleSessionError(event opencode.Event) {
	props, ok := event.Properties.(*struct {
		SessionID *string     `json:"sessionID,omitempty"`
		Error     interface{} `json:"error,omitempty"`
	})
	if !ok {
		return
	}

	errorMsg := "Unknown error"
	if props.Error != nil {
		if str, ok := props.Error.(string); ok {
			errorMsg = str
		} else {
			errorMsg = fmt.Sprintf("%v", props.Error)
		}
	}

	go func() {
		ctx := context.Background()
		msg := fmt.Sprintf("❌ Error: %s", errorMsg)
		b.tgBot.SendMessage(ctx, msg)

		if props.SessionID != nil {
			b.state.SetSessionStatus(*props.SessionID, state.SessionError)
		}
	}()
}

func (b *Bridge) handleMessageUpdated(event opencode.Event) {
	if event.Type != "message.updated" {
		return
	}

	msgEvent, ok := event.Properties.(*opencode.EventMessageUpdated)
	if !ok {
		return
	}

	// Extract message details
	msgData, ok := msgEvent.Properties.Message.(map[string]interface{})
	if !ok {
		return
	}

	sessionID, ok := msgData["sessionID"].(string)
	if !ok {
		return
	}

	// Extract text from parts
	parts, ok := msgData["parts"].([]interface{})
	if !ok {
		return
	}

	var textParts []string
	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if this is a text part
		if partType, ok := partMap["type"].(string); ok && partType == "text" {
			if text, ok := partMap["text"].(string); ok {
				textParts = append(textParts, text)
			}
		}
	}

	if len(textParts) == 0 {
		return
	}

	responseText := strings.Join(textParts, "\n")

	// Find the thinking message for this session
	thinkingMsgIDInterface, ok := b.thinkingMsgs.Load(sessionID)
	if !ok {
		return
	}

	thinkingMsgID, ok := thinkingMsgIDInterface.(int)
	if !ok {
		return
	}

	// Format and send the response
	go func() {
		ctx := context.Background()
		formattedText := telegram.FormatHTML(responseText)
		chunks := telegram.SplitMessage(formattedText, 4096)

		if len(chunks) > 0 {
			// Edit the thinking message with the first chunk
			if err := b.tgBot.EditMessage(ctx, thinkingMsgID, chunks[0]); err != nil {
				// Fallback: send as new message
				b.tgBot.SendMessage(ctx, chunks[0])
			}

			// Send remaining chunks as new messages
			for i := 1; i < len(chunks); i++ {
				b.tgBot.SendMessage(ctx, chunks[i])
			}
		}

		// Clean up
		b.thinkingMsgs.Delete(sessionID)
		b.state.SetSessionStatus(sessionID, state.SessionIdle)
	}()
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

func (b *Bridge) RegisterHandlers() {
	b.tgBot.(*telegram.Bot).RegisterTextHandler(func(ctx context.Context, text string) {
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

	b.tgBot.(*telegram.Bot).RegisterCommandHandler("abort", func(ctx context.Context, args string) {
		if err := cmdHandler.HandleAbortSession(ctx); err != nil {
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
}
