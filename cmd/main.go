package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/user/opencode-telegram/internal/bridge"
	"github.com/user/opencode-telegram/internal/config"
	"github.com/user/opencode-telegram/internal/health"
	"github.com/user/opencode-telegram/internal/metrics"
	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
	"github.com/user/opencode-telegram/internal/telegram"
	"github.com/user/opencode-telegram/internal/webhook"
)

func main() {
	// Read shared configuration
	ocBaseURL := getenv("OPENCODE_BASE_URL", "http://localhost:54321")
	ocDirectory := getenv("OPENCODE_DIRECTORY", ".")
	debounceStr := getenv("TELEGRAM_DEBOUNCE_MS", "1000")
	offsetFile := getenv("TELEGRAM_OFFSET_FILE", "~/.opencode-telegram-offset")
	stateFile := getenv("TELEGRAM_STATE_FILE", "~/.opencode-telegram-state")
	proxyURL := os.Getenv("TELEGRAM_PROXY")

	// Webhook mode variables
	webhookURL := os.Getenv("TELEGRAM_WEBHOOK_URL")
	webhookPort := getenv("TELEGRAM_WEBHOOK_PORT", "8443")
	webhookSecret := os.Getenv("TELEGRAM_WEBHOOK_SECRET")

	// OpenCode plugin webhook variables
	pluginWebhookPort := getenv("PLUGIN_WEBHOOK_PORT", "8888")
	usePlugin := getenv("USE_PLUGIN_MODE", "true") == "true"

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
	log.Printf("Plugin Mode: %v (webhook port: %s)", usePlugin, pluginWebhookPort)
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

	// Create shared SSE consumer (only if not using plugin mode)
	var sseConsumer *opencode.SSEConsumer
	if !usePlugin {
		if transport != nil {
			sseConsumer = opencode.NewSSEConsumerWithTransport(ocConfig, transport)
		} else {
			sseConsumer = opencode.NewSSEConsumer(ocConfig)
		}
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
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Create health monitor
	healthMonitor := health.NewHealthMonitor()

	// Start health endpoint
	healthPort := getenv("HEALTH_PORT", "8080")
	healthMux := http.NewServeMux()
	healthMux.Handle("/health", healthMonitor)
	healthMux.Handle("/metrics", promhttp.Handler())
	healthServer := &http.Server{
		Addr:    ":" + healthPort,
		Handler: healthMux,
	}
	go func() {
		log.Printf("Health endpoint listening on :%s/health", healthPort)
		log.Printf("Metrics endpoint listening on :%s/metrics", healthPort)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		healthServer.Shutdown(shutdownCtx)
	}()

	// Initialize Prometheus metrics
	_ = metrics.SSEEventProcessingLatency
	_ = metrics.TelegramMessageSendLatency
	_ = metrics.ActiveSSEConnections
	_ = metrics.SSEConnectionErrors

	// Start plugin webhook server if enabled
	var pluginWebhook *webhook.Server
	var firstBridge *bridge.Bridge
	if usePlugin {
		log.Printf("Plugin mode enabled, will start webhook server after bridge initialization")
	} else {
		// Connect SSE consumer (shared) if not using plugin
		if err := sseConsumer.Connect(ctx); err != nil {
			log.Fatalf("Failed to connect SSE consumer: %v", err)
		}
		defer sseConsumer.Close()
		healthMonitor.SetSSEConnected(true)
	}

	// Create and start bot instances (one per account)
	var wg sync.WaitGroup
	bridgeChan := make(chan *bridge.Bridge, 1)

	for i, account := range accounts {
		wg.Add(1)
		go func(idx int, acc config.AccountConfig) {
			defer wg.Done()
			bridgeInst := runBotInstance(ctx, idx, acc, ocClient, sseConsumer, healthMonitor, debounceDuration, offsetFile, stateFile, webhookURL, webhookPort, webhookSecret)
			if idx == 0 && usePlugin {
				bridgeChan <- bridgeInst
			}
		}(i, account)
	}

	if usePlugin {
		select {
		case firstBridge = <-bridgeChan:
			pluginWebhook = webhook.NewServer(":"+pluginWebhookPort, firstBridge)
			go func() {
				if err := pluginWebhook.Start(ctx); err != nil {
					log.Printf("Plugin webhook server error: %v", err)
				}
			}()
		case <-time.After(5 * time.Second):
			log.Printf("Warning: Timeout waiting for first bridge instance")
		}
	}

	// Wait for shutdown signal or reload
	for {
		sig := <-sigChan
		log.Printf("Received signal: %v", sig)

		if sig == syscall.SIGHUP {
			log.Println("Reloading configuration...")
			if err := reloadConfig(&ocDirectory); err != nil {
				log.Printf("Config reload failed: %v", err)
			} else {
				log.Println("Configuration reloaded successfully")
			}
			continue
		}

		log.Println("Shutting down gracefully...")
		break
	}

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
	healthMonitor *health.HealthMonitor,
	debounceDuration time.Duration,
	offsetFile string,
	stateFile string,
	webhookURL, webhookPort, webhookSecret string,
) *bridge.Bridge {
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

	// Set bot commands for auto-completion
	if err := tgBot.SetMyCommands(ctx); err != nil {
		log.Printf("[%s] Warning: failed to set commands: %v", accountName, err)
	}

	appState := state.NewAppState(stateFile)
	registry := state.NewIDRegistry()

	// Create bridge instance (one per account)
	bridgeInstance := bridge.NewBridge(ocClient, tgBot, appState, registry, debounceDuration)
	bridgeInstance.SetHealthMonitor(healthMonitor)

	// Start bridge (only if SSE consumer exists)
	if sseConsumer != nil {
		bridgeInstance.Start(ctx, sseConsumer)
	}
	bridgeInstance.RegisterHandlers()

	// Start registry cleanup
	registry.StartCleanup(ctx)

	go func() {
		if webhookURL != "" {
			log.Printf("[%s] Starting in webhook mode on port %s", accountName, webhookPort)
			if err := tgBot.StartWebhook(ctx, webhookURL, webhookPort, webhookSecret); err != nil {
				log.Printf("[%s] Webhook error: %v", accountName, err)
			}
		} else {
			log.Printf("[%s] Starting in polling mode", accountName)
			tgBot.Start(ctx)
		}
		log.Printf("[%s] Bot instance shut down", accountName)
	}()

	return bridgeInstance
}

func getenv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func reloadConfig(currentDirectory *string) error {
	credFile := os.ExpandEnv("$HOME/.opencode-telegram-credentials")
	data, err := os.ReadFile(credFile)
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		if key == "OPENCODE_DIRECTORY" && value != "" {
			if *currentDirectory != value {
				log.Printf("Updated OPENCODE_DIRECTORY: %s -> %s", *currentDirectory, value)
				*currentDirectory = value
			}
		}
	}

	return nil
}
