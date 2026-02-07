package bridge

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
)

func TestHandlePermissionAskedSSE(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)

	event := opencode.Event{
		Type: "permission.asked",
		Properties: &opencode.EventPermissionAsked{
			Type: "permission.asked",
			Properties: opencode.PermissionRequest{
				ID:         "perm_123456",
				SessionID:  "ses_123",
				Permission: "fs.read",
				Patterns:   []string{"/path/to/file.txt"},
				Metadata: map[string]interface{}{
					"action": "read",
					"path":   "/path/to/file.txt",
					"size":   "1.2 KB",
				},
			},
		},
	}

	mockTG.On("SendMessageWithKeyboard", context.Background(), mock.MatchedBy(func(text string) bool {
		return len(text) > 0
	}), mock.Anything).Return(1, nil)

	bridge.handlePermissionAsked(event)

	mockTG.AssertCalled(t, "SendMessageWithKeyboard", context.Background(), mock.Anything, mock.Anything)
}

func TestHandlePermissionCallback(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	shortKey := registry.Register("perm_123456", "p", "")

	bridge.permissions.Store(shortKey, PermissionState{
		PermissionID: "perm_123456",
		SessionID:    "ses_123",
		MessageID:    42,
	})

	mockOC.On("ReplyPermission", "ses_123", "perm_123456", opencode.PermissionOnce).Return(nil)
	mockTG.On("EditMessage", ctx, 42, mock.Anything).Return(nil)

	err := bridge.HandlePermissionCallback(ctx, shortKey, "once")

	assert.NoError(t, err)
	mockOC.AssertCalled(t, "ReplyPermission", "ses_123", "perm_123456", opencode.PermissionOnce)
	mockTG.AssertCalled(t, "EditMessage", ctx, 42, mock.Anything)
}

func TestPermissionMessageContainsPermissionType(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)

	event := opencode.Event{
		Type: "permission.asked",
		Properties: &opencode.EventPermissionAsked{
			Type: "permission.asked",
			Properties: opencode.PermissionRequest{
				ID:         "perm_789",
				SessionID:  "ses_abc",
				Permission: "env.read",
				Patterns:   []string{"DATABASE_URL"},
				Metadata: map[string]interface{}{
					"env_var": "DATABASE_URL",
				},
			},
		},
	}

	var capturedMessage string
	mockTG.On("SendMessageWithKeyboard", context.Background(), mock.MatchedBy(func(text string) bool {
		capturedMessage = text
		return true
	}), mock.Anything).Return(1, nil)

	bridge.handlePermissionAsked(event)

	assert.Contains(t, capturedMessage, "🔐")
	assert.Contains(t, capturedMessage, "Permission Request")
	assert.Contains(t, capturedMessage, "env.read")
}

func TestPermissionAllowOnceResponse(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	shortKey := registry.Register("perm_123", "p", "")
	bridge.permissions.Store(shortKey, PermissionState{
		PermissionID: "perm_123",
		SessionID:    "ses_123",
		MessageID:    1,
	})

	mockOC.On("ReplyPermission", "ses_123", "perm_123", opencode.PermissionOnce).Return(nil)
	mockTG.On("EditMessage", ctx, 1, mock.MatchedBy(func(text string) bool {
		return true
	})).Return(nil)

	err := bridge.HandlePermissionCallback(ctx, shortKey, "once")

	assert.NoError(t, err)
	mockOC.AssertCalled(t, "ReplyPermission", "ses_123", "perm_123", opencode.PermissionOnce)
}

func TestPermissionAlwaysAllowResponse(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	shortKey := registry.Register("perm_456", "p", "")
	bridge.permissions.Store(shortKey, PermissionState{
		PermissionID: "perm_456",
		SessionID:    "ses_456",
		MessageID:    2,
	})

	mockOC.On("ReplyPermission", "ses_456", "perm_456", opencode.PermissionAlways).Return(nil)
	mockTG.On("EditMessage", ctx, 2, mock.MatchedBy(func(text string) bool {
		return true
	})).Return(nil)

	err := bridge.HandlePermissionCallback(ctx, shortKey, "always")

	assert.NoError(t, err)
	mockOC.AssertCalled(t, "ReplyPermission", "ses_456", "perm_456", opencode.PermissionAlways)
}

func TestPermissionRejectResponse(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	shortKey := registry.Register("perm_789", "p", "")
	bridge.permissions.Store(shortKey, PermissionState{
		PermissionID: "perm_789",
		SessionID:    "ses_789",
		MessageID:    3,
	})

	mockOC.On("ReplyPermission", "ses_789", "perm_789", opencode.PermissionReject).Return(nil)
	mockTG.On("EditMessage", ctx, 3, mock.MatchedBy(func(text string) bool {
		return true
	})).Return(nil)

	err := bridge.HandlePermissionCallback(ctx, shortKey, "reject")

	assert.NoError(t, err)
	mockOC.AssertCalled(t, "ReplyPermission", "ses_789", "perm_789", opencode.PermissionReject)
}

func TestPermissionKeyboardRemovedAfterResponse(t *testing.T) {
	mockOC := new(MockOpenCodeClient)
	mockTG := NewMockTelegramBot()
	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridge := NewBridge(mockOC, mockTG, appState, registry, 100*time.Millisecond)
	ctx := context.Background()

	shortKey := registry.Register("perm_999", "p", "")
	bridge.permissions.Store(shortKey, PermissionState{
		PermissionID: "perm_999",
		SessionID:    "ses_999",
		MessageID:    99,
	})

	mockOC.On("ReplyPermission", "ses_999", "perm_999", opencode.PermissionOnce).Return(nil)
	mockTG.On("EditMessage", ctx, 99, mock.MatchedBy(func(text string) bool {
		return true
	})).Return(nil)

	err := bridge.HandlePermissionCallback(ctx, shortKey, "once")

	assert.NoError(t, err)
	mockTG.AssertCalled(t, "EditMessage", ctx, 99, mock.Anything)
}
