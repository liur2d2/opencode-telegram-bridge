package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// agentOpenCodeClient interface for agent switching
type agentOpenCodeClient interface {
	GetAgents() ([]string, error)
}

// agentTelegramBot interface for sending messages and keyboards
type agentTelegramBot interface {
	SendMessage(ctx context.Context, text string) (int, error)
	SendMessageWithKeyboard(ctx context.Context, text string, keyboard [][]string) (int, error)
	EditMessage(ctx context.Context, msgID int, text string) error
	AnswerCallbackQuery(ctx context.Context, callbackID string) error
}

// AgentState for app state access
type agentAppState interface {
	SetCurrentAgent(agent string)
	GetCurrentAgent() string
}

// AgentHandler manages OHO agent switching
type AgentHandler struct {
	ocClient agentOpenCodeClient
	tgBot    agentTelegramBot
	appState agentAppState
}

// NewAgentHandler creates a new AgentHandler
func NewAgentHandler(ocClient agentOpenCodeClient, tgBot agentTelegramBot, appState agentAppState) *AgentHandler {
	return &AgentHandler{
		ocClient: ocClient,
		tgBot:    tgBot,
		appState: appState,
	}
}

// HandleSwitch processes the /switch command
// With arg: validates and sets agent
// Without arg: shows available agents as Inline Keyboard
func (h *AgentHandler) HandleSwitch(ctx context.Context, agent *string) error {
	// If no agent specified, show available agents
	if agent == nil || *agent == "" {
		agents, err := h.GetAvailableAgents(ctx)
		if err != nil {
			// Fallback to hardcoded list
			agents = getDefaultAgents()
		}

		// Build keyboard with agent buttons
		keyboard := buildAgentKeyboard(agents)

		msg := "🤖 Select an OHO Agent:\n\n"
		for i, a := range agents {
			msg += fmt.Sprintf("%d. %s\n", i+1, a)
		}

		_, err = h.tgBot.SendMessageWithKeyboard(ctx, msg, keyboard)
		return err
	}

	// Validate agent
	agents, err := h.GetAvailableAgents(ctx)
	if err != nil {
		agents = getDefaultAgents()
	}

	if !isValidAgent(*agent, agents) {
		// Show error with available options
		msg := fmt.Sprintf("❌ Unknown agent: %s\n\nAvailable agents:\n", *agent)
		for _, a := range agents {
			msg += fmt.Sprintf("• %s\n", a)
		}

		_, err := h.tgBot.SendMessage(ctx, msg)
		return err
	}

	// Set agent
	h.appState.SetCurrentAgent(*agent)

	// Confirm
	msg := fmt.Sprintf("🔄 Switched to %s", *agent)
	_, err = h.tgBot.SendMessage(ctx, msg)
	return err
}

// HandleAgentCallback processes agent selection from Inline Keyboard
func (h *AgentHandler) HandleAgentCallback(ctx context.Context, msgID int, agentName string) error {
	// Validate agent
	agents, err := h.GetAvailableAgents(ctx)
	if err != nil {
		agents = getDefaultAgents()
	}

	if !isValidAgent(agentName, agents) {
		return fmt.Errorf("invalid agent: %s", agentName)
	}

	// Set agent
	h.appState.SetCurrentAgent(agentName)

	// Edit message to confirm
	msg := fmt.Sprintf("🔄 Switched to %s", agentName)
	return h.tgBot.EditMessage(ctx, msgID, msg)
}

// GetAvailableAgents returns the list of available OHO agents
// Tries to fetch from OpenCode client first, then oh-my-opencode.json, then hardcoded list
func (h *AgentHandler) GetAvailableAgents(ctx context.Context) ([]string, error) {
	// Try to fetch from OpenCode client (if it implements GetAgents)
	if h.ocClient != nil {
		agents, err := h.ocClient.GetAgents()
		if err == nil && len(agents) > 0 {
			return agents, nil
		}
	}

	// Try to fetch from oh-my-opencode.json
	agents, err := loadAgentsFromConfig()
	if err == nil && len(agents) > 0 {
		return agents, nil
	}

	// Fallback: return hardcoded list
	return getDefaultAgents(), nil
}

// buildAgentKeyboard creates an Inline Keyboard from agent list
// Format: buttons in rows, callback_data = "a:{agent_name}" (shorthand for agent selection)
func buildAgentKeyboard(agents []string) [][]string {
	keyboard := make([][]string, 0)
	// Create 2 columns of buttons
	for i := 0; i < len(agents); i += 2 {
		row := make([]string, 0, 2)
		row = append(row, agents[i])
		if i+1 < len(agents) {
			row = append(row, agents[i+1])
		}
		keyboard = append(keyboard, row)
	}
	return keyboard
}

// isValidAgent checks if agent name is in the list
func isValidAgent(agentName string, agents []string) bool {
	for _, a := range agents {
		if a == agentName {
			return true
		}
	}
	return false
}

// getDefaultAgents returns hardcoded list from oh-my-opencode.json
func getDefaultAgents() []string {
	agents, _ := loadAgentsFromConfig()
	if len(agents) > 0 {
		return agents
	}

	// Ultimate fallback
	return []string{
		"sisyphus",
		"prometheus",
		"oracle",
		"atlas",
		"metis",
		"momus",
		"explore",
		"librarian",
		"hephaestus",
		"sisyphus-junior",
		"multimodal-looker",
	}
}

// loadAgentsFromConfig loads agent list from ~/.config/opencode/oh-my-opencode.json
func loadAgentsFromConfig() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".config", "opencode", "oh-my-opencode.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config struct {
		Agents map[string]interface{} `json:"agents"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	agents := make([]string, 0, len(config.Agents))
	for agentName := range config.Agents {
		agents = append(agents, agentName)
	}

	// Sort for consistent output
	sortAgents(agents)
	return agents, nil
}

// sortAgents sorts the agent list (simple bubble sort for consistent order)
func sortAgents(agents []string) {
	for i := 0; i < len(agents); i++ {
		for j := i + 1; j < len(agents); j++ {
			if agents[j] < agents[i] {
				agents[i], agents[j] = agents[j], agents[i]
			}
		}
	}
}

// Helper function to check if string contains substring
func stringContains(s, substr string) bool {
	return strings.Contains(s, substr)
}
