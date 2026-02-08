package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/user/opencode-telegram/internal/metrics"
)

// Bot wraps the Telegram bot client
type Bot struct {
	bot            *bot.Bot
	chatID         int64
	token          string
	offset         int64
	offsetFilePath string
	maxUpdateID    int64
	offsetMu       sync.Mutex
}

// NewBot creates a new Telegram bot instance with optional initial offset
func NewBot(token string, chatID int64, initialOffset int64) *Bot {
	opts := []bot.Option{
		bot.WithSkipGetMe(),
		bot.WithInitialOffset(initialOffset),
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			models.AllowedUpdateMessage,
			models.AllowedUpdateCallbackQuery,
			models.AllowedUpdateMessageReaction,
		}),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create bot: %v", err))
	}

	return &Bot{
		bot:         b,
		chatID:      chatID,
		token:       token,
		offset:      initialOffset,
		maxUpdateID: initialOffset - 1,
	}
}

func (b *Bot) Token() string {
	return b.token
}

func (b *Bot) ChatID() int64 {
	return b.chatID
}

// SetOffset stores the offset file path for later persistence
func (b *Bot) SetOffset(offsetFilePath string) {
	b.offsetMu.Lock()
	defer b.offsetMu.Unlock()
	b.offsetFilePath = offsetFilePath
}

// GetMaxUpdateID returns the highest update ID seen so far
func (b *Bot) GetMaxUpdateID() int64 {
	b.offsetMu.Lock()
	defer b.offsetMu.Unlock()
	return b.maxUpdateID
}

// trackUpdateID records the highest update ID seen
func (b *Bot) trackUpdateID(update *models.Update) {
	b.offsetMu.Lock()
	defer b.offsetMu.Unlock()
	if update.ID > b.maxUpdateID {
		b.maxUpdateID = update.ID
	}
}

func (b *Bot) SendMessage(ctx context.Context, text string) (int, error) {
	start := time.Now()
	defer func() {
		metrics.ObserveTelegramMessageSend(start)
	}()

	msg, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    b.chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to send message: %w", err)
	}

	return msg.ID, nil
}

func (b *Bot) SendMessagePlain(ctx context.Context, text string) (int, error) {
	start := time.Now()
	defer func() {
		metrics.ObserveTelegramMessageSend(start)
	}()

	msg, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: b.chatID,
		Text:   text,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to send plain message: %w", err)
	}

	return msg.ID, nil
}

func (b *Bot) SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error) {
	msg, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      b.chatID,
		Text:        text,
		ReplyMarkup: keyboard,
		ParseMode:   models.ParseModeHTML,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to send message with keyboard: %w", err)
	}

	return msg.ID, nil
}

func (b *Bot) EditMessage(ctx context.Context, messageID int, text string) error {
	_, err := b.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    b.chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

func (b *Bot) EditMessagePlain(ctx context.Context, messageID int, text string) error {
	_, err := b.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    b.chatID,
		MessageID: messageID,
		Text:      text,
	})
	if err != nil {
		return fmt.Errorf("failed to edit plain message: %w", err)
	}

	return nil
}

// SendTyping sends a typing indicator to the chat
// The indicator expires after 5 seconds, so it should be refreshed every 4 seconds
func (b *Bot) SendTyping(ctx context.Context) error {
	_, err := b.bot.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: b.chatID,
		Action: models.ChatActionTyping,
	})
	if err != nil {
		return fmt.Errorf("failed to send typing: %w", err)
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

// SetMyCommands sets the bot's command list for auto-completion
func (b *Bot) SetMyCommands(ctx context.Context) error {
	commands := []models.BotCommand{
		{Command: "help", Description: "顯示所有可用指令"},
		{Command: "sessions", Description: "列出 sessions（表格檢視）"},
		{Command: "selectsession", Description: "選擇 session（互動選單）"},
		{Command: "status", Description: "顯示目前狀態"},
		{Command: "model", Description: "選擇 AI 模型"},
		{Command: "route", Description: "設定 agent 路由"},
		{Command: "new", Description: "建立新 session"},
		{Command: "abort", Description: "中止目前請求"},
	}

	_, err := b.bot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: commands,
	})
	if err != nil {
		return fmt.Errorf("failed to set commands: %w", err)
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

// StartWebhook starts the bot in webhook mode
func (b *Bot) StartWebhook(ctx context.Context, webhookURL string, port string, secretToken string) error {
	if port == "" {
		port = "8443"
	}

	// Set webhook on Telegram servers
	params := &bot.SetWebhookParams{
		URL: webhookURL,
	}
	if secretToken != "" {
		params.SecretToken = secretToken
	}

	_, err := b.bot.SetWebhook(ctx, params)
	if err != nil {
		return fmt.Errorf("set webhook: %w", err)
	}

	// Start webhook server (go-telegram/bot handles HTTP listener internally)
	// Note: StartWebhook blocks until context is cancelled
	b.bot.StartWebhook(ctx)
	return nil
}

// StopWebhook stops webhook mode and deletes the webhook
func (b *Bot) StopWebhook(ctx context.Context) error {
	_, err := b.bot.DeleteWebhook(ctx, &bot.DeleteWebhookParams{})
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	return nil
}

type TextHandler func(ctx context.Context, text string)
type CommandHandler func(ctx context.Context, args string)
type CallbackHandler func(ctx context.Context, callbackID, data string)

func (b *Bot) RegisterTextHandler(handler TextHandler) {
	b.bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		isMatch := update.Message != nil &&
			update.Message.Text != "" &&
			len(update.Message.Text) > 0 &&
			update.Message.Text[0] != '/'
		return isMatch
	}, func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[PANIC] Handler panicked: %v\n", r)
			}
		}()

		b.trackUpdateID(update)
		handler(ctx, update.Message.Text)
	})
}

