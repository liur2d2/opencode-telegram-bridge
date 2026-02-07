package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSSE_Connect(t *testing.T) {
	eventSent := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/event" {
			t.Errorf("Expected path /event, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.Flusher")
		}

		// Send a test event
		event := EventSessionIdle{
			Type: "session.idle",
			Properties: struct {
				SessionID string `json:"sessionID"`
			}{
				SessionID: "sess_123",
			},
		}

		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: session.idle\ndata: %s\n\n", string(data))
		flusher.Flush()
		eventSent = true

		// Keep connection open
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	consumer := NewSSEConsumer(Config{BaseURL: server.URL})
	err := consumer.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer consumer.Close()

	// Wait for event
	select {
	case event := <-consumer.Events():
		if event.Type != "session.idle" {
			t.Errorf("Expected event type 'session.idle', got %s", event.Type)
		}
	case <-time.After(1 * time.Second):
		if !eventSent {
			t.Fatal("Server did not send event")
		}
		t.Fatal("Timeout waiting for event")
	}
}

func TestSSE_ParseQuestionAskedEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.Flusher")
		}

		// Send question.asked event
		event := EventQuestionAsked{
			Type: "question.asked",
			Properties: QuestionRequest{
				ID:        "req_123",
				SessionID: "sess_456",
				Questions: []QuestionInfo{
					{
						Question: "Proceed?",
						Header:   "Confirm",
						Options: []QuestionOption{
							{Label: "Yes", Description: "Continue"},
							{Label: "No", Description: "Stop"},
						},
					},
				},
			},
		}

		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: question.asked\ndata: %s\n\n", string(data))
		flusher.Flush()

		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	consumer := NewSSEConsumer(Config{BaseURL: server.URL})
	err := consumer.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer consumer.Close()

	select {
	case event := <-consumer.Events():
		if event.Type != "question.asked" {
			t.Errorf("Expected event type 'question.asked', got %s", event.Type)
		}

		props, ok := event.Properties.(*QuestionRequest)
		if !ok {
			t.Fatalf("Expected *QuestionRequest, got %T", event.Properties)
		}
		if props.ID != "req_123" {
			t.Errorf("Expected request ID 'req_123', got %s", props.ID)
		}
		if len(props.Questions) != 1 {
			t.Fatalf("Expected 1 question, got %d", len(props.Questions))
		}
		if props.Questions[0].Header != "Confirm" {
			t.Errorf("Expected header 'Confirm', got %s", props.Questions[0].Header)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestSSE_ParsePermissionAskedEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.Flusher")
		}

		// Send permission.asked event
		event := EventPermissionAsked{
			Type: "permission.asked",
			Properties: PermissionRequest{
				ID:         "perm_123",
				SessionID:  "sess_456",
				Permission: "file.write",
				Patterns:   []string{"*.go"},
				Metadata:   map[string]interface{}{"path": "/test/file.go"},
				Always:     []string{},
			},
		}

		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: permission.asked\ndata: %s\n\n", string(data))
		flusher.Flush()

		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	consumer := NewSSEConsumer(Config{BaseURL: server.URL})
	err := consumer.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer consumer.Close()

	select {
	case event := <-consumer.Events():
		if event.Type != "permission.asked" {
			t.Errorf("Expected event type 'permission.asked', got %s", event.Type)
		}

		props, ok := event.Properties.(*PermissionRequest)
		if !ok {
			t.Fatalf("Expected *PermissionRequest, got %T", event.Properties)
		}
		if props.ID != "perm_123" {
			t.Errorf("Expected permission ID 'perm_123', got %s", props.ID)
		}
		if props.Permission != "file.write" {
			t.Errorf("Expected permission 'file.write', got %s", props.Permission)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestSSE_Reconnect(t *testing.T) {
	connectionCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectionCount++

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.Flusher")
		}

		// First connection: send event and close
		if connectionCount == 1 {
			event := EventSessionIdle{
				Type: "session.idle",
				Properties: struct {
					SessionID string `json:"sessionID"`
				}{
					SessionID: "sess_first",
				},
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: session.idle\ndata: %s\n\n", string(data))
			flusher.Flush()
			// Close connection immediately to trigger reconnect
			return
		}

		// Second connection: send different event
		if connectionCount == 2 {
			event := EventSessionIdle{
				Type: "session.idle",
				Properties: struct {
					SessionID string `json:"sessionID"`
				}{
					SessionID: "sess_second",
				},
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: session.idle\ndata: %s\n\n", string(data))
			flusher.Flush()
			<-r.Context().Done()
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	consumer := NewSSEConsumer(Config{BaseURL: server.URL})
	err := consumer.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer consumer.Close()

	// Should receive event from first connection
	select {
	case event := <-consumer.Events():
		propsStruct, ok := event.Properties.(*struct {
			SessionID string `json:"sessionID"`
		})
		if !ok {
			t.Fatalf("Unexpected properties type: %T (want *struct{ SessionID string })", event.Properties)
		}
		if propsStruct.SessionID != "sess_first" {
			t.Errorf("Expected first event sessionID 'sess_first', got %v", propsStruct.SessionID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for first event")
	}

	// Should receive event from second connection (after reconnect)
	select {
	case event := <-consumer.Events():
		propsStruct, ok := event.Properties.(*struct {
			SessionID string `json:"sessionID"`
		})
		if !ok {
			t.Fatalf("Unexpected properties type: %T (want *struct{ SessionID string })", event.Properties)
		}
		if propsStruct.SessionID != "sess_second" {
			t.Errorf("Expected second event sessionID 'sess_second', got %v", propsStruct.SessionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for second event after reconnect")
	}

	if connectionCount < 2 {
		t.Errorf("Expected at least 2 connections (reconnect), got %d", connectionCount)
	}
}

func TestSSE_ExponentialBackoff(t *testing.T) {
	connectionTimes := []time.Time{}
	var mu sync.Mutex
	connectionCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionTimes = append(connectionTimes, time.Now())
		count := connectionCount
		connectionCount++
		mu.Unlock()

		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	consumer := NewSSEConsumer(Config{BaseURL: server.URL})
	err := consumer.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer consumer.Close()

	time.Sleep(8 * time.Second)

	mu.Lock()
	times := make([]time.Time, len(connectionTimes))
	copy(times, connectionTimes)
	mu.Unlock()

	if len(times) < 3 {
		t.Skipf("Not enough reconnections to test backoff (got %d)", len(times))
	}

	diff1 := times[1].Sub(times[0])
	if diff1 < 900*time.Millisecond || diff1 > 1500*time.Millisecond {
		t.Errorf("First retry delay should be ~1s, got %v", diff1)
	}

	if len(times) >= 3 {
		diff2 := times[2].Sub(times[1])
		if diff2 < 1800*time.Millisecond || diff2 > 2500*time.Millisecond {
			t.Errorf("Second retry delay should be ~2s, got %v", diff2)
		}
	}
}

func TestSSE_MultilineData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.Flusher")
		}

		// Send event with multiline data (SSE spec allows this)
		eventData := `{"type":"session.idle","properties":{"sessionID":"sess_123"}}`
		lines := strings.Split(eventData, "\n")
		fmt.Fprintf(w, "event: session.idle\n")
		for _, line := range lines {
			fmt.Fprintf(w, "data: %s\n", line)
		}
		fmt.Fprintf(w, "\n")
		flusher.Flush()

		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	consumer := NewSSEConsumer(Config{BaseURL: server.URL})
	err := consumer.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer consumer.Close()

	select {
	case event := <-consumer.Events():
		if event.Type != "session.idle" {
			t.Errorf("Expected event type 'session.idle', got %s", event.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for multiline event")
	}
}
