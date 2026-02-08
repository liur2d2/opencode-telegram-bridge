package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-telegram/bot/models"
)

// modelTelegramBot interface for sending messages and keyboards
type modelTelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
	SendMessageWithKeyboard(ctx context.Context, text string, keyboard *models.InlineKeyboardMarkup) (int, error)
	EditMessage(ctx context.Context, msgID int, text string) error
	AnswerCallback(ctx context.Context, callbackID string) error
}

// modelAppState for app state access
type modelAppState interface {
	SetCurrentModel(model string)
	GetCurrentModel() string
}

// ModelHandler manages model selection
type ModelHandler struct {
	tgBot    modelTelegramBot
	appState modelAppState
}

// NewModelHandler creates a new ModelHandler
func NewModelHandler(tgBot modelTelegramBot, appState modelAppState) *ModelHandler {
	return &ModelHandler{
		tgBot:    tgBot,
		appState: appState,
	}
}

// HandleModelCommand processes the /model command
// Shows available models as paginated Inline Keyboard
func (h *ModelHandler) HandleModelCommand(ctx context.Context) error {
	models := h.GetAvailableModels(ctx)

	// Show first page
	return h.showModelPage(ctx, models, 0)
}

// HandleModelCallback processes model selection or pagination from Inline Keyboard
func (h *ModelHandler) HandleModelCallback(ctx context.Context, msgID int, data string) error {
	models := h.GetAvailableModels(ctx)

	// Parse callback: mdl:page:N or mdl:sel:MODEL
	if len(data) < 4 {
		return fmt.Errorf("invalid callback data: %s", data)
	}

	action := data[4:] // skip "mdl:"

	// Page navigation
	if strings.HasPrefix(action, "page:") {
		var page int
		_, err := fmt.Sscanf(action[5:], "%d", &page)
		if err != nil {
			return err
		}
		if page < 0 {
			page = 0
		}
		return h.editModelPage(ctx, msgID, models, page)
	}

	// Model selection
	if strings.HasPrefix(action, "sel:") {
		model := action[4:]
		if !isValidModel(model, models) {
			return fmt.Errorf("invalid model: %s", model)
		}

		h.appState.SetCurrentModel(model)

		msg := fmt.Sprintf("✅ Model set to: %s", model)
		_, err := h.tgBot.SendMessage(ctx, msg)
		return err
	}

	return fmt.Errorf("unknown callback action: %s", action)
}

// showModelPage displays models for a given page
func (h *ModelHandler) showModelPage(ctx context.Context, models []string, page int) error {
	const perPage = 8

	if page < 0 {
		page = 0
	}

	start := page * perPage
	if start >= len(models) {
		start = len(models) - 1
		if start < 0 {
			start = 0
		}
		page = start / perPage
	}

	end := start + perPage
	if end > len(models) {
		end = len(models)
	}

	pageModels := models[start:end]
	currentModel := h.appState.GetCurrentModel()

	// Build keyboard with pagination
	keyboard := buildModelKeyboard(pageModels, currentModel, page, len(models), perPage)

	msg := "🤖 Select a Model:\n\n"
	for _, m := range pageModels {
		prefix := "  "
		if m == currentModel {
			prefix = "✅"
		}
		msg += fmt.Sprintf("%s %s\n", prefix, m)
	}

	_, err := h.tgBot.SendMessageWithKeyboard(ctx, msg, keyboard)
	return err
}

// editModelPage edits the message for a given page
func (h *ModelHandler) editModelPage(ctx context.Context, msgID int, models []string, page int) error {
	const perPage = 8

	if page < 0 {
		page = 0
	}

	start := page * perPage
	if start >= len(models) {
		start = len(models) - 1
		if start < 0 {
			start = 0
		}
		page = start / perPage
	}

	end := start + perPage
	if end > len(models) {
		end = len(models)
	}

	pageModels := models[start:end]
	currentModel := h.appState.GetCurrentModel()

	// Build keyboard with pagination
	keyboard := buildModelKeyboard(pageModels, currentModel, page, len(models), perPage)

	msg := "🤖 Select a Model:\n\n"
	for _, m := range pageModels {
		prefix := "  "
		if m == currentModel {
			prefix = "✅"
		}
		msg += fmt.Sprintf("%s %s\n", prefix, m)
	}

	return h.tgBot.EditMessage(ctx, msgID, msg)
}

// buildModelKeyboard creates an Inline Keyboard with model buttons and pagination
func buildModelKeyboard(pageModels []string, currentModel string, page int, total int, perPage int) *models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0)

	// Model buttons in 2 columns
	for i := 0; i < len(pageModels); i += 2 {
		row := make([]models.InlineKeyboardButton, 0, 2)

		// First button
		text := pageModels[i]
		if text == currentModel {
			text = "✅ " + text
		}
		cbData := fmt.Sprintf("mdl:sel:%s", pageModels[i])
		row = append(row, models.InlineKeyboardButton{
			Text:         text,
			CallbackData: cbData,
		})

		// Second button (if exists)
		if i+1 < len(pageModels) {
			text := pageModels[i+1]
			if text == currentModel {
				text = "✅ " + text
			}
			cbData := fmt.Sprintf("mdl:sel:%s", pageModels[i+1])
			row = append(row, models.InlineKeyboardButton{
				Text:         text,
				CallbackData: cbData,
			})
		}

		buttons = append(buttons, row)
	}

	// Pagination buttons
	totalPages := (total + perPage - 1) / perPage
	if totalPages > 1 {
		navRow := make([]models.InlineKeyboardButton, 0, 2)
		if page > 0 {
			prevText := fmt.Sprintf("◀️ Prev")
			prevCbData := fmt.Sprintf("mdl:page:%d", page-1)
			navRow = append(navRow, models.InlineKeyboardButton{
				Text:         prevText,
				CallbackData: prevCbData,
			})
		}
		if page < totalPages-1 {
			nextText := fmt.Sprintf("Next ▶️")
			nextCbData := fmt.Sprintf("mdl:page:%d", page+1)
			navRow = append(navRow, models.InlineKeyboardButton{
				Text:         nextText,
				CallbackData: nextCbData,
			})
		}
		if len(navRow) > 0 {
			buttons = append(buttons, navRow)
		}
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

// isValidModel checks if model is in the list
func isValidModel(model string, models []string) bool {
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}

// GetAvailableModels returns the list of available models
func (h *ModelHandler) GetAvailableModels(ctx context.Context) []string {
	// Try to load from config first
	if models, err := loadModelsFromConfig(); err == nil && len(models) > 0 {
		sort.Strings(models)
		return models
	}

	// Hardcoded fallback
	return []string{
		"claude-sonnet-4-20250514",
		"claude-opus-4-20250514",
		"claude-haiku-4-20250514",
		"o4-mini",
		"gpt-4.1",
		"gemini-2.5-pro",
		"gemini-2.5-flash",
	}
}

// loadModelsFromConfig loads model list from ~/.config/opencode/config.json
func loadModelsFromConfig() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".config", "opencode", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config struct {
		Models []string `json:"models"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if len(config.Models) > 0 {
		return config.Models, nil
	}

	return nil, fmt.Errorf("no models in config")
}
