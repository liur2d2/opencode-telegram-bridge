package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAccountConfigsSingleAccount(t *testing.T) {
	// Save and restore env vars
	oldToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	oldChatID := os.Getenv("TELEGRAM_CHAT_ID")
	oldAccounts := os.Getenv("TELEGRAM_ACCOUNTS")
	defer func() {
		os.Setenv("TELEGRAM_BOT_TOKEN", oldToken)
		os.Setenv("TELEGRAM_CHAT_ID", oldChatID)
		os.Setenv("TELEGRAM_ACCOUNTS", oldAccounts)
	}()

	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	os.Setenv("TELEGRAM_CHAT_ID", "123456")
	os.Setenv("TELEGRAM_ACCOUNTS", "")

	accounts, err := ParseAccountConfigs()
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "test-token", accounts[0].Token)
	assert.Equal(t, int64(123456), accounts[0].ChatID)
	assert.Equal(t, "default", accounts[0].Name)
}

func TestParseAccountConfigsMultiAccount(t *testing.T) {
	oldAccounts := os.Getenv("TELEGRAM_ACCOUNTS")
	defer os.Setenv("TELEGRAM_ACCOUNTS", oldAccounts)

	jsonConfig := `[{"token":"token1","chat_id":111,"name":"work"},{"token":"token2","chat_id":222,"name":"personal"}]`
	os.Setenv("TELEGRAM_ACCOUNTS", jsonConfig)

	accounts, err := ParseAccountConfigs()
	require.NoError(t, err)
	require.Len(t, accounts, 2)
	assert.Equal(t, "token1", accounts[0].Token)
	assert.Equal(t, int64(111), accounts[0].ChatID)
	assert.Equal(t, "work", accounts[0].Name)
	assert.Equal(t, "token2", accounts[1].Token)
	assert.Equal(t, int64(222), accounts[1].ChatID)
	assert.Equal(t, "personal", accounts[1].Name)
}

func TestParseAccountConfigsInvalidJSON(t *testing.T) {
	oldAccounts := os.Getenv("TELEGRAM_ACCOUNTS")
	defer os.Setenv("TELEGRAM_ACCOUNTS", oldAccounts)

	os.Setenv("TELEGRAM_ACCOUNTS", `invalid json`)

	_, err := ParseAccountConfigs()
	assert.Error(t, err)
}

func TestParseAccountConfigsMaxLimit(t *testing.T) {
	oldAccounts := os.Getenv("TELEGRAM_ACCOUNTS")
	defer os.Setenv("TELEGRAM_ACCOUNTS", oldAccounts)

	// Create 7 accounts (exceeds limit)
	jsonConfig := `[
		{"token":"t1","chat_id":1,"name":"a1"},
		{"token":"t2","chat_id":2,"name":"a2"},
		{"token":"t3","chat_id":3,"name":"a3"},
		{"token":"t4","chat_id":4,"name":"a4"},
		{"token":"t5","chat_id":5,"name":"a5"},
		{"token":"t6","chat_id":6,"name":"a6"},
		{"token":"t7","chat_id":7,"name":"a7"}
	]`
	os.Setenv("TELEGRAM_ACCOUNTS", jsonConfig)

	accounts, err := ParseAccountConfigs()
	require.NoError(t, err)
	// Should be limited to 5
	assert.Len(t, accounts, 5)
}

func TestParseAccountConfigsEmptyEnv(t *testing.T) {
	oldToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	oldChatID := os.Getenv("TELEGRAM_CHAT_ID")
	oldAccounts := os.Getenv("TELEGRAM_ACCOUNTS")
	defer func() {
		os.Setenv("TELEGRAM_BOT_TOKEN", oldToken)
		os.Setenv("TELEGRAM_CHAT_ID", oldChatID)
		os.Setenv("TELEGRAM_ACCOUNTS", oldAccounts)
	}()

	os.Setenv("TELEGRAM_BOT_TOKEN", "")
	os.Setenv("TELEGRAM_CHAT_ID", "")
	os.Setenv("TELEGRAM_ACCOUNTS", "")

	accounts, err := ParseAccountConfigs()
	require.NoError(t, err)
	assert.Len(t, accounts, 0)
}
