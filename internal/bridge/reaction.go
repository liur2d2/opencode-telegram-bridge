package bridge

import (
	"context"
	"fmt"
	"log"

	"github.com/go-telegram/bot/models"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
)

// ReactionHandler handles emoji reactions on bot messages
type ReactionHandler struct {
	ocClient  reactionOpenCodeClient
	tgBot     reactionTelegramBot
	appState  reactionAppState
}

// Interfaces for dependency injection
type reactionOpenCodeClient interface {
	SendPrompt(sessionID, text string, agent *string) (*opencode.SendPromptResponse, error)
}

type reactionTelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
}

type reactionAppState interface {
	GetCurrentSession() string
	GetSessionStatus(sessionID string) state.SessionStatus
}

// NewReactionHandler creates a new reaction handler
func NewReactionHandler(ocClient reactionOpenCodeClient, tgBot reactionTelegramBot, appState reactionAppState) *ReactionHandler {
	return &ReactionHandler{
		ocClient:  ocClient,
		tgBot:     tgBot,
		appState:  appState,
	}
}

// HandleReaction handles a reaction event
// emoji: the emoji reaction (e.g. "👍")
// messageID: the ID of the message being reacted to
func (h *ReactionHandler) HandleReaction(ctx context.Context, emoji string, messageID int) error {
	// Get current active session
	sessionID := h.appState.GetCurrentSession()
	if sessionID == "" {
		log.Printf("[REACTION] No active session, ignoring reaction")
		// Silently ignore - reactions are best-effort optional
		return nil
	}

	// Check session status
	status := h.appState.GetSessionStatus(sessionID)
	if status == state.SessionBusy {
		log.Printf("[REACTION] Session is busy, skipping reaction for message %d", messageID)
		return nil
	}

	// Format and send reaction as text to AI
	notificationText := fmt.Sprintf("[User reacted with %s to message #%d]", emoji, messageID)
	_, err := h.ocClient.SendPrompt(sessionID, notificationText, nil)
	if err != nil {
		log.Printf("[REACTION] Failed to forward reaction: %v", err)
		// Non-fatal: reactions are best-effort optional
		return nil
	}

	log.Printf("[REACTION] Forwarded reaction %s for message %d to session %s", emoji, messageID, sessionID)
	return nil
}

// ExtractEmojiFromReaction extracts emoji from a reaction array
// Returns empty string if no emoji found (e.g. custom emoji or paid reactions)
func ExtractEmojiFromReaction(reactions []models.ReactionType) string {
	if len(reactions) == 0 {
		return ""
	}

	reaction := reactions[0]

	// Only extract emoji type reactions, not custom or paid
	if reaction.Type == models.ReactionTypeTypeEmoji && reaction.ReactionTypeEmoji != nil {
		return reaction.ReactionTypeEmoji.Emoji
	}

	// Return empty for custom emoji or paid reactions
	return ""
}
