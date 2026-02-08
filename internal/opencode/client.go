package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the OpenCode SDK HTTP client
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient creates a new OpenCode client
func NewClient(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:54321"
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// NewClientWithTransport creates a new OpenCode client with optional custom transport
func NewClientWithTransport(config Config, transport *http.Transport) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:54321"
	}

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	if transport != nil {
		httpClient.Transport = transport
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
	}
}

func (c *Client) Health() (map[string]interface{}, error) {
	req, err := http.NewRequest(http.MethodGet, c.config.BaseURL+"/health", nil)
	if err != nil {
		return nil, fmt.Errorf("create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	var healthData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&healthData); err != nil {
		return map[string]interface{}{"healthy": true}, nil
	}

	return healthData, nil
}

// ListSessions retrieves all sessions (without directory filter for Telegram UI)
func (c *Client) ListSessions() ([]Session, error) {
	url := c.config.BaseURL + "/session"
	// Do NOT filter by directory - show all sessions across all directories

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create list sessions request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list sessions failed with status: %d", resp.StatusCode)
	}

	var sessions []Session
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("decode sessions: %w", err)
	}

	return sessions, nil
}

// CreateSession creates a new session
func (c *Client) CreateSession(title *string, parentID *string) (*Session, error) {
	reqBody := SessionCreateRequest{
		Title:    title,
		ParentID: parentID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal create session request: %w", err)
	}

	url := c.config.BaseURL + "/session"
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create session request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create session failed with status %d: %s", resp.StatusCode, string(body))
	}

	var session Session
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	return &session, nil
}

// DeleteSession deletes a session
func (c *Client) DeleteSession(sessionID string) error {
	url := c.config.BaseURL + "/session/" + sessionID
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create delete session request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete session failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AbortSession aborts a running session
func (c *Client) AbortSession(sessionID string) error {
	url := c.config.BaseURL + "/session/" + sessionID + "/abort"
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create abort session request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("abort session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("abort session failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendPrompt sends a prompt to a session with text
func (c *Client) SendPrompt(sessionID, text string, agent *string) (*SendPromptResponse, error) {
	return c.SendPromptWithParts(sessionID, []interface{}{
		TextPartInput{
			Type: "text",
			Text: text,
		},
	}, agent)
}

// SendPromptWithParts sends a prompt to a session with mixed parts (text + images)
func (c *Client) SendPromptWithParts(sessionID string, parts []interface{}, agent *string) (*SendPromptResponse, error) {
	reqBody := SendPromptRequest{
		Agent:  agent,
		System: nil,
		Parts:  parts,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal send prompt request: %w", err)
	}

	url := c.config.BaseURL + "/session/" + sessionID + "/message"
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create send prompt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send prompt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("send prompt failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response SendPromptResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode send prompt response: %w", err)
	}

	return &response, nil
}

func (c *Client) TriggerPrompt(sessionID, text string, agent *string) error {
	parts := []interface{}{
		TextPartInput{
			Type: "text",
			Text: text,
		},
	}

	reqBody := SendPromptRequest{
		Agent:  agent,
		System: nil,
		Parts:  parts,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal send prompt request: %w", err)
	}

	url := c.config.BaseURL + "/session/" + sessionID + "/message"
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	fmt.Printf("[TriggerPrompt] Sending to: %s, text length: %d\n", url, len(text))

	triggerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(triggerCtx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		fmt.Printf("[TriggerPrompt] Failed to create request: %v\n", err)
		return fmt.Errorf("create trigger request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		if triggerCtx.Err() == context.DeadlineExceeded {
			fmt.Printf("[TriggerPrompt] Timeout after %v (request likely sent, waiting for async processing via SSE)\n", elapsed)
			return nil
		}
		fmt.Printf("[TriggerPrompt] Request failed after %v: %v\n", elapsed, err)
		return fmt.Errorf("trigger prompt: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("[TriggerPrompt] Response status: %d (elapsed: %v)\n", resp.StatusCode, elapsed)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("[TriggerPrompt] Error response body: %s\n", string(body))
		return fmt.Errorf("trigger prompt failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListQuestions retrieves all pending questions
func (c *Client) ListQuestions() ([]QuestionRequest, error) {
	url := c.config.BaseURL + "/question"
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create list questions request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list questions failed with status %d: %s", resp.StatusCode, string(body))
	}

	var questions []QuestionRequest
	if err := json.NewDecoder(resp.Body).Decode(&questions); err != nil {
		return nil, fmt.Errorf("decode questions: %w", err)
	}

	return questions, nil
}

// ReplyQuestion replies to a question request
func (c *Client) ReplyQuestion(requestID string, answers []QuestionAnswer) error {
	reqBody := QuestionReplyRequest{
		Answers: answers,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal reply question request: %w", err)
	}

	url := c.config.BaseURL + "/question/" + requestID + "/reply"
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create reply question request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reply question: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reply question failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// RejectQuestion rejects a question request
func (c *Client) RejectQuestion(requestID string) error {
	url := c.config.BaseURL + "/question/" + requestID + "/reject"
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create reject question request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reject question: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reject question failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ReplyPermission responds to a permission request
func (c *Client) ReplyPermission(sessionID, permissionID string, response PermissionResponse) error {
	reqBody := PermissionReplyRequest{
		Reply: response,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal reply permission request: %w", err)
	}

	url := c.config.BaseURL + "/session/" + sessionID + "/permissions/" + permissionID
	if c.config.Directory != "" {
		url += "?directory=" + c.config.Directory
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create reply permission request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("reply permission: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reply permission failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) GetConfig() (map[string]interface{}, error) {
	req, err := http.NewRequest(http.MethodGet, c.config.BaseURL+"/config", nil)
	if err != nil {
		return nil, fmt.Errorf("create config request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get config failed with status: %d", resp.StatusCode)
	}

	var configData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&configData); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	return configData, nil
}

func (c *Client) GetMessages(sessionID string, limit int) ([]Message, error) {
	url := fmt.Sprintf("%s/session/%s/messages?limit=%d", c.config.BaseURL, sessionID, limit)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create get messages request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get messages failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}

	return messages, nil
}
