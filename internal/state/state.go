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
	sessionStatus    map[string]SessionStatus
}

func NewAppState() *AppState {
	return &AppState{
		currentAgent:  "sisyphus",
		sessionStatus: make(map[string]SessionStatus),
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
