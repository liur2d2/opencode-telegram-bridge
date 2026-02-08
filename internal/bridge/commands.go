package bridge

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/state"
)

type CommandHandler struct {
	ocClient        OpenCodeClient
	tgBot           TelegramBot
	appState        *state.AppState
	sessionCache    []opencode.Session
	sessionCacheKey string
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

	primarySessions := []opencode.Session{}
	for _, sess := range sessions {
		if sess.ParentID == nil {
			primarySessions = append(primarySessions, sess)
		}
	}

	if len(primarySessions) == 0 {
		_, err := h.tgBot.SendMessage(ctx, "No primary sessions found.")
		return err
	}

	currentID := h.appState.GetCurrentSession()

	const maxDisplay = 15
	displaySessions := primarySessions
	if len(primarySessions) > maxDisplay {
		displaySessions = primarySessions[:maxDisplay]
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("📋 <b>Primary Sessions</b> (showing %d of %d)\n", len(displaySessions), len(primarySessions)))

	for _, sess := range displaySessions {
		var statusIcon string
		if sess.ID == currentID {
			statusIcon = "🟢"
		} else {
			statusIcon = "⚫"
		}

		displayTitle := sess.Title
		if len(displayTitle) > 30 {
			runes := []rune(displayTitle)
			if len(runes) > 30 {
				displayTitle = string(runes[:27]) + "..."
			}
		}

		lastUsed := time.Unix(0, sess.Time.Updated*int64(time.Millisecond))
		timeAgo := formatTimeAgo(time.Since(lastUsed))

		lines = append(lines, fmt.Sprintf("%s <b>%s</b> (%s)", statusIcon, displayTitle, sess.Slug))
		lines = append(lines, fmt.Sprintf("   <code>%s</code>", sess.ID))
		lines = append(lines, fmt.Sprintf("   🕐 %s\n", timeAgo))
	}

	if len(primarySessions) > maxDisplay {
		lines = append(lines, fmt.Sprintf("💡 <i>... and %d more sessions</i>", len(primarySessions)-maxDisplay))
	}

	lines = append(lines, "\n<b>Tip:</b> Use <code>/session &lt;id&gt;</code> or <code>/selectsession</code> for menu")

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

func (h *CommandHandler) HandleDeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		_, err := h.tgBot.SendMessage(ctx, "❌ Please provide session ID: /deletesession &lt;id&gt;")
		return err
	}

	sessions, err := h.ocClient.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	var targetSession *opencode.Session
	for i := range sessions {
		if sessions[i].ID == sessionID {
			targetSession = &sessions[i]
			break
		}
	}

	if targetSession == nil {
		_, err := h.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Session %s not found", sessionID))
		return err
	}

	if err := h.ocClient.DeleteSession(sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	currentID := h.appState.GetCurrentSession()
	if currentID == sessionID {
		h.appState.SetCurrentSession("")
	}

	msg := fmt.Sprintf("🗑️ Deleted session: %s\n📝 Title: %s", sessionID, targetSession.Title)
	_, err = h.tgBot.SendMessage(ctx, msg)
	return err
}

func (h *CommandHandler) HandleDeleteSessionMenu(ctx context.Context) error {
	sessions, err := h.ocClient.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	primarySessions := []opencode.Session{}
	for _, sess := range sessions {
		if sess.ParentID == nil {
			primarySessions = append(primarySessions, sess)
		}
	}

	if len(primarySessions) == 0 {
		_, err := h.tgBot.SendMessage(ctx, "No sessions found.")
		return err
	}

	h.sessionCache = primarySessions
	h.sessionCacheKey = fmt.Sprintf("delcache_%d", time.Now().Unix())

	currentID := h.appState.GetCurrentSession()

	const sessionsPerPage = 8
	totalPages := (len(primarySessions) + sessionsPerPage - 1) / sessionsPerPage

	return h.showDeleteSessionPage(ctx, primarySessions, currentID, 0, totalPages)
}

