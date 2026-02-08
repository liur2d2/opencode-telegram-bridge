package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/user/opencode-telegram/internal/state"
)

type mockModelTelegramBot struct {
	messages       []string
	keyboards      []*models.InlineKeyboardMarkup
	editedMessages map[int]string
}

func (m *mockModelTelegramBot) SendMessage(ctx context.Context, text string) (int, error) {
	m.messages = append(m.messages, text)
	return len(m.messages) - 1, nil
}

func (m *mockModelTelegramBot) SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error) {
	m.messages = append(m.messages, text)
	m.keyboards = append(m.keyboards, keyboard)
	return len(m.messages) - 1, nil
}

func (m *mockModelTelegramBot) EditMessage(ctx context.Context, msgID int, text string) error {
	if m.editedMessages == nil {
		m.editedMessages = make(map[int]string)
	}
	m.editedMessages[msgID] = text
	return nil
}

func (m *mockModelTelegramBot) AnswerCallback(ctx context.Context, callbackID string) error {
	return nil
}

type mockModelAppState struct {
	currentModel string
}

func (m *mockModelAppState) SetCurrentModel(model string) {
	m.currentModel = model
}

func (m *mockModelAppState) GetCurrentModel() string {
	return m.currentModel
}

func TestModelCommandShowsKeyboard(t *testing.T) {
	mockTG := &mockModelTelegramBot{}
	appState := &mockModelAppState{}
	registry := state.NewIDRegistry()
	handler := NewModelHandler(mockTG, appState, registry)
	err := handler.HandleModelCommand(context.Background())
	if err != nil {
		t.Fatalf("HandleModelCommand failed: %v", err)
	}
	if len(mockTG.messages) == 0 {
		t.Fatal("Expected message to be sent")
	}
	if !strings.Contains(mockTG.messages[0], "Select a Model") {
		t.Errorf("Expected message to contain 'Select a Model', got '%s'", mockTG.messages[0])
	}
	if len(mockTG.keyboards) == 0 {
		t.Fatal("Expected keyboard to be sent")
	}
}

func TestModelPaginationFirstPage(t *testing.T) {
	mockTG := &mockModelTelegramBot{}
	appState := &mockModelAppState{}
	registry := state.NewIDRegistry()
	handler := NewModelHandler(mockTG, appState, registry)
	err := handler.HandleModelCommand(context.Background())
	if err != nil {
		t.Fatalf("HandleModelCommand failed: %v", err)
	}
	msg := mockTG.messages[0]
	models := getDefaultModels()
	if len(models) > 8 {
		if !strings.Contains(msg, "Page 1") {
			t.Errorf("Expected message to contain 'Page 1', got '%s'", msg)
		}
	}
}

func TestModelPaginationLastPage(t *testing.T) {
	mockTG := &mockModelTelegramBot{}
	appState := &mockModelAppState{}
	registry := state.NewIDRegistry()
	handler := NewModelHandler(mockTG, appState, registry)
	err := handler.HandleModelCommand(context.Background())
	if err != nil {
		t.Fatalf("HandleModelCommand failed: %v", err)
	}
	models := getDefaultModels()
	if len(models) <= 8 {
		t.Skip("Skipping last page test - models list too small for pagination")
	}
	lastPageNum := (len(models) + 7) / 8 - 1
	pageNavData := buildPageNavData(lastPageNum)
	mockTG.messages = nil
	mockTG.editedMessages = make(map[int]string)
	err = handler.HandleModelCallback(context.Background(), 0, pageNavData)
	if err != nil {
		t.Fatalf("HandleModelCallback for last page failed: %v", err)
	}
	if len(mockTG.editedMessages) == 0 {
		t.Fatal("Expected message to be edited")
	}
}

func TestModelSelectionUpdatesState(t *testing.T) {
	mockTG := &mockModelTelegramBot{}
	appState := &mockModelAppState{}
	registry := state.NewIDRegistry()
	handler := NewModelHandler(mockTG, appState, registry)
	models := getDefaultModels()
	selectedModel := models[0]
	shortKey := registry.Register(selectedModel, "mdl", "sel")
	callbackData := "mdl:sel:" + shortKey
	err := handler.HandleModelCallback(context.Background(), 0, callbackData)
	if err != nil {
		t.Fatalf("HandleModelCallback failed: %v", err)
	}
	if appState.GetCurrentModel() != selectedModel {
		t.Errorf("Expected model '%s', got '%s'", selectedModel, appState.GetCurrentModel())
	}
	if len(mockTG.editedMessages) == 0 {
		t.Fatal("Expected confirmation message")
	}
	msg := mockTG.editedMessages[0]
	if !strings.Contains(msg, "Model set to") {
		t.Errorf("Expected confirmation message, got '%s'", msg)
	}
}

func TestModelCurrentHighlighted(t *testing.T) {
	mockTG := &mockModelTelegramBot{}
	appState := &mockModelAppState{}
	registry := state.NewIDRegistry()
	models := getDefaultModels()
	selectedModel := models[0]
	appState.SetCurrentModel(selectedModel)
	handler := NewModelHandler(mockTG, appState, registry)
	err := handler.HandleModelCommand(context.Background())
	if err != nil {
		t.Fatalf("HandleModelCommand failed: %v", err)
	}
	msg := mockTG.messages[0]
	if !strings.Contains(msg, "✅") {
		t.Errorf("Expected message to contain '✅' for current model, got '%s'", msg)
	}
}

func TestModelCallbackPageNavigation(t *testing.T) {
	mockTG := &mockModelTelegramBot{}
	appState := &mockModelAppState{}
	registry := state.NewIDRegistry()
	handler := NewModelHandler(mockTG, appState, registry)
	models := getDefaultModels()
	if len(models) <= 8 {
		t.Skip("Skipping pagination test - models list too small")
	}
	callbackData := "mdl:page:1"
	mockTG.editedMessages = make(map[int]string)
	err := handler.HandleModelCallback(context.Background(), 0, callbackData)
	if err != nil {
		t.Fatalf("HandleModelCallback for page navigation failed: %v", err)
	}
	if len(mockTG.editedMessages) == 0 {
		t.Fatal("Expected message to be edited for page navigation")
	}
	msg := mockTG.editedMessages[0]
	if !strings.Contains(msg, "Page 2") {
		t.Errorf("Expected message to show 'Page 2', got '%s'", msg)
	}
}

func buildPageNavData(pageNum int) string {
	return "mdl:page:" + formatPageNum(pageNum)
}

func formatPageNum(num int) string {
	if num < 10 {
		return "0" + string(rune('0'+num))
	}
	return string(rune('0'+num/10)) + string(rune('0'+num%10))
}
