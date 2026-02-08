package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
)

type MockOpenCodeClient struct {
	mock.Mock
}

func (m *MockOpenCodeClient) CreateSession(title *string, parentID *string) (*opencode.Session, error) {
	args := m.Called(title, parentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*opencode.Session), args.Error(1)
}

func (m *MockOpenCodeClient) SendPrompt(sessionID, text string, agent *string) (*opencode.SendPromptResponse, error) {
	args := m.Called(sessionID, text, agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*opencode.SendPromptResponse), args.Error(1)
}

func (m *MockOpenCodeClient) SendPromptWithParts(sessionID string, parts []interface{}, agent *string) (*opencode.SendPromptResponse, error) {
	args := m.Called(sessionID, parts, agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*opencode.SendPromptResponse), args.Error(1)
}

func (m *MockOpenCodeClient) AbortSession(sessionID string) error {
	args := m.Called(sessionID)
	return args.Error(0)
}

func (m *MockOpenCodeClient) ListSessions() ([]opencode.Session, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]opencode.Session), args.Error(1)
}

func (m *MockOpenCodeClient) Health() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockOpenCodeClient) ReplyPermission(sessionID, permissionID string, response opencode.PermissionResponse) error {
	args := m.Called(sessionID, permissionID, response)
	return args.Error(0)
}

func (m *MockOpenCodeClient) ReplyQuestion(requestID string, answers []opencode.QuestionAnswer) error {
	args := m.Called(requestID, answers)
	return args.Error(0)
}

func (m *MockOpenCodeClient) GetConfig() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockOpenCodeClient) TriggerPrompt(sessionID, text string, agent *string) error {
	args := m.Called(sessionID, text, agent)
	return args.Error(0)
}

type MockTelegramBot struct {
	mock.Mock
	mu             sync.Mutex
	lastMessageID  int
	sentMessages   []string
	editedMessages map[int][]string
}

func NewMockTelegramBot() *MockTelegramBot {
	return &MockTelegramBot{
		editedMessages: make(map[int][]string),
	}
}

func (m *MockTelegramBot) SendMessage(ctx context.Context, text string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx, text)
	m.lastMessageID++
	m.sentMessages = append(m.sentMessages, text)
	return m.lastMessageID, args.Error(1)
}

func (m *MockTelegramBot) SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx, text, keyboard)
	m.lastMessageID++
	m.sentMessages = append(m.sentMessages, text)
	return m.lastMessageID, args.Error(1)
}

func (m *MockTelegramBot) EditMessage(ctx context.Context, messageID int, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx, messageID, text)
	m.editedMessages[messageID] = append(m.editedMessages[messageID], text)
	return args.Error(0)
}

func (m *MockTelegramBot) AnswerCallback(ctx context.Context, callbackID string) error {
	args := m.Called(ctx, callbackID)
	return args.Error(0)
}

func (m *MockTelegramBot) SendTyping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTelegramBot) SendMessagePlain(ctx context.Context, text string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx, text)
	m.lastMessageID++
	m.sentMessages = append(m.sentMessages, text)
	return m.lastMessageID, args.Error(1)
}

func (m *MockTelegramBot) EditMessagePlain(ctx context.Context, messageID int, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(ctx, messageID, text)
	m.editedMessages[messageID] = append(m.editedMessages[messageID], text)
	return args.Error(0)
}

func (m *MockTelegramBot) EditMessageKeyboard(ctx context.Context, messageID int, keyboard *models.InlineKeyboardMarkup) error {
	args := m.Called(ctx, messageID, keyboard)
	return args.Error(0)
}

func (m *MockTelegramBot) GetEditedMessages(messageID int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.editedMessages[messageID]
}

func TestBridgeHandleUserMessage_CreatesSessionIfNotExists(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppStateForTest()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)

	ctx := context.Background()

	session := &opencode.Session{
		ID:    "ses_123",
		Title: "Telegram Chat",
	}
	mockOC.On("CreateSession", mock.Anything, mock.Anything).Return(session, nil)
	mockOC.On("TriggerPrompt", "ses_123", "Hello", mock.Anything).Return(nil)
	mockTG.On("SendMessage", ctx, "⏳ Processing...").Return(1, nil)
	mockTG.On("SendTyping", ctx).Return(nil)
	mockTG.On("EditMessage", ctx, 1, mock.Anything).Return(nil)

	err := bridge.HandleUserMessage(ctx, "Hello")

	assert.NoError(t, err)
	assert.Equal(t, "ses_123", appState.GetCurrentSession())
}

func TestBridgeHandleUserMessage_BusySession(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppStateForTest()
	registry := state.NewIDRegistry()

	appState.SetCurrentSession("ses_123")
	appState.SetSessionStatus("ses_123", state.SessionBusy)

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)

	ctx := context.Background()

	mockTG.On("SendMessage", ctx, "⏳ Still processing your previous request...").Return(1, nil)

	err := bridge.HandleUserMessage(ctx, "Another message")

	assert.NoError(t, err)
	mockTG.AssertExpectations(t)
	mockOC.AssertNotCalled(t, "SendPrompt")
}