func (h *CommandHandler) HandleDeleteSessionPageCallback(ctx context.Context, page int) error {
	if h.sessionCache == nil || len(h.sessionCache) == 0 {
		_, err := h.tgBot.SendMessage(ctx, "❌ Session list expired. Please use /deletesessions again.")
		return err
	}

	currentID := h.appState.GetCurrentSession()
	const sessionsPerPage = 8
	totalPages := (len(h.sessionCache) + sessionsPerPage - 1) / sessionsPerPage

	return h.showDeleteSessionPage(ctx, h.sessionCache, currentID, page, totalPages)
}

func (h *CommandHandler) showDeleteSessionPage(ctx context.Context, sessions []opencode.Session, currentID string, page, totalPages int) error {
	const sessionsPerPage = 8
	start := page * sessionsPerPage
	end := start + sessionsPerPage
	if end > len(sessions) {
		end = len(sessions)
	}

	pageSessions := sessions[start:end]
	keyboard := h.buildDeleteKeyboard(pageSessions, currentID, page, totalPages)

	text := fmt.Sprintf("🗑️ Select session to delete (Page %d/%d):", page+1, totalPages)
	_, err := h.tgBot.SendMessageWithKeyboard(ctx, text, keyboard)
	return err
}

func (h *CommandHandler) buildDeleteKeyboard(sessions []opencode.Session, currentID string, page, totalPages int) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	for _, sess := range sessions {
		label := sess.Title
		if label == "" {
			label = sess.Slug
		}

		dir := h.shortenDirectory(sess.Directory)
		label = fmt.Sprintf("%s [%s]", label, dir)

		if sess.ID == currentID {
			label = "✅ " + label
		}

		maxLen := 50
		if len(label) > maxLen {
			label = label[:maxLen-3] + "..."
		}

		button := models.InlineKeyboardButton{
			Text:         label,
			CallbackData: fmt.Sprintf("del:%s", sess.ID),
		}
		rows = append(rows, []models.InlineKeyboardButton{button})
	}

	var navRow []models.InlineKeyboardButton
	if page > 0 {
		navRow = append(navRow, models.InlineKeyboardButton{
			Text:         "⬅️ Previous",
			CallbackData: fmt.Sprintf("delpage:%d", page-1),
		})
	}
	if page < totalPages-1 {
		navRow = append(navRow, models.InlineKeyboardButton{
			Text:         "Next ➡️",
			CallbackData: fmt.Sprintf("delpage:%d", page+1),
		})
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	cancelRow := []models.InlineKeyboardButton{
		{
			Text:         "❌ Cancel",
			CallbackData: "delcancel",
		},
	}
	rows = append(rows, cancelRow)

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

func (h *CommandHandler) HandleDeleteConfirmCallback(ctx context.Context, sessionID string) error {
	sessions, err := h.ocClient.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	var targetSession *opencode.Session
	for i := range sessions {
		if sessions[i].ID == sessionID {
			targetSession = &sessions[i]
			break
		}
	}

	if targetSession == nil {
		_, err := h.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Session %s not found", sessionID))
		return err
	}

	confirmText := fmt.Sprintf("⚠️ Confirm deletion?\n\n📝 Title: %s\n🆔 ID: %s\n📂 Directory: %s",
		targetSession.Title, sessionID, targetSession.Directory)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Yes, delete", CallbackData: fmt.Sprintf("delconfirm:%s", sessionID)},
				{Text: "❌ Cancel", CallbackData: "delcancel"},
			},
		},
	}

	_, err = h.tgBot.SendMessageWithKeyboard(ctx, confirmText, keyboard)
	return err
}

