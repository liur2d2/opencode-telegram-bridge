package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
)

type mockReactionOpenCodeClient struct {
	sentPrompts []string
}

func (m *mockReactionOpenCodeClient) SendPrompt(sessionID, text string, agent *string) (*opencode.SendPromptResponse, error) {
	m.sentPrompts = append(m.sentPrompts, text)
	return &opencode.SendPromptResponse{}, nil
}

type mockReactionTelegramBot struct {
	messages []string
}

func (m *mockReactionTelegramBot) SendMessage(ctx context.Context, text string) (int, error) {
	m.messages = append(m.messages, text)
	return len(m.messages) - 1, nil
}

type mockReactionAppState struct {
	currentSession string
	status         state.SessionStatus
}

func (m *mockReactionAppState) GetCurrentSession() string {
	return m.currentSession
}

func (m *mockReactionAppState) GetSessionStatus(sessionID string) state.SessionStatus {
	return m.status
}

func TestReactionForwardedToAI(t *testing.T) {
	ocClient := &mockReactionOpenCodeClient{}
	tgBot := &mockReactionTelegramBot{}
	appState := &mockReactionAppState{currentSession: "test-session-123"}
	handler := NewReactionHandler(ocClient, tgBot, appState)

	err := handler.HandleReaction(context.Background(), "👍", 42)
	assert.NoError(t, err)
	assert.Len(t, ocClient.sentPrompts, 1)
	assert.True(t, strings.Contains(ocClient.sentPrompts[0], "👍"))
	assert.True(t, strings.Contains(ocClient.sentPrompts[0], "42"))
}

func TestReactionNoSessionIgnored(t *testing.T) {
	ocClient := &mockReactionOpenCodeClient{}
	tgBot := &mockReactionTelegramBot{}
	appState := &mockReactionAppState{currentSession: ""}
	handler := NewReactionHandler(ocClient, tgBot, appState)

	err := handler.HandleReaction(context.Background(), "❤️", 42)
	assert.NoError(t, err)
	assert.Len(t, ocClient.sentPrompts, 0, "Should not send to AI if no session")
	// No acknowledgement sent in current implementation (silent ignore)
}

func TestReactionEmojiExtraction(t *testing.T) {
	reactions := []models.ReactionType{
		{
			Type: models.ReactionTypeTypeEmoji,
			ReactionTypeEmoji: &models.ReactionTypeEmoji{
				Type:  models.ReactionTypeTypeEmoji,
				Emoji: "😀",
			},
		},
	}

	emoji := ExtractEmojiFromReaction(reactions)
	assert.Equal(t, "😀", emoji)
}

func TestReactionEmojiExtractionEmpty(t *testing.T) {
	reactions := []models.ReactionType{}
	emoji := ExtractEmojiFromReaction(reactions)
	assert.Equal(t, "", emoji)
}

func TestReactionEmojiExtractionCustom(t *testing.T) {
	reactions := []models.ReactionType{
		{
			Type: models.ReactionTypeTypeCustomEmoji,
			ReactionTypeCustomEmoji: &models.ReactionTypeCustomEmoji{
				Type:          models.ReactionTypeTypeCustomEmoji,
				CustomEmojiID: "12345",
			},
		},
	}

	emoji := ExtractEmojiFromReaction(reactions)
	assert.Equal(t, "", emoji, "Should return empty string for custom emoji")
}
