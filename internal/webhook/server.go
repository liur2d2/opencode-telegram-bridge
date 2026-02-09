package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/user/opencode-telegram/internal/opencode"
)

type WebhookEvent struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

type EventHandler interface {
	HandleSSEEvent(event opencode.Event)
}

type Server struct {
	addr    string
	handler EventHandler
	server  *http.Server
}

func NewServer(addr string, handler EventHandler) *Server {
	return &Server{
		addr:    addr,
		handler: handler,
	}
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("[WEBHOOK] Failed to decode event: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[WEBHOOK] Received event: type=%s, timestamp=%d", event.Type, event.Timestamp)

	sseEvent, err := s.convertToSSEEvent(event)
	if err != nil {
		log.Printf("[WEBHOOK] Failed to convert event: %v", err)
		http.Error(w, "Invalid event format", http.StatusBadRequest)
		return
	}

	s.handler.HandleSSEEvent(*sseEvent)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) convertToSSEEvent(webhook WebhookEvent) (*opencode.Event, error) {
	switch webhook.Type {
	case "session.created":
		var data struct {
			SessionID string `json:"sessionId"`
			Directory string `json:"directory"`
		}
		if err := json.Unmarshal(webhook.Data, &data); err != nil {
			return nil, err
		}

		return &opencode.Event{
			Type:       "session.created",
			Properties: nil,
			Timestamp:  time.Unix(0, webhook.Timestamp*1e6),
		}, nil

	case "message.updated":
		var data struct {
			SessionID string  `json:"sessionId"`
			MessageID string  `json:"messageId"`
			Role      string  `json:"role"`
			Content   *string `json:"content"`
		}
		if err := json.Unmarshal(webhook.Data, &data); err != nil {
			return nil, err
		}

		completed := time.Now().UnixMilli()
		event := &opencode.Event{
			Type: "message.updated",
			Properties: &opencode.EventMessageUpdated{
				Type: "message.updated",
				Properties: struct {
					Info *struct {
						ID        string `json:"id"`
						SessionID string `json:"sessionID"`
						Role      string `json:"role"`
						ParentID  string `json:"parentID,omitempty"`
						Time      struct {
							Created   int64  `json:"created"`
							Completed *int64 `json:"completed,omitempty"`
						} `json:"time"`
						ModelID    string `json:"modelID,omitempty"`
						ProviderID string `json:"providerID,omitempty"`
						Mode       string `json:"mode,omitempty"`
						Agent      string `json:"agent,omitempty"`
					} `json:"info,omitempty"`
					Parts   []interface{} `json:"parts,omitempty"`
					Content *string       `json:"content,omitempty"`
				}{
					Info: &struct {
						ID        string `json:"id"`
						SessionID string `json:"sessionID"`
						Role      string `json:"role"`
						ParentID  string `json:"parentID,omitempty"`
						Time      struct {
							Created   int64  `json:"created"`
							Completed *int64 `json:"completed,omitempty"`
						} `json:"time"`
						ModelID    string `json:"modelID,omitempty"`
						ProviderID string `json:"providerID,omitempty"`
						Mode       string `json:"mode,omitempty"`
						Agent      string `json:"agent,omitempty"`
					}{
						ID:        data.MessageID,
						SessionID: data.SessionID,
						Role:      data.Role,
						Time: struct {
							Created   int64  `json:"created"`
							Completed *int64 `json:"completed,omitempty"`
						}{
							Created:   webhook.Timestamp,
							Completed: &completed,
						},
					},
					Content: data.Content,
				},
			},
			Timestamp: time.Unix(0, webhook.Timestamp*1e6),
		}

		if data.Content != nil {
			log.Printf("[WEBHOOK] Message content received, length=%d", len(*data.Content))
		}

		return event, nil

	case "session.idle":
		var data struct {
			SessionID    string  `json:"sessionId"`
			SessionTitle string  `json:"sessionTitle"`
			Content      *string `json:"content"`
		}
		if err := json.Unmarshal(webhook.Data, &data); err != nil {
			return nil, err
		}

		if data.Content != nil {
			log.Printf("[WEBHOOK] Session idle content received, session=%s, length=%d", data.SessionID, len(*data.Content))
		}

		return &opencode.Event{
			Type: "session.idle",
			Properties: &opencode.EventSessionIdle{
				Type: "session.idle",
				Properties: struct {
					SessionID string  `json:"sessionID"`
					Content   *string `json:"content,omitempty"`
				}{
					SessionID: data.SessionID,
					Content:   data.Content,
				},
			},
			Timestamp: time.Unix(0, webhook.Timestamp*1e6),
		}, nil

	case "question.asked":
		var evt opencode.EventQuestionAsked
		if err := json.Unmarshal(webhook.Data, &evt); err != nil {
			return nil, fmt.Errorf("unmarshal question.asked: %w", err)
		}

		log.Printf("[WEBHOOK] question.asked received, requestID=%s, sessionID=%s, questions=%d",
			evt.Properties.ID, evt.Properties.SessionID, len(evt.Properties.Questions))

		return &opencode.Event{
			Type:       "question.asked",
			Properties: &evt,
			Timestamp:  time.Unix(0, webhook.Timestamp*1e6),
		}, nil

	case "permission.asked":
		var evt opencode.EventPermissionAsked
		if err := json.Unmarshal(webhook.Data, &evt); err != nil {
			return nil, fmt.Errorf("unmarshal permission.asked: %w", err)
		}

		log.Printf("[WEBHOOK] permission.asked received, permissionID=%s", evt.Properties.ID)

		return &opencode.Event{
			Type:       "permission.asked",
			Properties: &evt,
			Timestamp:  time.Unix(0, webhook.Timestamp*1e6),
		}, nil

	default:
		return nil, fmt.Errorf("unknown event type: %s", webhook.Type)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", s.handleWebhook)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(shutdownCtx)
	}()

	log.Printf("[WEBHOOK] Starting webhook server on %s", s.addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("webhook server error: %w", err)
	}
	return nil
}
