package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/user/opencode-telegram/internal/bridge"
	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
	"github.com/user/opencode-telegram/internal/telegram"
)

func main() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	ocBaseURL := getenv("OPENCODE_BASE_URL", "http://localhost:54321")
	ocDirectory := getenv("OPENCODE_DIRECTORY", ".")
	debounceStr := getenv("TELEGRAM_DEBOUNCE_MS", "1000")
	offsetFile := getenv("TELEGRAM_OFFSET_FILE", "~/.opencode-telegram-offset")
	proxyURL := os.Getenv("TELEGRAM_PROXY") // Optional proxy (empty if not set)

	// Webhook mode variables
	webhookURL := os.Getenv("TELEGRAM_WEBHOOK_URL")         // Optional webhook URL
	webhookPort := getenv("TELEGRAM_WEBHOOK_PORT", "8443")  // Default webhook port
	webhookSecret := os.Getenv("TELEGRAM_WEBHOOK_SECRET")   // Optional secret token

	if botToken == "" || chatIDStr == "" {
		log.Fatal("Missing required environment variables: TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TELEGRAM_CHAT_ID (must be a number): %v", err)
	}

	// Parse debounce milliseconds with validation
	debounceMs, err := strconv.ParseInt(debounceStr, 10, 64)
	if err != nil || debounceMs < 0 || debounceMs > 3000 {
		debounceMs = 1000
	}
	debounceDuration := time.Duration(debounceMs) * time.Millisecond

	// Load saved offset from disk
	currentOffset, err := state.LoadOffset(offsetFile)
	if err != nil {
		log.Printf("Warning: Failed to load offset: %v. Starting from beginning.", err)
		currentOffset = 0
	}

	log.Printf("Starting OpenCode-Telegram Bridge...")
	log.Printf("OpenCode URL: %s", ocBaseURL)
	log.Printf("OpenCode Directory: %s", ocDirectory)
	log.Printf("Telegram Chat ID: %d", chatID)
	log.Printf("Debounce Duration: %dms", debounceMs)
	log.Printf("Offset File: %s (current offset: %d)", offsetFile, currentOffset)
	if proxyURL != "" {
		log.Printf("Proxy URL: %s", proxyURL)
	}

	// Determine mode (webhook or polling)
	if webhookURL != "" {
		log.Printf("Webhook Mode: URL=%s, Port=%s", webhookURL, webhookPort)
	} else {
		log.Printf("Polling Mode enabled (no TELEGRAM_WEBHOOK_URL set)")
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

	// Create OpenCode client with proxy support
	var ocClient *opencode.Client
	if transport != nil {
		ocClient = opencode.NewClientWithTransport(ocConfig, transport)
	} else {
		ocClient = opencode.NewClient(ocConfig)
	}

	// Create SSE consumer with proxy support
	var sseConsumer *opencode.SSEConsumer
	if transport != nil {
		sseConsumer = opencode.NewSSEConsumerWithTransport(ocConfig, transport)
	} else {
		sseConsumer = opencode.NewSSEConsumer(ocConfig)
	}

	// Create shared HTTP client for media downloads with proxy support
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

	// Create Telegram bot (no direct proxy parameter yet; bot library handles internally)
	tgBot := telegram.NewBot(botToken, chatID, currentOffset)
	tgBot.SetOffset(offsetFile)

	// If using proxy, optionally inject it into bot's HTTP client
	if transport != nil {
		botHTTPClient := &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
		// Note: go-telegram/bot doesn't expose direct HTTP client injection in constructor
		// This would require using bot.WithHTTPClient() at creation time if available
		_ = botHTTPClient // Placeholder for future bot proxy injection
	}

	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridgeInstance := bridge.NewBridge(ocClient, tgBot, appState, registry, debounceDuration)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if err := sseConsumer.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect SSE consumer: %v", err)
	}
	defer sseConsumer.Close()

	registry.StartCleanup(ctx)
	bridgeInstance.Start(ctx, sseConsumer)
	bridgeInstance.RegisterHandlers()

	// Start bot in appropriate mode
	go func() {
		if webhookURL != "" {
			// Webhook mode
			log.Printf("Starting Telegram bot in webhook mode on port %s...", webhookPort)
			if err := tgBot.StartWebhook(ctx, webhookURL, webhookPort, webhookSecret); err != nil {
				log.Printf("Webhook error: %v", err)
			}
		} else {
			// Polling mode (default)
			log.Println("Starting Telegram bot polling...")
			tgBot.Start(ctx)
		}
	}()

	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down gracefully...")

	// Stop webhook if in webhook mode
	if webhookURL != "" {
		if err := tgBot.StopWebhook(ctx); err != nil {
			log.Printf("Warning: Failed to stop webhook: %v", err)
		}
	}

	cancel()

	time.Sleep(2 * time.Second)

	log.Println("Shutdown complete")
}

func getenv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
