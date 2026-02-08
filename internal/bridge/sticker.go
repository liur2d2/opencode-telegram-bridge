package bridge

import (
	"context"
	"fmt"

	"github.com/user/opencode-telegram/internal/opencode"
)

// stickerOpenCodeClient interface for sending prompts
type stickerOpenCodeClient interface {
	SendPrompt(sessionID, text string, agent *string) (*opencode.SendPromptResponse, error)
}

// stickerTelegramBot interface for sending messages
type stickerTelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
}

// stickerAppState interface for accessing session state
type stickerAppState interface {
	GetCurrentSession() string
}

// StickerHandler manages incoming sticker messages
type StickerHandler struct {
	ocClient stickerOpenCodeClient
	tgBot    stickerTelegramBot
	appState stickerAppState
}

// NewStickerHandler creates a new StickerHandler
func NewStickerHandler(ocClient stickerOpenCodeClient, tgBot stickerTelegramBot, appState stickerAppState) *StickerHandler {
	return &StickerHandler{
		ocClient: ocClient,
		tgBot:    tgBot,
		appState: appState,
	}
}

// HandleSticker processes incoming sticker messages
// Formats sticker as text description for AI
// Format: [Sticker: {emoji} from set "{setName}"]
// With fallbacks for missing emoji/setName
func (h *StickerHandler) HandleSticker(ctx context.Context, emoji string, setName string) error {
	// Build text description with fallbacks
	var text string

	if emoji != "" && setName != "" {
		text = fmt.Sprintf("[Sticker: %s from set \"%s\"]", emoji, setName)
	} else if emoji != "" {
		text = fmt.Sprintf("[Sticker: %s]", emoji)
	} else if setName != "" {
		text = fmt.Sprintf("[Sticker from set \"%s\"]", setName)
	} else {
		text = "[Sticker]"
	}

	// Send to current OpenCode session
	sessionID := h.appState.GetCurrentSession()
	if sessionID == "" {
		// No active session, just acknowledge
		_, err := h.tgBot.SendMessage(ctx, "📌 Sticker received (no active session)")
		return err
	}

	// Send to AI session
	_, err := h.ocClient.SendPrompt(sessionID, text, nil)
	if err != nil {
		return err
	}

	return nil
}
