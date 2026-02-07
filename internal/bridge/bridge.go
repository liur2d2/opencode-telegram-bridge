package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"

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
	fmt.Printf("[BRIDGE] HandleUserMessage called with text: '%s'\n", text)

	sessionID := b.state.GetCurrentSession()
	fmt.Printf("[BRIDGE] Current session: '%s'\n", sessionID)

	if sessionID == "" {
		fmt.Println("[BRIDGE] No session, creating new one...")
		title := "Telegram Chat"
		session, err := b.ocClient.CreateSession(&title, nil)
		if err != nil {
			fmt.Printf("[BRIDGE] CreateSession error: %v\n", err)
			return fmt.Errorf("create session: %w", err)
		}
		sessionID = session.ID
		fmt.Printf("[BRIDGE] Created session: %s\n", sessionID)
		b.state.SetCurrentSession(sessionID)
	}

	if b.state.GetSessionStatus(sessionID) == state.SessionBusy {
		fmt.Println("[BRIDGE] Session busy, sending wait message")
		waitMsg := telegram.FormatMarkdownV2("⏳ Still processing your previous request...")
		_, err := b.tgBot.SendMessage(ctx, waitMsg)
		return err
	}

	b.state.SetSessionStatus(sessionID, state.SessionBusy)
	fmt.Println("[BRIDGE] Sending thinking message...")

	thinkingMsg := telegram.FormatMarkdownV2("⏳ Processing...")
	thinkingMsgID, err := b.tgBot.SendMessage(ctx, thinkingMsg)
	if err != nil {
		fmt.Printf("[BRIDGE] SendMessage error: %v\n", err)
		b.state.SetSessionStatus(sessionID, state.SessionIdle)
		return fmt.Errorf("send thinking message: %w", err)
	}

	fmt.Printf("[BRIDGE] Thinking message sent, ID: %d\n", thinkingMsgID)
	b.thinkingMsgs.Store(sessionID, thinkingMsgID)

	fmt.Println("[BRIDGE] Launching async goroutine...")
	go b.sendPromptAsync(context.Background(), sessionID, text, thinkingMsgID)

	return nil
}

