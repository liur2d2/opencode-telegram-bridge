package bridge

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/user/opencode-telegram/internal/opencode"
)

type MockQuestionTelegramBot struct {
	mock.Mock
}

func (m *MockQuestionTelegramBot) SendMessage(ctx context.Context, text string) (int, error) {
	args := m.Called(ctx, text)
	return args.Get(0).(int), args.Error(1)
}

func (m *MockQuestionTelegramBot) SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error) {
	args := m.Called(ctx, text, keyboard)
	return args.Get(0).(int), args.Error(1)
}

func (m *MockQuestionTelegramBot) EditMessage(ctx context.Context, messageID int, text string) error {
	args := m.Called(ctx, messageID, text)
	return args.Error(0)
}

func (m *MockQuestionTelegramBot) AnswerCallback(ctx context.Context, callbackID string) error {
	args := m.Called(ctx, callbackID)
	return args.Error(0)
}

type MockQuestionOpenCodeClient struct {
	mock.Mock
}

func (m *MockQuestionOpenCodeClient) ReplyQuestion(requestID string, answers opencode.QuestionAnswer) error {
	args := m.Called(requestID, answers)
	return args.Error(0)
}

func (m *MockQuestionOpenCodeClient) RejectQuestion(requestID string) error {
	args := m.Called(requestID)
	return args.Error(0)
}

func TestHandleQuestionAsked_SingleSelect(t *testing.T) {
	mockTG := new(MockQuestionTelegramBot)
	mockOC := new(MockQuestionOpenCodeClient)

	handler := NewQuestionHandler(mockOC, mockTG)
	ctx := context.Background()

	questionRequest := opencode.QuestionRequest{
		ID:        "req_123",
		SessionID: "ses_456",
		Questions: []opencode.QuestionInfo{
			{
				Question: "Which option?",
				Header:   "Choose one",
				Options: []opencode.QuestionOption{
					{Label: "Option A", Description: "First option"},
					{Label: "Option B", Description: "Second option"},
				},
				Multiple: func() *bool { b := false; return &b }(),
				Custom:   func() *bool { b := false; return &b }(),
			},
		},
	}

	event := opencode.EventQuestionAsked{
		Properties: questionRequest,
	}

	mockTG.On("SendMessageWithKeyboard", ctx, mock.MatchedBy(func(text string) bool {
		return text != ""
	}), mock.AnythingOfType("*models.InlineKeyboardMarkup")).Return(1, nil)

	err := handler.HandleQuestionAsked(ctx, event)

	assert.NoError(t, err)
	mockTG.AssertExpectations(t)
}

func TestHandleQuestionAsked_MultiSelect(t *testing.T) {
	mockTG := new(MockQuestionTelegramBot)
	mockOC := new(MockQuestionOpenCodeClient)

	handler := NewQuestionHandler(mockOC, mockTG)
	ctx := context.Background()

	questionRequest := opencode.QuestionRequest{
		ID:        "req_234",
		SessionID: "ses_567",
		Questions: []opencode.QuestionInfo{
			{
				Question: "Select multiple?",
				Header:   "Choose many",
				Options: []opencode.QuestionOption{
					{Label: "Option A", Description: "First"},
					{Label: "Option B", Description: "Second"},
					{Label: "Option C", Description: "Third"},
				},
				Multiple: func() *bool { b := true; return &b }(),
				Custom:   func() *bool { b := false; return &b }(),
			},
		},
	}

	event := opencode.EventQuestionAsked{
		Properties: questionRequest,
	}

	mockTG.On("SendMessageWithKeyboard", ctx, mock.Anything, mock.AnythingOfType("*models.InlineKeyboardMarkup")).Return(1, nil)

	err := handler.HandleQuestionAsked(ctx, event)

	assert.NoError(t, err)
	mockTG.AssertExpectations(t)
}

func TestHandleQuestionCallback_SingleSelect(t *testing.T) {
	mockTG := new(MockQuestionTelegramBot)
	mockOC := new(MockQuestionOpenCodeClient)

	handler := NewQuestionHandler(mockOC, mockTG)
	ctx := context.Background()

	questionRequest := opencode.QuestionRequest{
		ID:        "req_345",
		SessionID: "ses_678",
		Questions: []opencode.QuestionInfo{
			{
				Question: "Choose",
				Options: []opencode.QuestionOption{
					{Label: "OptionX"},
					{Label: "OptionY"},
				},
				Multiple: func() *bool { b := false; return &b }(),
			},
		},
	}

	event := opencode.EventQuestionAsked{
		Properties: questionRequest,
	}

	mockTG.On("SendMessageWithKeyboard", ctx, mock.Anything, mock.AnythingOfType("*models.InlineKeyboardMarkup")).Return(1, nil)
	mockTG.On("AnswerCallback", ctx, mock.Anything).Return(nil)
	mockTG.On("EditMessage", ctx, 1, mock.Anything).Return(nil)
	mockOC.On("ReplyQuestion", "req_345", opencode.QuestionAnswer{"OptionX"}).Return(nil)

	handler.HandleQuestionAsked(ctx, event)

	callbackID := "test_callback"
	callbackData := "q:12345678:0"

	err := handler.HandleQuestionCallback(ctx, callbackID, callbackData)

	assert.NoError(t, err)
}