func (b *Bot) RegisterCommandHandler(command string, handler CommandHandler) {
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/"+command, bot.MatchTypePrefix, func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		b.trackUpdateID(update)

		text := update.Message.Text
		args := ""
		if len(text) > len(command)+2 {
			args = text[len(command)+2:]
		}

		handler(ctx, args)
	})
}

func (b *Bot) RegisterCallbackHandler(prefix string, handler CallbackHandler) {
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, prefix, bot.MatchTypePrefix, func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		if update.CallbackQuery == nil {
			return
		}

		b.trackUpdateID(update)
		b.AnswerCallback(ctx, update.CallbackQuery.ID)
		handler(ctx, update.CallbackQuery.ID, update.CallbackQuery.Data)
	})
}

type PhotoHandler func(ctx context.Context, photos []models.PhotoSize, caption string, botToken string)

func (b *Bot) RegisterPhotoHandler(handler PhotoHandler) {
	b.bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.Message != nil && update.Message.Photo != nil && len(update.Message.Photo) > 0
	}, func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[PANIC] Photo handler panicked: %v\n", r)
			}
		}()

		b.trackUpdateID(update)
		caption := update.Message.Caption

		handler(ctx, update.Message.Photo, caption, b.token)
	})
}

type StickerHandler func(ctx context.Context, emoji string, setName string)

func (b *Bot) RegisterStickerHandler(handler StickerHandler) {
	b.bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.Message != nil && update.Message.Sticker != nil
	}, func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[PANIC] Sticker handler panicked: %v\n", r)
			}
		}()

		b.trackUpdateID(update)
		sticker := update.Message.Sticker

		// Reject animated/video stickers
		if sticker.IsAnimated || sticker.IsVideo {
			b.SendMessage(ctx, "⚠️ Animated/video stickers are not supported")
			return
		}

		// Extract emoji and set name
		emoji := sticker.Emoji
		setName := sticker.SetName

		handler(ctx, emoji, setName)
	})
}

type UnsupportedMediaHandler func(ctx context.Context)

func (b *Bot) RegisterUnsupportedMediaHandler(handler UnsupportedMediaHandler) {
	b.bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		if update.Message == nil {
			return false
		}
		return update.Message.Audio != nil ||
			update.Message.Document != nil ||
			update.Message.Video != nil ||
			update.Message.Voice != nil ||
			update.Message.VideoNote != nil ||
			update.Message.Animation != nil
	}, func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[PANIC] Unsupported media handler panicked: %v\n", r)
			}
		}()

		b.trackUpdateID(update)
		handler(ctx)
	})
}

type ReactionHandler func(ctx context.Context, messageID int, userID int64, newReaction []models.ReactionType)

func (b *Bot) RegisterReactionHandler(handler ReactionHandler) {
	b.bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.MessageReaction != nil
	}, func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[PANIC] Reaction handler panicked: %v\n", r)
			}
		}()

		b.trackUpdateID(update)
		reaction := update.MessageReaction

		messageID := reaction.MessageID
		userID := int64(0)
		if reaction.User != nil {
			userID = reaction.User.ID
		}

		handler(ctx, messageID, userID, reaction.NewReaction)
	})
}