func (b *Bridge) sendPromptAsync(ctx context.Context, sessionID, text string, thinkingMsgID int) {
	fmt.Printf("[BRIDGE-ASYNC] Started for session %s, text: '%s'\n", sessionID, text)

	agent := b.state.GetCurrentAgent()
	fmt.Printf("[BRIDGE-ASYNC] Current agent: %s\n", agent)

	fmt.Printf("[BRIDGE-ASYNC] Calling SendPrompt...\n")
	resp, err := b.ocClient.SendPrompt(sessionID, text, &agent)
	if err != nil {
		fmt.Printf("[BRIDGE-ASYNC] SendPrompt error: %v\n", err)
		errorMsg := fmt.Sprintf("❌ Error: %s", err.Error())
		b.tgBot.EditMessage(ctx, thinkingMsgID, errorMsg)
		b.state.SetSessionStatus(sessionID, state.SessionError)
		return
	}

	fmt.Printf("[BRIDGE-ASYNC] SendPrompt success, extracting text...\n")
	responseText := b.extractResponseText(resp)
	fmt.Printf("[BRIDGE-ASYNC] Response text: %s\n", responseText[:min(100, len(responseText))])

	formattedText := telegram.FormatMarkdownV2(responseText)
	chunks := telegram.SplitMessage(formattedText, 4096)
	fmt.Printf("[BRIDGE-ASYNC] Split into %d chunks\n", len(chunks))

	if len(chunks) > 0 {
		fmt.Printf("[BRIDGE-ASYNC] Editing thinking message...\n")
		if err := b.tgBot.EditMessage(ctx, thinkingMsgID, chunks[0]); err != nil {
			fmt.Printf("[BRIDGE-ASYNC] Edit failed: %v, sending new message\n", err)
			b.tgBot.SendMessage(ctx, chunks[0])
		}

		for i := 1; i < len(chunks); i++ {
			b.tgBot.SendMessage(ctx, chunks[i])
		}
	}

	fmt.Println("[BRIDGE-ASYNC] Complete, cleaning up...")
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
	fmt.Printf("[BRIDGE] extractResponseText: Parts=%+v\n", resp.Parts)

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
	fmt.Printf("[BRIDGE] Extracted %d text parts, total length: %d\n", len(textParts), len(result))
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
	fmt.Printf("[BRIDGE] handleMessageUpdated called, event type: %s\n", event.Type)

	// The event could be EventMessageUpdated or EventMessagePartUpdated
	// For now, we'll focus on message.updated which should have the complete message
	if event.Type != "message.updated" {
		return // Ignore message.part.updated for now
	}

	msgEvent, ok := event.Properties.(*opencode.EventMessageUpdated)
	if !ok {
		fmt.Printf("[BRIDGE] Failed to cast event.Properties to EventMessageUpdated\n")
		return
	}

	fmt.Printf("[BRIDGE] Message event: %+v\n", msgEvent.Properties.Message)

	// Extract message details
	msgData, ok := msgEvent.Properties.Message.(map[string]interface{})
	if !ok {
		fmt.Printf("[BRIDGE] Message is not a map\n")
		return
	}

	sessionID, ok := msgData["sessionID"].(string)
	if !ok {
		fmt.Printf("[BRIDGE] No sessionID in message\n")
		return
	}

	// Extract text from parts
	parts, ok := msgData["parts"].([]interface{})
	if !ok {
		fmt.Printf("[BRIDGE] No parts in message\n")
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
		fmt.Printf("[BRIDGE] No text parts found in message\n")
		return
	}

	responseText := strings.Join(textParts, "\n")
	fmt.Printf("[BRIDGE] Extracted response text (%d chars)\n", len(responseText))

	// Find the thinking message for this session
	thinkingMsgIDInterface, ok := b.thinkingMsgs.Load(sessionID)
	if !ok {
		fmt.Printf("[BRIDGE] No thinking message found for session %s\n", sessionID)
		return
	}

	thinkingMsgID, ok := thinkingMsgIDInterface.(int)
	if !ok {
		fmt.Printf("[BRIDGE] Invalid thinking message ID type\n")
		return
	}

	// Format and send the response
	go func() {
		ctx := context.Background()
		formattedText := telegram.FormatMarkdownV2(responseText)
		chunks := telegram.SplitMessage(formattedText, 4096)

		fmt.Printf("[BRIDGE] Formatted response into %d chunks\n", len(chunks))

		if len(chunks) > 0 {
			// Edit the thinking message with the first chunk
			if err := b.tgBot.EditMessage(ctx, thinkingMsgID, chunks[0]); err != nil {
				fmt.Printf("[BRIDGE] Failed to edit thinking message: %v\n", err)
				// Fallback: send as new message
				b.tgBot.SendMessage(ctx, chunks[0])
			} else {
				fmt.Printf("[BRIDGE] Successfully edited thinking message\n")
			}

			// Send remaining chunks as new messages
			for i := 1; i < len(chunks); i++ {
				b.tgBot.SendMessage(ctx, chunks[i])
			}
		}

		// Clean up
		b.thinkingMsgs.Delete(sessionID)
		b.state.SetSessionStatus(sessionID, state.SessionIdle)
		fmt.Printf("[BRIDGE] Completed message update for session %s\n", sessionID)
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
		fmt.Printf("[HANDLER] Received text from Telegram: '%s'\n", text)
		if err := b.HandleUserMessage(ctx, text); err != nil {
			fmt.Printf("[HANDLER] Error from HandleUserMessage: %v\n", err)
			b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Error: %v", err))
		} else {
			fmt.Println("[HANDLER] HandleUserMessage returned success")
		}
	})
}
