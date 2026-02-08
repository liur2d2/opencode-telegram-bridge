package state

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

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
	chatAgentMap     map[string]string
	sessionStatus    map[string]SessionStatus
	stateFile        string
}

func NewAppState(stateFile string) *AppState {
	state := &AppState{
		currentAgent:  "sisyphus",
		sessionStatus: make(map[string]SessionStatus),
		chatAgentMap:  make(map[string]string),
		stateFile:     stateFile,
	}

	if stateFile != "" {
		if sessionID, err := LoadSessionState(stateFile); err == nil && sessionID != "" {
			state.currentSessionID = sessionID
			log.Printf("[STATE] Loaded saved session: %s", sessionID)
		} else if err != nil {
			log.Printf("[STATE] Failed to load session state: %v", err)
		}
	}

	return state
}

func NewAppStateForTest() *AppState {
	return NewAppState("")
}

func (s *AppState) SetCurrentSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentSessionID = sessionID

	// Persist to file
	if s.stateFile != "" {
		if err := SaveSessionState(s.stateFile, sessionID); err != nil {
			log.Printf("[ERROR] Failed to save session state: %v", err)
		}
	}
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
	result := make(map[string]string)
	for k, v := range s.chatAgentMap {
		result[k] = v
	}
	return result
}

// GetAgentForChat returns the agent to use for a given chat ID
// Returns per-chat agent if set, otherwise returns currentAgent
func (s *AppState) GetAgentForChat(chatID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if agent := s.chatAgentMap[chatID]; agent != "" {
		return agent
	}
	return s.currentAgent
}

// LoadSessionState loads the saved session ID from a file.
// Returns empty string if file doesn't exist (first run).
func LoadSessionState(filePath string) (string, error) {
	// Expand ~ to home directory
	expanded, err := expandHome(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to expand path: %w", err)
	}

	// Read the file
	data, err := os.ReadFile(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - first run
			return "", nil
		}
		return "", fmt.Errorf("failed to read state file: %w", err)
	}

	// Return trimmed session ID
	sessionID := strings.TrimSpace(string(data))
	return sessionID, nil
}

// SaveSessionState saves the session ID to a file atomically.
// Uses write-to-temp-file + rename pattern to prevent corruption.
func SaveSessionState(filePath string, sessionID string) error {
	// Expand ~ to home directory
	expanded, err := expandHome(filePath)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temp file first
	tempFile := expanded + ".tmp"
	sessionBytes := []byte(sessionID)

	if err := os.WriteFile(tempFile, sessionBytes, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomically rename temp file to target file
	if err := os.Rename(tempFile, expanded); err != nil {
		// Clean up temp file if rename failed
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
