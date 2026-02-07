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
	ocPassword := os.Getenv("OPENCODE_SERVER_PASSWORD")

	if botToken == "" || chatIDStr == "" {
		log.Fatal("Missing required environment variables: TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TELEGRAM_CHAT_ID (must be a number): %v", err)
	}

	log.Printf("Starting OpenCode-Telegram Bridge...")
	log.Printf("OpenCode URL: %s", ocBaseURL)
	log.Printf("OpenCode Directory: %s", ocDirectory)
	log.Printf("Telegram Chat ID: %d", chatID)

	ocConfig := opencode.Config{
		BaseURL:        ocBaseURL,
		Directory:      ocDirectory,
		ServerPassword: ocPassword,
	}
	ocClient := opencode.NewClient(ocConfig)

	sseConsumer := opencode.NewSSEConsumer(ocConfig)

	tgBot := telegram.NewBot(botToken, chatID)

	appState := state.NewAppState()
	registry := state.NewIDRegistry()

	bridgeInstance := bridge.NewBridge(ocClient, tgBot, appState, registry)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if err := sseConsumer.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect SSE consumer: %v", err)
	}
	defer sseConsumer.Close()

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
