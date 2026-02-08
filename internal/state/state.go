package state

import "sync"

type SessionStatus int

const (
	SessionIdle SessionStatus = iota
	SessionBusy
	SessionError
)

type AppState struct {
	mu               sync.RWMutex
	currentSessionID string
	currentAgent     string
	currentModel     string
	chatAgentMap map[string]string
	sessionStatus    map[string]SessionStatus
}

func NewAppState() *AppState {
	return &AppState{
		currentAgent:  "sisyphus",
		sessionStatus: make(map[string]SessionStatus),
		chatAgentMap: make(map[string]string),
	}
}

func (s *AppState) SetCurrentSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentSessionID = sessionID
}

func (s *AppState) GetCurrentSession() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentSessionID
}

func (s *AppState) SetCurrentAgent(agent string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentAgent = agent
}

func (s *AppState) GetCurrentAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentAgent
}

func (s *AppState) SetSessionStatus(sessionID string, status SessionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionStatus[sessionID] = status
}

func (s *AppState) GetSessionStatus(sessionID string) SessionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if status, exists := s.sessionStatus[sessionID]; exists {
		return status
	}
	return SessionIdle
}

func (s *AppState) SetCurrentModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentModel = model
}

func (s *AppState) GetCurrentModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentModel
}

// SetChatAgent assigns an agent to a specific chat
func (s *AppState) SetChatAgent(chatID string, agent string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chatAgentMap[chatID] = agent
}

// GetChatAgent gets the agent assigned to a specific chat (empty if none)
func (s *AppState) GetChatAgent(chatID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.chatAgentMap[chatID]
}

// RemoveChatAgent removes the per-chat agent assignment
func (s *AppState) RemoveChatAgent(chatID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.chatAgentMap, chatID)
}

// ListChatAgents returns all per-chat agent assignments
func (s *AppState) ListChatAgents() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to avoid external modification
	result := make(map[string]string)
	for k, v := range s.chatAgentMap {
		result[k] = v
	}
	return result
}
