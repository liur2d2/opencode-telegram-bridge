package bridge

import (
	"context"

	"github.com/go-telegram/bot/models"

	"github.com/user/opencode-telegram/internal/opencode"
)

type questionOpenCodeClient interface {
	ReplyQuestion(requestID string, answers opencode.QuestionAnswer) error
	RejectQuestion(requestID string) error
}

type questionTelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
	SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error)
	EditMessage(ctx context.Context, messageID int, text string) error
	AnswerCallback(ctx context.Context, callbackID string) error
}

type QuestionHandler struct {
	ocClient questionOpenCodeClient
	tgBot    questionTelegramBot
}

func NewQuestionHandler(ocClient questionOpenCodeClient, tgBot questionTelegramBot) *QuestionHandler {
	return &QuestionHandler{
		ocClient: ocClient,
		tgBot:    tgBot,
	}
}

func (h *QuestionHandler) HandleQuestionAsked(ctx context.Context, event opencode.EventQuestionAsked) error {
	props := event.Properties

	for range props.Questions {
		_, err := h.tgBot.SendMessageWithKeyboard(ctx, "Question message", nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *QuestionHandler) HandleQuestionCallback(ctx context.Context, callbackID, callbackData string) error {
	return nil
}
