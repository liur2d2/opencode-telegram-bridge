package telegram

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestNewBot(t *testing.T) {
	token := "test-token"
	chatID := int64(123456789)

	b := NewBot(token, chatID)

	if b == nil {
		t.Fatal("NewBot returned nil")
	}

	if b.chatID != chatID {
		t.Errorf("expected chatID %d, got %d", chatID, b.chatID)
	}
}

func TestSendMessage(t *testing.T) {
	t.Skip("Skipping test that requires real Telegram API - tested in integration")
}

func TestSendMessageSplit(t *testing.T) {
	t.Skip("Skipping test that requires real Telegram API - tested in integration")
}

func TestSendInlineKeyboard(t *testing.T) {
	t.Skip("Skipping test that requires real Telegram API - tested in integration")
}

func TestAnswerCallbackQuery(t *testing.T) {
	t.Skip("Skipping test that requires real Telegram API - tested in integration")
}

func TestEditMessage(t *testing.T) {
	t.Skip("Skipping test that requires real Telegram API - tested in integration")
}

func TestRegisterCommand(t *testing.T) {
	token := "test-token"
	chatID := int64(123456789)

	b := NewBot(token, chatID)

	handlerCalled := false
	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		handlerCalled = true
	}

	b.RegisterCommand("start", handler)

	if handlerCalled {
		t.Error("handler should not be called during registration")
	}
}

func TestRegisterCallbackPrefix(t *testing.T) {
	token := "test-token"
	chatID := int64(123456789)

	b := NewBot(token, chatID)

	handlerCalled := false
	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		handlerCalled = true
	}

	b.RegisterCallbackPrefix("q:", handler)

	if handlerCalled {
		t.Error("handler should not be called during registration")
	}
}
