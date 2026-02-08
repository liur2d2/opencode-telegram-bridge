package main

import (
	"context"
	"encoding/json"
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

type BotConfig struct {
	Token  string `json:"token"`
	ChatID int64  `json:"chatID"`
}

func main() {
	ocBaseURL := getenv("OPENCODE_BASE_URL", "http://localhost:54321")
	ocDirectory := getenv("OPENCODE_DIRECTORY", ".")
	debounceStr := getenv("TELEGRAM_DEBOUNCE_MS", "1000")
	offsetFile := getenv("TELEGRAM_OFFSET_FILE", "~/.opencode-telegram-offset")
	proxyURL := os.Getenv("TELEGRAM_PROXY")
	webhookURL := os.Getenv("TELEGRAM_WEBHOOK_URL")
	webhookPort := getenv("TELEGRAM_WEBHOOK_PORT", "8443")
	webhookSecret := os.Getenv("TELEGRAM_WEBHOOK_SECRET")
	telegramBotsJSON := os.Getenv("TELEGRAM_BOTS")

	var botConfigs []BotConfig

	if telegramBotsJSON != "" {
		if err := json.Unmarshal([]byte(telegramBotsJSON), &botConfigs); err != nil {
			log.Fatalf("Invalid TELEGRAM_BOTS JSON: %v", err)
		}
		if len(botConfigs) == 0 {
			log.Fatal("TELEGRAM_BOTS array is empty")
		}
		if len(botConfigs) > 10 {
			log.Fatal("Too many bots (max 10)")
		}
	} else {
		botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
		chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
		if botToken == "" || chatIDStr == "" {
			log.Fatal("Missing required environment variables: TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID or TELEGRAM_BOTS")
		}
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			log.Fatalf("Invalid TELEGRAM_CHAT_ID (must be a number): %v", err)
		}
		botConfigs = []BotConfig{{Token: botToken, ChatID: chatID}}
	}

	debounceMs, err := strconv.ParseInt(debounceStr, 10, 64)
	if err != nil || debounceMs < 0 || debounceMs > 3000 {
		debounceMs = 1000
	}
	debounceDuration := time.Duration(debounceMs) * time.Millisecond

	currentOffset, err := state.LoadOffset(offsetFile)
	if err != nil {
		log.Printf("Warning: Failed to load offset: %v. Starting from beginning.", err)
		currentOffset = 0
	}

	log.Printf("Starting OpenCode-Telegram Bridge...")
	log.Printf("OpenCode URL: %s", ocBaseURL)
	log.Printf("OpenCode Directory: %s", ocDirectory)
	log.Printf("Number of bots: %d", len(botConfigs))
	for i, cfg := range botConfigs {
		log.Printf("  Bot %d: Chat ID: %d", i+1, cfg.ChatID)
	}
	log.Printf("Debounce Duration: %dms", debounceMs)
	log.Printf("Offset File: %s (current offset: %d)", offsetFile, currentOffset)
	if proxyURL != "" {
		log.Printf("Proxy URL: %s", proxyURL)
	}
	if webhookURL != "" {
		log.Printf("Webhook Mode: URL=%s, Port=%s", webhookURL, webhookPort)
	} else {
		log.Printf("Polling Mode enabled (no TELEGRAM_WEBHOOK_URL set)")
	}

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

	var ocClient *opencode.Client
	if transport != nil {
		ocClient = opencode.NewClientWithTransport(ocConfig, transport)
	} else {
		ocClient = opencode.NewClient(ocConfig)
	}

	var sseConsumer *opencode.SSEConsumer
	if transport != nil {
		sseConsumer = opencode.NewSSEConsumerWithTransport(ocConfig, transport)
	} else {
		sseConsumer = opencode.NewSSEConsumer(ocConfig)
	}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if err := sseConsumer.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect SSE consumer: %v", err)
	}
	defer sseConsumer.Close()

	registry := state.NewIDRegistry()
	registry.StartCleanup(ctx)

	var bridges []*bridge.Bridge
	var bots []*telegram.Bot

	for i, cfg := range botConfigs {
		log.Printf("Initializing bot %d (Chat ID: %d)...", i+1, cfg.ChatID)

		tgBot := telegram.NewBot(cfg.Token, cfg.ChatID, currentOffset)
		tgBot.SetOffset(offsetFile)
		bots = append(bots, tgBot)

		appState := state.NewAppState()
		bridgeInstance := bridge.NewBridge(ocClient, tgBot, appState, registry, debounceDuration)
		bridges = append(bridges, bridgeInstance)

		bridgeInstance.Start(ctx, sseConsumer)
		bridgeInstance.RegisterHandlers()
	}

	for i, tgBot := range bots {
		i := i
		tgBot := tgBot
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Bot %d crashed: %v", i+1, r)
				}
			}()

			if webhookURL != "" {
				log.Printf("Bot %d: Starting in webhook mode on port %s...", i+1, webhookPort)
				if err := tgBot.StartWebhook(ctx, webhookURL, webhookPort, webhookSecret); err != nil {
					log.Printf("Bot %d webhook error: %v", i+1, err)
				}
			} else {
				log.Printf("Bot %d: Starting polling...", i+1)
				tgBot.Start(ctx)
			}
		}()
	}

	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down gracefully...")

	if webhookURL != "" {
		for i, tgBot := range bots {
			if err := tgBot.StopWebhook(ctx); err != nil {
				log.Printf("Bot %d: Warning: Failed to stop webhook: %v", i+1, err)
			}
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
