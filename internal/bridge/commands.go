package bridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
)

type CommandHandler struct {
	ocClient OpenCodeClient
	tgBot    TelegramBot
	appState *state.AppState
}

func NewCommandHandler(ocClient OpenCodeClient, tgBot TelegramBot, appState *state.AppState) *CommandHandler {
	return &CommandHandler{
		ocClient: ocClient,
		tgBot:    tgBot,
		appState: appState,
	}
}

func (h *CommandHandler) HandleNewSession(ctx context.Context, title *string) error {
	if title == nil || *title == "" {
		defaultTitle := "Telegram Chat"
		title = &defaultTitle
	}

	session, err := h.ocClient.CreateSession(title, nil)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	h.appState.SetCurrentSession(session.ID)

	msg := fmt.Sprintf("✅ New session created: %s (%s)", session.ID, session.Title)
	_, err = h.tgBot.SendMessage(ctx, msg)
	return err
}

func (h *CommandHandler) HandleListSessions(ctx context.Context) error {
	sessions, err := h.ocClient.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		_, err := h.tgBot.SendMessage(ctx, "No sessions found. Use /newsession to create one.")
		return err
	}

	currentID := h.appState.GetCurrentSession()
	var lines []string
	lines = append(lines, "📋 Available sessions:\n")

	for _, sess := range sessions {
		prefix := ""
		if sess.ID == currentID {
			prefix = "✅ "
		} else {
			prefix = "  "
		}
		lines = append(lines, fmt.Sprintf("%s%s - %s", prefix, sess.ID, sess.Title))
	}

	_, err = h.tgBot.SendMessage(ctx, strings.Join(lines, "\n"))
	return err
}

func (h *CommandHandler) HandleAbortSession(ctx context.Context) error {
	currentID := h.appState.GetCurrentSession()
	if currentID == "" {
		_, err := h.tgBot.SendMessage(ctx, "❌ No active session to abort")
		return err
	}

	err := h.ocClient.AbortSession(currentID)
	if err != nil {
		return fmt.Errorf("abort session: %w", err)
	}

	h.appState.SetCurrentSession("")

	_, err = h.tgBot.SendMessage(ctx, fmt.Sprintf("🛑 Session %s aborted", currentID))
	return err
}

func (h *CommandHandler) HandleSwitchSession(ctx context.Context, sessionID string) error {
	sessions, err := h.ocClient.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	found := false
	var selectedSession *opencode.Session
	for i := range sessions {
		if sessions[i].ID == sessionID {
			found = true
			selectedSession = &sessions[i]
			break
		}
	}

	if !found {
		_, err := h.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Session %s not found", sessionID))
		return err
	}

	h.appState.SetCurrentSession(sessionID)
	msg := fmt.Sprintf("✅ Switched to session: %s (%s)", selectedSession.ID, selectedSession.Title)
	_, err = h.tgBot.SendMessage(ctx, msg)
	return err
}

func (h *CommandHandler) HandleStatus(ctx context.Context) error {
	sessionID := h.appState.GetCurrentSession()
	agent := h.appState.GetCurrentAgent()
	status := h.appState.GetSessionStatus(sessionID)

	statusStr := "idle"
	if status == state.SessionBusy {
		statusStr = "processing"
	} else if status == state.SessionError {
		statusStr = "error"
	}

	health, err := h.ocClient.Health()
	healthStr := "unknown"
	if err == nil {
		if healthy, ok := health["healthy"].(bool); ok && healthy {
			healthStr = "healthy"
		} else {
			healthStr = "unhealthy"
		}
	}

	lines := []string{
		"📊 Status:",
		"",
		fmt.Sprintf("Session: %s", sessionID),
		fmt.Sprintf("Agent: %s", agent),
		fmt.Sprintf("Status: %s", statusStr),
		fmt.Sprintf("OpenCode: %s", healthStr),
	}

	_, err = h.tgBot.SendMessage(ctx, strings.Join(lines, "\n"))
	return err
}

func (h *CommandHandler) HandleHelp(ctx context.Context) error {
	help := `🆘 Available Commands:

/newsession [title] - Create a new session
/sessions - List all sessions
/session <id> - Switch to a session
/abort - Abort current session
/status - Show current status
/switch [agent] - Switch OHO agent
/route [agent] - Set or view per-chat agent assignment
/help - Show this help message`

	_, err := h.tgBot.SendMessage(ctx, help)
	return err
}

func (h *CommandHandler) HandleRouteCommand(ctx context.Context, args string, chatID string) error {
	args = strings.TrimSpace(args)

	if args == "" {
		return h.showRouteStatus(ctx, chatID)
	}

	if args == "clear" {
		h.appState.RemoveChatAgent(chatID)
		_, err := h.tgBot.SendMessage(ctx, "✅ Chat agent assignment cleared. Using global agent.")
		return err
	}

	agent := args
	h.appState.SetChatAgent(chatID, agent)
	msg := fmt.Sprintf("✅ Chat agent set to: %s", agent)
	_, err := h.tgBot.SendMessage(ctx, msg)
	return err
}

func (h *CommandHandler) showRouteStatus(ctx context.Context, chatID string) error {
	chatAgent := h.appState.GetChatAgent(chatID)
	globalAgent := h.appState.GetCurrentAgent()

	var status string
	if chatAgent != "" {
		status = fmt.Sprintf("🎯 Chat Agent: %s\n📍 Global Agent: %s", chatAgent, globalAgent)
	} else {
		status = fmt.Sprintf("📍 Using Global Agent: %s\n(No chat-specific override)", globalAgent)
	}

	_, err := h.tgBot.SendMessage(ctx, status)
	return err
}