func TestHandleQuestionCallback_MultiSelect(t *testing.T) {
	mockTG := new(MockQuestionTelegramBot)
	mockOC := new(MockQuestionOpenCodeClient)

	handler := NewQuestionHandler(mockOC, mockTG)
	ctx := context.Background()

	questionRequest := opencode.QuestionRequest{
		ID:        "req_456",
		SessionID: "ses_789",
		Questions: []opencode.QuestionInfo{
			{
				Question: "Select many",
				Options: []opencode.QuestionOption{
					{Label: "A"},
					{Label: "B"},
					{Label: "C"},
				},
				Multiple: func() *bool { b := true; return &b }(),
			},
		},
	}

	event := opencode.EventQuestionAsked{
		Properties: questionRequest,
	}

	mockTG.On("SendMessageWithKeyboard", ctx, mock.Anything, mock.AnythingOfType("*models.InlineKeyboardMarkup")).Return(1, nil)
	mockTG.On("AnswerCallback", ctx, mock.Anything).Return(nil)
	mockTG.On("EditMessage", ctx, 1, mock.Anything).Return(nil)
	mockOC.On("ReplyQuestion", "req_456", opencode.QuestionAnswer{"A", "C"}).Return(nil)

	handler.HandleQuestionAsked(ctx, event)
}

func TestHandleQuestionCallback_CustomInput(t *testing.T) {
	mockTG := new(MockQuestionTelegramBot)
	mockOC := new(MockQuestionOpenCodeClient)

	handler := NewQuestionHandler(mockOC, mockTG)
	ctx := context.Background()

	questionRequest := opencode.QuestionRequest{
		ID:        "req_567",
		SessionID: "ses_890",
		Questions: []opencode.QuestionInfo{
			{
				Question: "Custom answer?",
				Custom:   func() *bool { b := true; return &b }(),
			},
		},
	}

	event := opencode.EventQuestionAsked{
		Properties: questionRequest,
	}

	mockTG.On("SendMessageWithKeyboard", ctx, mock.Anything, mock.AnythingOfType("*models.InlineKeyboardMarkup")).Return(1, nil)

	handler.HandleQuestionAsked(ctx, event)
}

func TestHandleQuestionAsked_MultipleQuestions(t *testing.T) {
	mockTG := new(MockQuestionTelegramBot)
	mockOC := new(MockQuestionOpenCodeClient)

	handler := NewQuestionHandler(mockOC, mockTG)
	ctx := context.Background()

	questionRequest := opencode.QuestionRequest{
		ID:        "req_678",
		SessionID: "ses_901",
		Questions: []opencode.QuestionInfo{
			{
				Question: "First question?",
				Options: []opencode.QuestionOption{
					{Label: "Q1A"},
					{Label: "Q1B"},
				},
			},
			{
				Question: "Second question?",
				Options: []opencode.QuestionOption{
					{Label: "Q2A"},
					{Label: "Q2B"},
					{Label: "Q2C"},
				},
			},
		},
	}

	event := opencode.EventQuestionAsked{
		Properties: questionRequest,
	}

	mockTG.On("SendMessageWithKeyboard", ctx, mock.Anything, mock.AnythingOfType("*models.InlineKeyboardMarkup")).Return(1, nil)
	mockTG.On("SendMessageWithKeyboard", ctx, mock.Anything, mock.AnythingOfType("*models.InlineKeyboardMarkup")).Return(2, nil)

	err := handler.HandleQuestionAsked(ctx, event)

	assert.NoError(t, err)
	mockTG.AssertNumberOfCalls(t, "SendMessageWithKeyboard", 2)
}

func TestQuestionKeyboardCleanup(t *testing.T) {
	mockTG := new(MockQuestionTelegramBot)
	mockOC := new(MockQuestionOpenCodeClient)

	handler := NewQuestionHandler(mockOC, mockTG)
	ctx := context.Background()

	questionRequest := opencode.QuestionRequest{
		ID:        "req_789",
		SessionID: "ses_012",
		Questions: []opencode.QuestionInfo{
			{
				Question: "Clean up?",
				Options: []opencode.QuestionOption{
					{Label: "Yes"},
				},
			},
		},
	}

	event := opencode.EventQuestionAsked{
		Properties: questionRequest,
	}

	mockTG.On("SendMessageWithKeyboard", ctx, mock.Anything, mock.AnythingOfType("*models.InlineKeyboardMarkup")).Return(1, nil)
	mockTG.On("AnswerCallback", ctx, mock.Anything).Return(nil)
	mockTG.On("EditMessage", ctx, 1, mock.MatchedBy(func(text string) bool {
		return text != "" && text != "⏳ Please type your answer..."
	})).Return(nil)
	mockOC.On("ReplyQuestion", "req_789", mock.Anything).Return(nil)

	handler.HandleQuestionAsked(ctx, event)
}
