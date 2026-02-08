package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/user/opencode-telegram/internal/bridge"
	"github.com/user/opencode-telegram/internal/config"
	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
	"github.com/user/opencode-telegram/internal/telegram"
)

func main() {
	// Read shared configuration
	ocBaseURL := getenv("OPENCODE_BASE_URL", "http://localhost:54321")
	ocDirectory := getenv("OPENCODE_DIRECTORY", ".")
	debounceStr := getenv("TELEGRAM_DEBOUNCE_MS", "1000")
	offsetFile := getenv("TELEGRAM_OFFSET_FILE", "~/.opencode-telegram-offset")
	proxyURL := os.Getenv("TELEGRAM_PROXY")

	// Webhook mode variables
	webhookURL := os.Getenv("TELEGRAM_WEBHOOK_URL")
	webhookPort := getenv("TELEGRAM_WEBHOOK_PORT", "8443")
	webhookSecret := os.Getenv("TELEGRAM_WEBHOOK_SECRET")

	// Parse bot accounts
	accounts, err := config.ParseAccountConfigs()
	if err != nil {
		log.Fatalf("Failed to parse account configs: %v", err)
	}

	if len(accounts) == 0 {
		log.Fatal("No bot accounts configured. Set TELEGRAM_BOT_TOKEN + TELEGRAM_CHAT_ID or TELEGRAM_ACCOUNTS")
	}

	// Parse debounce with validation
	debounceMs, err := strconv.ParseInt(debounceStr, 10, 64)
	if err != nil || debounceMs < 0 || debounceMs > 3000 {
		debounceMs = 1000
	}
	debounceDuration := time.Duration(debounceMs) * time.Millisecond

	log.Printf("Starting OpenCode-Telegram Bridge...")
	log.Printf("OpenCode URL: %s", ocBaseURL)
	log.Printf("OpenCode Directory: %s", ocDirectory)
	log.Printf("Debounce Duration: %dms", debounceMs)
	log.Printf("Active Accounts: %d", len(accounts))
	if proxyURL != "" {
		log.Printf("Proxy URL: %s", proxyURL)
	}
	if webhookURL != "" {
		log.Printf("Webhook Mode: URL=%s, Port=%s", webhookURL, webhookPort)
	} else {
		log.Printf("Polling Mode enabled")
	}

	// Create shared HTTP transport with proxy support
	var transport *http.Transport
	if proxyURL != "" {
		var err error
		transport, err = opencode.NewProxyTransport(proxyURL)
		if err != nil {
			log.Fatalf("Failed to create proxy transport: %v", err)
		}
		log.Printf("Proxy transport created: %s", proxyURL)
	}

	ocConfig := opencode.Config{
		BaseURL:   ocBaseURL,
		Directory: ocDirectory,
	}

	// Create shared OpenCode client (one per bridge)
	var ocClient *opencode.Client
	if transport != nil {
		ocClient = opencode.NewClientWithTransport(ocConfig, transport)
	} else {
		ocClient = opencode.NewClient(ocConfig)
	}

	// Create shared SSE consumer (one for all accounts)
	var sseConsumer *opencode.SSEConsumer
	if transport != nil {
		sseConsumer = opencode.NewSSEConsumerWithTransport(ocConfig, transport)
	} else {
		sseConsumer = opencode.NewSSEConsumer(ocConfig)
	}

	// Create shared HTTP client for media downloads
	var mediaClient *http.Client
	if transport != nil {
		mediaClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
	} else {
		mediaClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	telegram.SetMediaClient(mediaClient)

	// Setup context and signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Connect SSE consumer (shared)
	if err := sseConsumer.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect SSE consumer: %v", err)
	}
	defer sseConsumer.Close()

	// Create and start bot instances (one per account)
	var wg sync.WaitGroup
	for i, account := range accounts {
		wg.Add(1)
		go func(idx int, acc config.AccountConfig) {
			defer wg.Done()
			runBotInstance(ctx, idx, acc, ocClient, sseConsumer, debounceDuration, offsetFile, webhookURL, webhookPort, webhookSecret)
		}(i, account)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down gracefully...")

	cancel()

	// Wait for all bots to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait up to 5 seconds for graceful shutdown
	select {
	case <-done:
		log.Println("All bots shut down gracefully")
	case <-time.After(5 * time.Second):
		log.Println("Shutdown timeout exceeded")
	}

	log.Println("Shutdown complete")
}

// runBotInstance runs a single bot instance for one account
func runBotInstance(
	ctx context.Context,
	accountIdx int,
	account config.AccountConfig,
	ocClient *opencode.Client,
	sseConsumer *opencode.SSEConsumer,
	debounceDuration time.Duration,
	offsetFile string,
	webhookURL, webhookPort, webhookSecret string,
) {
	// Load offset for this account
	currentOffset, err := state.LoadOffset(offsetFile)
	if err != nil {
		log.Printf("[Account %d] Warning: Failed to load offset: %v. Starting from beginning.", accountIdx, err)
		currentOffset = 0
	}

	accountName := account.Name
	if accountName == "" {
		accountName = "account-" + strconv.Itoa(accountIdx)
	}

	log.Printf("[%s] Starting bot instance (ChatID: %d)", accountName, account.ChatID)

	// Create bot instance (one per account)
	tgBot := telegram.NewBot(account.Token, account.ChatID, currentOffset)
	tgBot.SetOffset(offsetFile)

	// Create state instance (one per account)
	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	// Create bridge instance (one per account)
	bridgeInstance := bridge.NewBridge(ocClient, tgBot, appState, registry, debounceDuration)

	// Start bridge
	bridgeInstance.Start(ctx, sseConsumer)
	bridgeInstance.RegisterHandlers()

	// Start registry cleanup
	registry.StartCleanup(ctx)

	// Start bot in appropriate mode
	if webhookURL != "" {
		// Webhook mode
		log.Printf("[%s] Starting in webhook mode on port %s", accountName, webhookPort)
		if err := tgBot.StartWebhook(ctx, webhookURL, webhookPort, webhookSecret); err != nil {
			log.Printf("[%s] Webhook error: %v", accountName, err)
		}
	} else {
		// Polling mode (default)
		log.Printf("[%s] Starting in polling mode", accountName)
		tgBot.Start(ctx)
	}

	log.Printf("[%s] Bot instance shut down", accountName)
}

func getenv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
