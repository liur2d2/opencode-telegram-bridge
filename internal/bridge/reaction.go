package bridge

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot/models"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
)

// reactionOpenCodeClient interface for sending prompts
type reactionOpenCodeClient interface {
	SendPrompt(sessionID, text string, agent *string) (*opencode.SendPromptResponse, error)
}

// reactionTelegramBot interface for sending messages
type reactionTelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
}

// reactionAppState for app state access
type reactionAppState interface {
	GetCurrentSession() string
	GetSessionStatus(sessionID string) state.SessionStatus
}

// ReactionHandler manages incoming message reactions
type ReactionHandler struct {
	ocClient reactionOpenCodeClient
	tgBot    reactionTelegramBot
	appState reactionAppState
}

// NewReactionHandler creates a new ReactionHandler
func NewReactionHandler(ocClient reactionOpenCodeClient, tgBot reactionTelegramBot, appState reactionAppState) *ReactionHandler {
	return &ReactionHandler{
		ocClient: ocClient,
		tgBot:    tgBot,
		appState: appState,
	}
}

// HandleReaction processes incoming message reactions
func (h *ReactionHandler) HandleReaction(ctx context.Context, emoji string, messageID int) error {
	sessionID := h.appState.GetCurrentSession()

	// No active session—send acknowledgement
	if sessionID == "" {
		_, err := h.tgBot.SendMessage(ctx, fmt.Sprintf("👍 You reacted with %s to message #%d (no active session)", emoji, messageID))
		return err
	}

	// Format reaction text
	text := fmt.Sprintf("[User reacted with %s to message #%d]", emoji, messageID)

	// Send as regular prompt to current session
	_, err := h.ocClient.SendPrompt(sessionID, text, nil)
	return err
}

// ExtractEmojiFromReaction extracts emoji text from ReactionType array
func ExtractEmojiFromReaction(reactions []models.ReactionType) string {
	if len(reactions) == 0 {
		return ""
	}

	// Take first reaction emoji
	reaction := reactions[0]
	if reaction.ReactionTypeEmoji != nil {
		return reaction.ReactionTypeEmoji.Emoji
	}

	return ""
}
