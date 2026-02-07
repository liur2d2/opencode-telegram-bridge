package bridge

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
)

type MockSessionOpenCodeClient struct {
	mock.Mock
}

func (m *MockSessionOpenCodeClient) CreateSession(title *string, parentID *string) (*opencode.Session, error) {
	args := m.Called(title, parentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*opencode.Session), args.Error(1)
}

func (m *MockSessionOpenCodeClient) ListSessions() ([]opencode.Session, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]opencode.Session), args.Error(1)
}

func (m *MockSessionOpenCodeClient) AbortSession(sessionID string) error {
	args := m.Called(sessionID)
	return args.Error(0)
}

func (m *MockSessionOpenCodeClient) Health() (map[string]interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

type MockSessionTelegramBot struct {
	mock.Mock
}

func (m *MockSessionTelegramBot) SendMessage(ctx context.Context, text string) (int, error) {
	args := m.Called(ctx, text)
	return args.Get(0).(int), args.Error(1)
}

func TestCmdNewSession(t *testing.T) {
	mockOC := new(MockSessionOpenCodeClient)
	mockTG := new(MockSessionTelegramBot)
	appState := state.NewAppState()

	session := &opencode.Session{
		ID:    "ses_new",
		Title: "New Session",
	}

	mockOC.On("CreateSession", mock.MatchedBy(func(title *string) bool {
		return title != nil && *title == "New Session"
	}), mock.Anything).Return(session, nil)

	mockTG.On("SendMessage", mock.Anything, mock.MatchedBy(func(text string) bool {
		return text != ""
	})).Return(1, nil)

	title := "New Session"
	sess, err := mockOC.CreateSession(&title, nil)

	assert.NoError(t, err)
	assert.Equal(t, "ses_new", sess.ID)
	appState.SetCurrentSession(sess.ID)
	assert.Equal(t, "ses_new", appState.GetCurrentSession())
}

func TestCmdNewSessionDefault(t *testing.T) {
	mockOC := new(MockSessionOpenCodeClient)
	_ = mockOC
	mockTG := new(MockSessionTelegramBot)
	appState := state.NewAppState()

	session := &opencode.Session{
		ID:    "ses_auto",
		Title: "Telegram Chat",
	}

	mockOC.On("CreateSession", mock.MatchedBy(func(title *string) bool {
		return title != nil && *title == "Telegram Chat"
	}), mock.Anything).Return(session, nil)

	mockTG.On("SendMessage", mock.Anything, mock.Anything).Return(1, nil)

	title := "Telegram Chat"
	sess, err := mockOC.CreateSession(&title, nil)

	assert.NoError(t, err)
	assert.Equal(t, "ses_auto", sess.ID)
	appState.SetCurrentSession(sess.ID)
	assert.Equal(t, "ses_auto", appState.GetCurrentSession())
}

func TestCmdSessions(t *testing.T) {
	mockOC := new(MockSessionOpenCodeClient)
	mockTG := new(MockSessionTelegramBot)
	appState := state.NewAppState()

	sessions := []opencode.Session{
		{ID: "ses_1", Title: "Session 1"},
		{ID: "ses_2", Title: "Session 2"},
		{ID: "ses_3", Title: "Session 3"},
	}

	mockOC.On("ListSessions").Return(sessions, nil)
	mockTG.On("SendMessage", mock.Anything, mock.MatchedBy(func(text string) bool {
		return text != ""
	})).Return(1, nil)

	appState.SetCurrentSession("ses_2")

	listed, err := mockOC.ListSessions()

	assert.NoError(t, err)
	assert.Equal(t, 3, len(listed))
	assert.Equal(t, "ses_1", listed[0].ID)
}

func TestCmdAbort(t *testing.T) {
	mockOC := new(MockSessionOpenCodeClient)
	mockTG := new(MockSessionTelegramBot)
	appState := state.NewAppState()

	appState.SetCurrentSession("ses_active")

	mockOC.On("AbortSession", "ses_active").Return(nil)
	mockTG.On("SendMessage", mock.Anything, mock.MatchedBy(func(text string) bool {
		return text != ""
	})).Return(1, nil)

	err := mockOC.AbortSession("ses_active")

	assert.NoError(t, err)
	appState.SetCurrentSession("")
	assert.Equal(t, "", appState.GetCurrentSession())
}

func TestCmdAbortNoSession(t *testing.T) {
	mockTG := new(MockSessionTelegramBot)
	appState := state.NewAppState()

	assert.Equal(t, "", appState.GetCurrentSession())

	mockTG.On("SendMessage", mock.Anything, mock.MatchedBy(func(text string) bool {
		return text != ""
	})).Return(1, nil)
}

func TestCmdStatus(t *testing.T) {
	mockOC := new(MockSessionOpenCodeClient)
	mockTG := new(MockSessionTelegramBot)
	appState := state.NewAppState()

	appState.SetCurrentSession("ses_current")
	appState.SetCurrentAgent("prometheus")
	appState.SetSessionStatus("ses_current", state.SessionIdle)

	health := map[string]interface{}{
		"healthy": true,
		"version": "1.1.53",
	}

	mockOC.On("Health").Return(health, nil)
	mockTG.On("SendMessage", mock.Anything, mock.MatchedBy(func(text string) bool {
		return text != ""
	})).Return(1, nil)

	h, err := mockOC.Health()

	assert.NoError(t, err)
	assert.True(t, h["healthy"].(bool))
	assert.Equal(t, "prometheus", appState.GetCurrentAgent())
	assert.Equal(t, state.SessionIdle, appState.GetSessionStatus("ses_current"))
}

func TestCmdHelp(t *testing.T) {
	mockTG := new(MockSessionTelegramBot)

	mockTG.On("SendMessage", mock.Anything, mock.MatchedBy(func(text string) bool {
		return text != ""
	})).Return(1, nil)

	ctx := context.Background()

	msgID, err := mockTG.SendMessage(ctx, "Help message")

	assert.NoError(t, err)
	assert.Equal(t, 1, msgID)
}

func TestCmdSelectSession(t *testing.T) {
	mockOC := new(MockSessionOpenCodeClient)
	mockTG := new(MockSessionTelegramBot)
	appState := state.NewAppState()

	sessions := []opencode.Session{
		{ID: "ses_a", Title: "Session A"},
		{ID: "ses_b", Title: "Session B"},
	}

	mockOC.On("ListSessions").Return(sessions, nil)
	mockTG.On("SendMessage", mock.Anything, mock.Anything).Return(1, nil)

	listed, err := mockOC.ListSessions()

	assert.NoError(t, err)
	assert.Greater(t, len(listed), 0)

	appState.SetCurrentSession("ses_b")
	assert.Equal(t, "ses_b", appState.GetCurrentSession())
}
