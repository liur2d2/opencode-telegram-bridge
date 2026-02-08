package bridge

import (
	"context"
	"testing"

	"github.com/user/opencode-telegram/internal/opencode"
)

// Mock clients for sticker tests
type mockStickerOpenCodeClient struct {
	messages map[string][]string // sessionID -> messages
}

func (m *mockStickerOpenCodeClient) SendPrompt(sessionID string, text string, agent *string) (*opencode.SendPromptResponse, error) {
	if m.messages == nil {
		m.messages = make(map[string][]string)
	}
	m.messages[sessionID] = append(m.messages[sessionID], text)
	return nil, nil
}

type mockStickerTelegramBot struct {
	messages []string
}

func (m *mockStickerTelegramBot) SendMessage(ctx context.Context, text string) (int, error) {
	m.messages = append(m.messages, text)
	return len(m.messages) - 1, nil
}

type mockStickerAppState struct {
	currentSessionID string
}

func (m *mockStickerAppState) GetCurrentSession() string {
	return m.currentSessionID
}

func TestStickerWithEmojiAndSet(t *testing.T) {
	// Test sticker with both emoji and set name
	mockOC := &mockStickerOpenCodeClient{}
	mockTG := &mockStickerTelegramBot{}
	appState := &mockStickerAppState{currentSessionID: "sess123"}

	handler := NewStickerHandler(mockOC, mockTG, appState)

	err := handler.HandleSticker(context.Background(), "👍", "animals")
	if err != nil {
		t.Fatalf("HandleSticker failed: %v", err)
	}

	// Verify message was sent to OpenCode
	if len(mockOC.messages["sess123"]) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockOC.messages["sess123"]))
	}

	expected := "[Sticker: 👍 from set \"animals\"]"
	if mockOC.messages["sess123"][0] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, mockOC.messages["sess123"][0])
	}
}

func TestStickerWithEmojiOnly(t *testing.T) {
	// Test sticker with emoji only
	mockOC := &mockStickerOpenCodeClient{}
	mockTG := &mockStickerTelegramBot{}
	appState := &mockStickerAppState{currentSessionID: "sess456"}

	handler := NewStickerHandler(mockOC, mockTG, appState)

	err := handler.HandleSticker(context.Background(), "😀", "")
	if err != nil {
		t.Fatalf("HandleSticker failed: %v", err)
	}

	// Verify message was sent to OpenCode
	if len(mockOC.messages["sess456"]) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockOC.messages["sess456"]))
	}

	expected := "[Sticker: 😀]"
	if mockOC.messages["sess456"][0] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, mockOC.messages["sess456"][0])
	}
}

func TestStickerWithSetOnly(t *testing.T) {
	// Test sticker with set name only
	mockOC := &mockStickerOpenCodeClient{}
	mockTG := &mockStickerTelegramBot{}
	appState := &mockStickerAppState{currentSessionID: "sess789"}

	handler := NewStickerHandler(mockOC, mockTG, appState)

	err := handler.HandleSticker(context.Background(), "", "custom_pack")
	if err != nil {
		t.Fatalf("HandleSticker failed: %v", err)
	}

	// Verify message was sent to OpenCode
	if len(mockOC.messages["sess789"]) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockOC.messages["sess789"]))
	}

	expected := "[Sticker from set \"custom_pack\"]"
	if mockOC.messages["sess789"][0] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, mockOC.messages["sess789"][0])
	}
}

func TestStickerEmpty(t *testing.T) {
	// Test sticker with no emoji or set name
	mockOC := &mockStickerOpenCodeClient{}
	mockTG := &mockStickerTelegramBot{}
	appState := &mockStickerAppState{currentSessionID: "sess000"}

	handler := NewStickerHandler(mockOC, mockTG, appState)

	err := handler.HandleSticker(context.Background(), "", "")
	if err != nil {
		t.Fatalf("HandleSticker failed: %v", err)
	}

	// Verify message was sent to OpenCode
	if len(mockOC.messages["sess000"]) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockOC.messages["sess000"]))
	}

	expected := "[Sticker]"
	if mockOC.messages["sess000"][0] != expected {
		t.Errorf("Expected '%s', got '%s'", expected, mockOC.messages["sess000"][0])
	}
}

func TestStickerNoActiveSession(t *testing.T) {
	// Test sticker with no active session - should acknowledge but not crash
	mockOC := &mockStickerOpenCodeClient{}
	mockTG := &mockStickerTelegramBot{}
	appState := &mockStickerAppState{currentSessionID: ""}

	handler := NewStickerHandler(mockOC, mockTG, appState)

	err := handler.HandleSticker(context.Background(), "🎉", "party")
	if err != nil {
		t.Fatalf("HandleSticker failed: %v", err)
	}

	// Verify acknowledgement was sent to Telegram
	if len(mockTG.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockTG.messages))
	}

	if mockTG.messages[0] != "📌 Sticker received (no active session)" {
		t.Errorf("Expected acknowledgement message, got '%s'", mockTG.messages[0])
	}
}

func TestStickerAnimatedSkipped(t *testing.T) {
	// Test that animated stickers are rejected at bot.go level
	// This test verifies the handler itself doesn't break on any input
	mockOC := &mockStickerOpenCodeClient{}
	mockTG := &mockStickerTelegramBot{}
	appState := &mockStickerAppState{currentSessionID: "sess123"}

	handler := NewStickerHandler(mockOC, mockTG, appState)

	// Handler should work fine - rejection happens in bot.go RegisterStickerHandler
	err := handler.HandleSticker(context.Background(), "🎬", "animated_pack")
	if err != nil {
		t.Fatalf("HandleSticker failed: %v", err)
	}

	// Verify message was sent (handler doesn't know about animated flag)
	if len(mockOC.messages["sess123"]) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockOC.messages["sess123"]))
	}
}

func TestStickerVideoSkipped(t *testing.T) {
	// Test that video stickers are rejected at bot.go level
	// This test verifies the handler itself doesn't break on any input
	mockOC := &mockStickerOpenCodeClient{}
	mockTG := &mockStickerTelegramBot{}
	appState := &mockStickerAppState{currentSessionID: "sess456"}

	handler := NewStickerHandler(mockOC, mockTG, appState)

	// Handler should work fine - rejection happens in bot.go RegisterStickerHandler
	err := handler.HandleSticker(context.Background(), "🎥", "video_pack")
	if err != nil {
		t.Fatalf("HandleSticker failed: %v", err)
	}

	// Verify message was sent (handler doesn't know about video flag)
	if len(mockOC.messages["sess456"]) != 1 {
		t.Errorf("Expected 1 message, got %d", len(mockOC.messages["sess456"]))
	}
}
