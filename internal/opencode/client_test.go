package opencode

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("Expected path /health, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	err := client.Health()
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}

func TestClient_ListSessions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session" {
			t.Errorf("Expected path /session, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		sessions := []Session{
			{
				ID:        "sess_123",
				ProjectID: "proj_456",
				Directory: "/test/dir",
				Title:     "Test Session",
				Version:   "1.0",
				Time: struct {
					Created    int64  `json:"created"`
					Updated    int64  `json:"updated"`
					Compacting *int64 `json:"compacting,omitempty"`
					Archived   *int64 `json:"archived,omitempty"`
				}{
					Created: 1234567890000,
					Updated: 1234567890000,
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(sessions)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "sess_123" {
		t.Errorf("Expected session ID sess_123, got %s", sessions[0].ID)
	}
}

func TestClient_CreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session" {
			t.Errorf("Expected path /session, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var req SessionCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}
		if req.Title != nil && *req.Title != "New Session" {
			t.Errorf("Expected title 'New Session', got %v", *req.Title)
		}

		session := Session{
			ID:        "sess_new",
			ProjectID: "proj_456",
			Directory: "/test/dir",
			Title:     "New Session",
			Version:   "1.0",
			Time: struct {
				Created    int64  `json:"created"`
				Updated    int64  `json:"updated"`
				Compacting *int64 `json:"compacting,omitempty"`
				Archived   *int64 `json:"archived,omitempty"`
			}{
				Created: 1234567890000,
				Updated: 1234567890000,
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(session)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	title := "New Session"
	session, err := client.CreateSession(&title, nil)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.ID != "sess_new" {
		t.Errorf("Expected session ID sess_new, got %s", session.ID)
	}
}

func TestClient_DeleteSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session/sess_123" {
			t.Errorf("Expected path /session/sess_123, got %s", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(true)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	err := client.DeleteSession("sess_123")
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
}

func TestClient_SendPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session/sess_123/message" {
			t.Errorf("Expected path /session/sess_123/message, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var req SendPromptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}
		if len(req.Parts) != 1 {
			t.Fatalf("Expected 1 part, got %d", len(req.Parts))
		}
		if req.Parts[0].Type != "text" {
			t.Errorf("Expected part type 'text', got %s", req.Parts[0].Type)
		}
		if req.Parts[0].Text != "Hello OpenCode" {
			t.Errorf("Expected text 'Hello OpenCode', got %s", req.Parts[0].Text)
		}
		if req.Agent != nil && *req.Agent != "build" {
			t.Errorf("Expected agent 'build', got %v", *req.Agent)
		}

		response := SendPromptResponse{
			Info: AssistantMessage{
				ID:        "msg_456",
				SessionID: "sess_123",
				Role:      "assistant",
				ParentID:  "msg_parent",
				Time: struct {
					Created   int64  `json:"created"`
					Completed *int64 `json:"completed,omitempty"`
				}{
					Created: 1234567890000,
				},
			},
			Parts: []interface{}{},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	agent := "build"
	resp, err := client.SendPrompt("sess_123", "Hello OpenCode", &agent)
	if err != nil {
		t.Fatalf("SendPrompt() error = %v", err)
	}
	if resp.Info.ID != "msg_456" {
		t.Errorf("Expected message ID msg_456, got %s", resp.Info.ID)
	}
}

func TestClient_ListQuestions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/question" {
			t.Errorf("Expected path /question, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		questions := []QuestionRequest{
			{
				ID:        "req_123",
				SessionID: "sess_456",
				Questions: []QuestionInfo{
					{
						Question: "Choose an option",
						Header:   "Choice",
						Options: []QuestionOption{
							{Label: "Yes", Description: "Proceed"},
							{Label: "No", Description: "Cancel"},
						},
					},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(questions)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	questions, err := client.ListQuestions()
	if err != nil {
		t.Fatalf("ListQuestions() error = %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("Expected 1 question, got %d", len(questions))
	}
	if questions[0].ID != "req_123" {
		t.Errorf("Expected question ID req_123, got %s", questions[0].ID)
	}
}

func TestClient_ReplyQuestion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/question/req_123/reply" {
			t.Errorf("Expected path /question/req_123/reply, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var req QuestionReplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}
		if len(req.Answers) != 1 {
			t.Fatalf("Expected 1 answer, got %d", len(req.Answers))
		}
		if len(req.Answers[0]) != 1 || req.Answers[0][0] != "Yes" {
			t.Errorf("Expected answer ['Yes'], got %v", req.Answers[0])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	answers := []QuestionAnswer{{"Yes"}}
	err := client.ReplyQuestion("req_123", answers)
	if err != nil {
		t.Fatalf("ReplyQuestion() error = %v", err)
	}
}

func TestClient_RejectQuestion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/question/req_123/reject" {
			t.Errorf("Expected path /question/req_123/reject, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	err := client.RejectQuestion("req_123")
	if err != nil {
		t.Fatalf("RejectQuestion() error = %v", err)
	}
}

func TestClient_ReplyPermission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session/sess_123/permissions/perm_456" {
			t.Errorf("Expected path /session/sess_123/permissions/perm_456, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var req PermissionReplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request body: %v", err)
		}
		if req.Reply != PermissionOnce {
			t.Errorf("Expected reply 'once', got %s", req.Reply)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL})
	err := client.ReplyPermission("sess_123", "perm_456", PermissionOnce)
	if err != nil {
		t.Fatalf("ReplyPermission() error = %v", err)
	}
}
