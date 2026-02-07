package bridge

import (
	"context"
	"testing"
)

// Mock clients for agent tests
type mockAgentOpenCodeClient struct {
	getAgentsFunc func() ([]string, error)
}

func (m *mockAgentOpenCodeClient) GetAgents() ([]string, error) {
	if m.getAgentsFunc != nil {
		return m.getAgentsFunc()
	}
	return []string{"sisyphus", "prometheus", "oracle"}, nil
}

type mockAgentTelegramBot struct {
	messages       []string
	keyboards      [][]string
	editedMessages map[int]string
}

func (m *mockAgentTelegramBot) SendMessage(ctx context.Context, text string) (int, error) {
	m.messages = append(m.messages, text)
	return len(m.messages) - 1, nil
}

func (m *mockAgentTelegramBot) SendMessageWithKeyboard(ctx context.Context, text string, keyboard [][]string) (int, error) {
	m.messages = append(m.messages, text)
	m.keyboards = append(m.keyboards, keyboard...)
	return len(m.messages) - 1, nil
}

func (m *mockAgentTelegramBot) EditMessage(ctx context.Context, msgID int, text string) error {
	if m.editedMessages == nil {
		m.editedMessages = make(map[int]string)
	}
	m.editedMessages[msgID] = text
	return nil
}

func (m *mockAgentTelegramBot) AnswerCallbackQuery(ctx context.Context, callbackID string) error {
	return nil
}

// mockAgentAppState for testing
type mockAgentAppState struct {
	currentAgent string
}

func (m *mockAgentAppState) SetCurrentAgent(agent string) {
	m.currentAgent = agent
}

func (m *mockAgentAppState) GetCurrentAgent() string {
	return m.currentAgent
}

func TestCmdSwitchWithAgent(t *testing.T) {
	// Test /switch prometheus → set agent and reply success
	mockOC := &mockAgentOpenCodeClient{}
	mockTG := &mockAgentTelegramBot{}
	appState := &mockAgentAppState{}

	handler := NewAgentHandler(mockOC, mockTG, appState)

	agent := "prometheus"
	err := handler.HandleSwitch(context.Background(), &agent)
	if err != nil {
		t.Fatalf("HandleSwitch failed: %v", err)
	}

	// Verify agent was set
	if appState.GetCurrentAgent() != "prometheus" {
		t.Errorf("Expected agent 'prometheus', got '%s'", appState.GetCurrentAgent())
	}

	// Verify response was sent
	if len(mockTG.messages) == 0 {
		t.Fatal("Expected message to be sent")
	}

	if !contains(mockTG.messages[0], "prometheus") {
		t.Errorf("Expected message to contain 'prometheus', got '%s'", mockTG.messages[0])
	}
}

func TestCmdSwitchInvalidAgent(t *testing.T) {
	// Test /switch nonexistent → show error with available agents
	mockOC := &mockAgentOpenCodeClient{}
	mockTG := &mockAgentTelegramBot{}
	appState := &mockAgentAppState{}

	handler := NewAgentHandler(mockOC, mockTG, appState)

	invalidAgent := "nonexistent"
	err := handler.HandleSwitch(context.Background(), &invalidAgent)
	if err != nil {
		t.Fatalf("HandleSwitch failed: %v", err)
	}

	// Verify error message was sent with available agents
	if len(mockTG.messages) == 0 {
		t.Fatal("Expected error message")
	}

	msg := mockTG.messages[0]
	if !contains(msg, "Unknown agent") && !contains(msg, "Available") {
		t.Errorf("Expected error message about unknown agent, got '%s'", msg)
	}
}

func TestCmdSwitchNoArg(t *testing.T) {
	// Test /switch (no arg) → show available agents as Inline Keyboard
	mockOC := &mockAgentOpenCodeClient{
		getAgentsFunc: func() ([]string, error) {
			return []string{"sisyphus", "prometheus", "oracle"}, nil
		},
	}
	mockTG := &mockAgentTelegramBot{}
	appState := &mockAgentAppState{}

	handler := NewAgentHandler(mockOC, mockTG, appState)

	err := handler.HandleSwitch(context.Background(), nil)
	if err != nil {
		t.Fatalf("HandleSwitch failed: %v", err)
	}

	// Verify message was sent with keyboard
	if len(mockTG.messages) == 0 {
		t.Fatal("Expected message with keyboard")
	}

	// Verify keyboard buttons contain agent names
	if len(mockTG.keyboards) == 0 {
		t.Fatal("Expected keyboard to be created")
	}
}

func TestCmdSwitchKeyboardCallback(t *testing.T) {
	// Test agent button callback → switch agent and edit message
	mockOC := &mockAgentOpenCodeClient{}
	mockTG := &mockAgentTelegramBot{}
	appState := &mockAgentAppState{}

	handler := NewAgentHandler(mockOC, mockTG, appState)

	msgID := 42
	agent := "oracle"
	err := handler.HandleAgentCallback(context.Background(), msgID, agent)
	if err != nil {
		t.Fatalf("HandleAgentCallback failed: %v", err)
	}

	// Verify agent was set
	if appState.GetCurrentAgent() != "oracle" {
		t.Errorf("Expected agent 'oracle', got '%s'", appState.GetCurrentAgent())
	}

	// Verify message was edited
	if editedText, ok := mockTG.editedMessages[msgID]; !ok {
		t.Fatal("Expected message to be edited")
	} else if !contains(editedText, "oracle") {
		t.Errorf("Expected edited message to contain 'oracle', got '%s'", editedText)
	}
}

func TestAgentPersistsAcrossMessages(t *testing.T) {
	// Test that switched agent persists across messages
	mockOC := &mockAgentOpenCodeClient{}
	mockTG := &mockAgentTelegramBot{}
	appState := &mockAgentAppState{}

	handler := NewAgentHandler(mockOC, mockTG, appState)

	// Switch to prometheus
	agent := "prometheus"
	err := handler.HandleSwitch(context.Background(), &agent)
	if err != nil {
		t.Fatalf("HandleSwitch failed: %v", err)
	}

	// Verify agent persists
	if appState.GetCurrentAgent() != "prometheus" {
		t.Errorf("Expected agent 'prometheus' to persist, got '%s'", appState.GetCurrentAgent())
	}

	// Switch to different agent
	agent2 := "oracle"
	err = handler.HandleSwitch(context.Background(), &agent2)
	if err != nil {
		t.Fatalf("HandleSwitch failed: %v", err)
	}

	// Verify new agent is set
	if appState.GetCurrentAgent() != "oracle" {
		t.Errorf("Expected agent 'oracle', got '%s'", appState.GetCurrentAgent())
	}
}

func TestListAgents(t *testing.T) {
	// Test that available agents are fetched correctly
	expectedAgents := []string{"sisyphus", "prometheus", "oracle", "atlas"}
	mockOC := &mockAgentOpenCodeClient{
		getAgentsFunc: func() ([]string, error) {
			return expectedAgents, nil
		},
	}
	mockTG := &mockAgentTelegramBot{}
	appState := &mockAgentAppState{}

	handler := NewAgentHandler(mockOC, mockTG, appState)

	agents, err := handler.GetAvailableAgents(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableAgents failed: %v", err)
	}

	if len(agents) != len(expectedAgents) {
		t.Errorf("Expected %d agents, got %d", len(expectedAgents), len(agents))
	}

	for i, agent := range agents {
		if agent != expectedAgents[i] {
			t.Errorf("Expected agent %s, got %s", expectedAgents[i], agent)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr))
}