func (h *CommandHandler) HandleDeleteExecuteCallback(ctx context.Context, sessionID string) error {
	sessions, err := h.ocClient.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	var targetSession *opencode.Session
	for i := range sessions {
		if sessions[i].ID == sessionID {
			targetSession = &sessions[i]
			break
		}
	}

	if targetSession == nil {
		_, err := h.tgBot.SendMessage(ctx, "❌ Session not found or already deleted")
		return err
	}

	if err := h.ocClient.DeleteSession(sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	currentID := h.appState.GetCurrentSession()
	if currentID == sessionID {
		h.appState.SetCurrentSession("")
	}

	msg := fmt.Sprintf("✅ Deleted successfully!\n\n📝 Title: %s\n🆔 ID: %s", targetSession.Title, sessionID)
	_, err = h.tgBot.SendMessage(ctx, msg)
	return err
}

func (h *CommandHandler) HandleSwitchSession(ctx context.Context, sessionID string) error {
	log.Printf("[CMD] HandleSwitchSession: switching to %s, statePtr=%p", sessionID, h.appState)
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
	log.Printf("[CMD] SetCurrentSession done, verifying: %s", h.appState.GetCurrentSession())
	msg := fmt.Sprintf("✅ Switched to session: %s (%s)", selectedSession.Slug, selectedSession.Title)
	_, err = h.tgBot.SendMessage(ctx, msg)
	return err
}

func (h *CommandHandler) HandleSelectSession(ctx context.Context) error {
	log.Printf("[CMD] HandleSelectSession: started")
	sessions, err := h.ocClient.ListSessions()
	if err != nil {
		log.Printf("[CMD] HandleSelectSession: ListSessions error: %v", err)
		return fmt.Errorf("list sessions: %w", err)
	}
	log.Printf("[CMD] HandleSelectSession: got %d total sessions", len(sessions))

	primarySessions := []opencode.Session{}
	for _, sess := range sessions {
		if sess.ParentID == nil {
			primarySessions = append(primarySessions, sess)
		}
	}
	log.Printf("[CMD] HandleSelectSession: found %d primary sessions", len(primarySessions))

	if len(primarySessions) == 0 {
		log.Printf("[CMD] HandleSelectSession: no primary sessions, sending error")
		_, err := h.tgBot.SendMessage(ctx, "No primary sessions found.")
		return err
	}

	h.sessionCache = primarySessions
	h.sessionCacheKey = fmt.Sprintf("cache_%d", time.Now().Unix())

	currentID := h.appState.GetCurrentSession()
	log.Printf("[CMD] HandleSelectSession: currentID=%s", currentID)

	const sessionsPerPage = 8
	totalPages := (len(primarySessions) + sessionsPerPage - 1) / sessionsPerPage
	log.Printf("[CMD] HandleSelectSession: showing page 0/%d", totalPages)

	return h.showSessionPage(ctx, primarySessions, currentID, 0, totalPages)
}

func (h *CommandHandler) HandleSessionPageCallback(ctx context.Context, page int) error {
	if h.sessionCache == nil || len(h.sessionCache) == 0 {
		_, err := h.tgBot.SendMessage(ctx, "❌ Session list expired. Please use /selectsession again.")
		return err
	}

	currentID := h.appState.GetCurrentSession()
	const sessionsPerPage = 8
	totalPages := (len(h.sessionCache) + sessionsPerPage - 1) / sessionsPerPage

	return h.showSessionPage(ctx, h.sessionCache, currentID, page, totalPages)
}

func (h *CommandHandler) showSessionPage(ctx context.Context, sessions []opencode.Session, currentID string, page, totalPages int) error {
	const sessionsPerPage = 8
	start := page * sessionsPerPage
	end := start + sessionsPerPage
	if end > len(sessions) {
		end = len(sessions)
	}

	log.Printf("[CMD] showSessionPage: page=%d, start=%d, end=%d, total=%d", page, start, end, len(sessions))
	pageSessions := sessions[start:end]

	keyboard := h.buildSessionKeyboard(pageSessions, currentID, page, totalPages)
	log.Printf("[CMD] showSessionPage: keyboard built with %d rows", len(keyboard.InlineKeyboard))

	msg := fmt.Sprintf("📋 <b>Select Session</b> (page %d/%d)", page+1, totalPages)
	log.Printf("[CMD] showSessionPage: sending message with keyboard...")
	msgID, err := h.tgBot.SendMessageWithKeyboard(ctx, msg, keyboard)
	if err != nil {
		log.Printf("[CMD] showSessionPage: SendMessageWithKeyboard failed: %v", err)
		return err
	}
	log.Printf("[CMD] showSessionPage: message sent successfully, msgID=%d", msgID)
	return nil
}

func (h *CommandHandler) buildSessionKeyboard(sessions []opencode.Session, currentID string, page, totalPages int) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	for _, sess := range sessions {
		dirDisplay := h.shortenDirectory(sess.Directory)

		var label string
		if sess.ID == currentID {
			label = fmt.Sprintf("🟢 %s [%s]", sess.Title, dirDisplay)
		} else {
			label = fmt.Sprintf("%s [%s]", sess.Title, dirDisplay)
		}

		if len(label) > 60 {
			runes := []rune(label)
			if len(runes) > 60 {
				label = string(runes[:57]) + "..."
			}
		}

		rows = append(rows, []models.InlineKeyboardButton{
			{
				Text:         label,
				CallbackData: "sess:" + sess.ID,
			},
		})
	}

	var navRow []models.InlineKeyboardButton
	if page > 0 {
		navRow = append(navRow, models.InlineKeyboardButton{
			Text:         "◀️ Prev",
			CallbackData: fmt.Sprintf("sesspage:%d", page-1),
		})
	}

	navRow = append(navRow, models.InlineKeyboardButton{
		Text:         fmt.Sprintf("%d/%d", page+1, totalPages),
		CallbackData: "noop",
	})

	if page < totalPages-1 {
		navRow = append(navRow, models.InlineKeyboardButton{
			Text:         "Next ▶️",
			CallbackData: fmt.Sprintf("sesspage:%d", page+1),
		})
	}

	rows = append(rows, navRow)

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

func (h *CommandHandler) shortenDirectory(dir string) string {
	if dir == "" || dir == "." {
		return "."
	}
	if dir == "/" {
		return "/"
	}

	if strings.HasPrefix(dir, "/Users/master/") {
		shortened := "~/" + strings.TrimPrefix(dir, "/Users/master/")
		if len(shortened) > 20 {
			parts := strings.Split(shortened, "/")
			if len(parts) > 2 {
				return "~/" + parts[len(parts)-1]
			}
		}
		return shortened
	}

	if strings.HasPrefix(dir, "/Volumes/") {
		parts := strings.Split(dir, "/")
		if len(parts) >= 3 {
			return "/" + parts[2] + "/..."
		}
	}

	if len(dir) > 20 {
		parts := strings.Split(dir, "/")
		return ".../" + parts[len(parts)-1]
	}

	return dir
}

func (h *CommandHandler) HandleStatus(ctx context.Context) error {
	sessionID := h.appState.GetCurrentSession()
	agent := h.appState.GetCurrentAgent()
	model := h.appState.GetCurrentModel()
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

	if model == "" {
		config, err := h.ocClient.GetConfig()
		if err == nil {
			if agents, ok := config["agent"].(map[string]interface{}); ok {
				if agentConfig, ok := agents[agent].(map[string]interface{}); ok {
					if m, ok := agentConfig["model"].(string); ok {
						model = m
					}
				}
			}
		}
		if model == "" {
			model = "(unknown)"
		}
	}

	sessionName := "(none)"
	sessionDir := "(none)"
	if sessionID != "" {
		sessions, err := h.ocClient.ListSessions()
		if err == nil {
			for _, s := range sessions {
				if s.ID == sessionID {
					sessionName = s.Title
					sessionDir = s.Directory
					break
				}
			}
		}
	} else {
		sessionID = "(none)"
	}

	lines := []string{
		"📊 Status:",
		"",
		fmt.Sprintf("Session: %s", sessionName),
		fmt.Sprintf("Session ID: %s", sessionID),
		fmt.Sprintf("Directory: %s", sessionDir),
		fmt.Sprintf("Agent: %s", agent),
		fmt.Sprintf("Model: %s", model),
		fmt.Sprintf("Status: %s", statusStr),
		fmt.Sprintf("OpenCode: %s", healthStr),
	}

	_, err = h.tgBot.SendMessage(ctx, strings.Join(lines, "\n"))
	return err
}

func (h *CommandHandler) HandleHelp(ctx context.Context) error {
	help := `🆘 Available Commands:

/newsession [title] - Create a new session
/sessions - List primary sessions
/selectsession - Select session from menu
/deletesessions - Delete sessions (interactive menu)
/session &lt;id&gt; - Switch to a session
/deletesession &lt;id&gt; - Delete a session directly
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

func formatTimeAgo(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if d < 7*24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	if d < 30*24*time.Hour {
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}
	months := int(d.Hours() / 24 / 30)
	if months == 1 {
		return "1 month ago"
	}
	if months < 12 {
		return fmt.Sprintf("%d months ago", months)
	}
	years := int(d.Hours() / 24 / 365)
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}
