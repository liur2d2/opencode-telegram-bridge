package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
)

// AccountConfig represents a single bot account configuration
type AccountConfig struct {
	Token  string `json:"token"`
	ChatID int64  `json:"chat_id"`
	Name   string `json:"name"` // Optional label for the account
}

// ParseAccountConfigs parses bot accounts from environment variables
// First checks TELEGRAM_ACCOUNTS (JSON array), then falls back to single-account mode
// Returns []AccountConfig with max 5 accounts (safety limit)
func ParseAccountConfigs() ([]AccountConfig, error) {
	// Check for multi-account JSON config
	accountsJSON := os.Getenv("TELEGRAM_ACCOUNTS")
	if accountsJSON != "" {
		var accounts []AccountConfig
		if err := json.Unmarshal([]byte(accountsJSON), &accounts); err != nil {
			return nil, err
		}

		// Enforce max 5 accounts
		if len(accounts) > 5 {
			log.Printf("Warning: More than 5 accounts configured, limiting to 5")
			accounts = accounts[:5]
		}

		// Validate each account
		for i, acc := range accounts {
			if acc.Token == "" {
				log.Fatalf("Account %d: missing token", i)
			}
			if acc.ChatID == 0 {
				log.Fatalf("Account %d: missing or invalid chat_id", i)
			}
		}

		return accounts, nil
	}

	// Fall back to single-account mode from env vars
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" || chatIDStr == "" {
		return nil, nil // Will be caught by main.go
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return nil, err
	}

	return []AccountConfig{
		{
			Token:  botToken,
			ChatID: chatID,
			Name:   "default",
		},
	}, nil
}
