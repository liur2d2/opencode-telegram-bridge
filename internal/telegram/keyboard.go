package telegram

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/go-telegram/bot/models"
)

// QuestionOption represents a single option in a question
type QuestionOption struct {
	Label       string
	Description string
}

// QuestionInfo represents question information from OpenCode
type QuestionInfo struct {
	Question string
	Header   string
	Options  []QuestionOption
	Multiple bool
	Custom   bool
}

// BuildQuestionKeyboard builds an inline keyboard for a question
// Each option becomes a button with callback_data: {shortKey}:{optionIndex}
// shortKey is expected to be the short key from registry (e.g., "q:2:0")
func BuildQuestionKeyboard(info QuestionInfo, shortKey string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	// Add a button for each option
	for i, option := range info.Options {
		button := models.InlineKeyboardButton{
			Text:         option.Label,
			CallbackData: fmt.Sprintf("%s:%d", shortKey, i),
		}
		rows = append(rows, []models.InlineKeyboardButton{button})
	}

	// If multiple selection, add Submit button
	if info.Multiple {
		submitButton := models.InlineKeyboardButton{
			Text:         "✅ Submit",
			CallbackData: fmt.Sprintf("%s:submit", shortKey),
		}
		rows = append(rows, []models.InlineKeyboardButton{submitButton})
	}

	// If custom input allowed, add custom button
	if info.Custom {
		customButton := models.InlineKeyboardButton{
			Text:         "✏️ Type custom...",
			CallbackData: fmt.Sprintf("%s:custom", shortKey),
		}
		rows = append(rows, []models.InlineKeyboardButton{customButton})
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

// BuildPermissionKeyboard builds the standard permission keyboard
// Returns keyboard with: Allow Once, Always Allow, Reject
func BuildPermissionKeyboard(permissionID string) *models.InlineKeyboardMarkup {
	// Generate a short ID to keep callback_data under 64 bytes
	shortID := generateShortID(permissionID)

	rows := [][]models.InlineKeyboardButton{
		{
			{
				Text:         "✅ Allow Once",
				CallbackData: fmt.Sprintf("p:%s:once", shortID),
			},
		},
		{
			{
				Text:         "✅ Always Allow",
				CallbackData: fmt.Sprintf("p:%s:always", shortID),
			},
		},
		{
			{
				Text:         "❌ Reject",
				CallbackData: fmt.Sprintf("p:%s:reject", shortID),
			},
		},
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

// generateShortID creates a short hash from a long ID to keep callback_data under 64 bytes
// Takes first 8 characters of SHA256 hash
func generateShortID(id string) string {
	hash := sha256.Sum256([]byte(id))
	return hex.EncodeToString(hash[:4]) // 8 characters
}
