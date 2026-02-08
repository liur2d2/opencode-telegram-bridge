package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// AccountConfig represents a single Telegram bot account
type AccountConfig struct {
	Token  string `json:"token"`
	ChatID int64  `json:"chat_id"`
	Name   string `json:"name,omitempty"`
}

// ParseAccountConfigs reads and parses Telegram bot account configurations
// Supports TELEGRAM_ACCOUNTS JSON env var for multi-account mode
// Falls back to single-account mode with TELEGRAM_BOT_TOKEN + TELEGRAM_CHAT_ID
func ParseAccountConfigs() ([]AccountConfig, error) {
	// Check for multi-account configuration
	accountsJSON := os.Getenv("TELEGRAM_ACCOUNTS")
	if accountsJSON != "" {
		var accounts []AccountConfig
		if err := json.Unmarshal([]byte(accountsJSON), &accounts); err != nil {
			return nil, fmt.Errorf("parse TELEGRAM_ACCOUNTS JSON: %w", err)
		}

		if len(accounts) > 5 {
			return nil, fmt.Errorf("too many accounts (max 5, got %d)", len(accounts))
		}

		// Validate all accounts have token and chatID
		for i, acc := range accounts {
			if acc.Token == "" {
				return nil, fmt.Errorf("account %d missing token", i)
			}
			if acc.ChatID == 0 {
				return nil, fmt.Errorf("account %d missing chat_id", i)
			}
		}

		return accounts, nil
	}

	// Single-account fallback
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}

	if chatIDStr == "" {
		return nil, fmt.Errorf("TELEGRAM_CHAT_ID not set")
	}

	var chatID int64
	_, err := fmt.Sscanf(chatIDStr, "%d", &chatID)
	if err != nil {
		return nil, fmt.Errorf("parse TELEGRAM_CHAT_ID: %w", err)
	}

	return []AccountConfig{
		{
			Token:  token,
			ChatID: chatID,
			Name:   "default",
		},
	}, nil
}
