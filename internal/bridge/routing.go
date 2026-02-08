package bridge

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// RoutingHandler handles /route commands for per-chat agent assignment
type RoutingHandler struct {
	appState routingAppState
	tgBot    routingTelegramBot
}

// Interfaces for dependency injection
type routingAppState interface {
	GetCurrentAgent() string
	SetChatAgent(chatID string, agent string)
	GetChatAgent(chatID string) string
	RemoveChatAgent(chatID string)
}

type routingTelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
	AnswerCallback(ctx context.Context, callbackID string) error
}

// NewRoutingHandler creates a new routing handler
func NewRoutingHandler(appState routingAppState, tgBot routingTelegramBot) *RoutingHandler {
	return &RoutingHandler{
		appState: appState,
		tgBot:    tgBot,
	}
}

// GetEffectiveAgent returns the agent to use for a given chat
// First checks per-chat assignment, then falls back to global agent
func GetEffectiveAgent(chatID string, appState routingAppState) string {
	// Check per-chat assignment first
	if chatAgent := appState.GetChatAgent(chatID); chatAgent != "" {
		return chatAgent
	}
	// Fall back to global agent
	return appState.GetCurrentAgent()
}

// HandleRouteCommand handles the /route command
func (h *RoutingHandler) HandleRouteCommand(ctx context.Context, chatID string, args string) {
	args = strings.TrimSpace(args)

	if args == "" {
		// Show current routing status
		h.showRoutingStatus(ctx, chatID)
		return
	}

	if args == "clear" {
		// Remove per-chat assignment
		h.appState.RemoveChatAgent(chatID)
		if _, err := h.tgBot.SendMessage(ctx, "✅ Chat agent assignment cleared. Using global agent."); err != nil {
			log.Printf("[ROUTING] Failed to send message: %v", err)
		}
		return
	}

	// Set per-chat agent
	agentName := args
	h.appState.SetChatAgent(chatID, agentName)
	message := fmt.Sprintf("✅ Chat agent set to: %s", agentName)
	if _, err := h.tgBot.SendMessage(ctx, message); err != nil {
		log.Printf("[ROUTING] Failed to send message: %v", err)
	}
}

// showRoutingStatus shows the current routing configuration
func (h *RoutingHandler) showRoutingStatus(ctx context.Context, chatID string) {
	chatAgent := h.appState.GetChatAgent(chatID)
	globalAgent := h.appState.GetCurrentAgent()

	var status string
	if chatAgent != "" {
		status = fmt.Sprintf("🎯 Chat Agent: %s\n📍 Global Agent: %s", chatAgent, globalAgent)
	} else {
		status = fmt.Sprintf("📍 Using Global Agent: %s\n(No chat-specific override)", globalAgent)
	}

	if _, err := h.tgBot.SendMessage(ctx, status); err != nil {
		log.Printf("[ROUTING] Failed to send message: %v", err)
	}
}
