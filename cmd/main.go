package main

import (
	"context"
	"log"
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

	if botToken == "" || chatIDStr == "" {
		log.Fatal("Missing required environment variables: TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TELEGRAM_CHAT_ID (must be a number): %v", err)
	}

	// Parse debounce milliseconds
	debounceMs, err := strconv.ParseInt(debounceStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TELEGRAM_DEBOUNCE_MS (must be a number): %v", err)
	}
	debounceDuration := time.Duration(debounceMs) * time.Millisecond

	log.Printf("Starting OpenCode-Telegram Bridge...")
	log.Printf("OpenCode URL: %s", ocBaseURL)
	log.Printf("OpenCode Directory: %s", ocDirectory)
	log.Printf("Telegram Chat ID: %d", chatID)
	log.Printf("Debounce Duration: %dms", debounceMs)

	ocConfig := opencode.Config{
		BaseURL:   ocBaseURL,
		Directory: ocDirectory,
	}
	ocClient := opencode.NewClient(ocConfig)

	sseConsumer := opencode.NewSSEConsumer(ocConfig)

	tgBot := telegram.NewBot(botToken, chatID)

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

	go func() {
		log.Println("Starting Telegram bot polling...")
		tgBot.Start(ctx)
	}()

	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down gracefully...")

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