func TestBridgeHandleUserMessage_LongResponse(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppStateForTest()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	session := &opencode.Session{
		ID:    "ses_123",
		Title: "Telegram Chat",
	}

	longText := string(make([]byte, 5000))
	for i := 0; i < 5000; i++ {
		longText = longText[:i] + "a" + longText[i+1:]
	}

	mockOC.On("CreateSession", mock.Anything, mock.Anything).Return(session, nil)
	mockOC.On("TriggerPrompt", "ses_123", "Hello", mock.Anything).Return(nil)
	mockTG.On("SendMessage", ctx, "⏳ Processing...").Return(1, nil)
	mockTG.On("EditMessage", ctx, 1, mock.Anything).Return(nil)
	mockTG.On("SendTyping", ctx).Return(nil)
	mockTG.On("SendMessage", ctx, mock.Anything).Return(2, nil)

	err := bridge.HandleUserMessage(ctx, "Hello")

	assert.NoError(t, err)
	assert.Equal(t, "ses_123", appState.GetCurrentSession())
}

func TestBridgeHandleUserMessage_NoSession(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppStateForTest()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	session := &opencode.Session{
		ID:    "ses_new",
		Title: "Telegram Chat",
	}

	mockOC.On("CreateSession", mock.Anything, mock.Anything).Return(session, nil)
	mockOC.On("TriggerPrompt", "ses_new", "First message", mock.Anything).Return(nil)
	mockTG.On("SendMessage", ctx, "⏳ Processing...").Return(1, nil)
	mockTG.On("SendTyping", ctx).Return(nil)
	mockTG.On("EditMessage", ctx, 1, mock.Anything).Return(nil)

	assert.Equal(t, "", appState.GetCurrentSession())

	err := bridge.HandleUserMessage(ctx, "First message")

	assert.NoError(t, err)
	assert.Equal(t, "ses_new", appState.GetCurrentSession())
}

func TestBridgeHandleUserMessage_SessionError(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppStateForTest()
	registry := state.NewIDRegistry()

	appState.SetCurrentSession("ses_123")
	appState.SetSessionStatus("ses_123", state.SessionIdle)

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	mockOC.On("TriggerPrompt", "ses_123", "Hello", mock.Anything).Return(fmt.Errorf("connection failed"))
	mockTG.On("SendTyping", ctx).Return(nil)
	mockTG.On("SendMessage", ctx, "⏳ Processing...").Return(1, nil)
	mockTG.On("EditMessagePlain", mock.Anything, 1, mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, "Error") && strings.Contains(msg, "connection failed")
	})).Return(nil)

	err := bridge.HandleUserMessage(ctx, "Hello")

	assert.NoError(t, err)
}

func TestBridgeHandleSSEEvent_SessionIdle(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppStateForTest()
	registry := state.NewIDRegistry()

	appState.SetCurrentSession("ses_123")
	appState.SetSessionStatus("ses_123", state.SessionBusy)

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)

	event := opencode.Event{
		Type: "session.idle",
		Properties: &struct {
			SessionID string `json:"sessionID"`
		}{
			SessionID: "ses_123",
		},
	}

	bridge.HandleSSEEvent(event)

	assert.Equal(t, state.SessionIdle, appState.GetSessionStatus("ses_123"))
}

func TestBridgeHandleSSEEvent_SessionError(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppStateForTest()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	mockTG.On("SendMessage", ctx, mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, "Error") && strings.Contains(msg, "Something went wrong")
	})).Return(1, nil)

	errorMsg := "Something went wrong"
	sessionID := "ses_123"

	event := opencode.Event{
		Type: "session.error",
		Properties: &struct {
			SessionID *string     `json:"sessionID,omitempty"`
			Error     interface{} `json:"error,omitempty"`
		}{
			SessionID: &sessionID,
			Error:     errorMsg,
		},
	}

	bridge.HandleSSEEvent(event)
}

func TestBridgeThinkingIndicator(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppStateForTest()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	session := &opencode.Session{
		ID:    "ses_123",
		Title: "Telegram Chat",
	}

	mockOC.On("CreateSession", mock.Anything, mock.Anything).Return(session, nil)
	mockOC.On("TriggerPrompt", "ses_123", "Hello", mock.Anything).Return(nil)

	mockTG.On("SendMessage", ctx, "⏳ Processing...").Return(1, nil)
	mockTG.On("SendTyping", ctx).Return(nil)
	mockTG.On("EditMessage", ctx, 1, mock.Anything).Return(nil)

	err := bridge.HandleUserMessage(ctx, "Hello")

	assert.NoError(t, err)

	// Wait for debounce timer to fire (100ms + buffer)
	time.Sleep(150 * time.Millisecond)

	mockTG.AssertCalled(t, "SendMessage", ctx, "⏳ Processing...")
}
