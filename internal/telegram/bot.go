package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Bot wraps the Telegram bot client
type Bot struct {
	bot    *bot.Bot
	chatID int64
}

// NewBot creates a new Telegram bot instance
func NewBot(token string, chatID int64) *Bot {
	opts := []bot.Option{
		bot.WithSkipGetMe(), // Skip validation call to allow testing
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		// In production, we'd return the error, but for now we'll panic
		// since the bot is critical for operation
		panic(fmt.Sprintf("failed to create bot: %v", err))
	}

	return &Bot{
		bot:    b,
		chatID: chatID,
	}
}

// SendMessage sends a message to the configured chat
// Returns the message ID for later editing
// NOTE: Does NOT automatically split - caller must handle long messages
func (b *Bot) SendMessage(ctx context.Context, text string) (int, error) {
	msg, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    b.chatID,
		Text:      text,
		ParseMode: "MarkdownV2",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to send message: %w", err)
	}

	return msg.ID, nil
}

func (b *Bot) SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error) {
	msg, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      b.chatID,
		Text:        text,
		ReplyMarkup: keyboard,
		ParseMode:   "MarkdownV2",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to send message with keyboard: %w", err)
	}

	return msg.ID, nil
}

// EditMessage edits an existing message
func (b *Bot) EditMessage(ctx context.Context, messageID int, text string) error {
	_, err := b.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    b.chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: "MarkdownV2",
	})
	if err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

// AnswerCallback answers a callback query
func (b *Bot) AnswerCallback(ctx context.Context, callbackID string) error {
	_, err := b.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
	})
	if err != nil {
		return fmt.Errorf("failed to answer callback: %w", err)
	}

	return nil
}

// RegisterCommand registers a command handler
func (b *Bot) RegisterCommand(command string, handler bot.HandlerFunc) {
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/"+command, bot.MatchTypePrefix, handler)
}

// RegisterCallbackPrefix registers a callback query handler for a specific prefix
func (b *Bot) RegisterCallbackPrefix(prefix string, handler bot.HandlerFunc) {
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, prefix, bot.MatchTypePrefix, handler)
}

// Start starts the bot with long polling
func (b *Bot) Start(ctx context.Context) {
	b.bot.Start(ctx)
}
