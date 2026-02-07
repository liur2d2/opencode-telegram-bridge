package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SSEConsumer consumes Server-Sent Events from OpenCode
type SSEConsumer struct {
	config     Config
	httpClient *http.Client
	eventChan  chan Event
	closeChan  chan struct{}
	closeOnce  sync.Once
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewSSEConsumer creates a new SSE consumer
func NewSSEConsumer(config Config) *SSEConsumer {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:54321"
	}

	return &SSEConsumer{
		config: config,
		httpClient: &http.Client{
			Timeout: 0, // No timeout for SSE connections
		},
		eventChan: make(chan Event, 100), // Buffer events
		closeChan: make(chan struct{}),
	}
}

// Events returns the channel for receiving events
func (s *SSEConsumer) Events() <-chan Event {
	return s.eventChan
}

// Connect establishes the SSE connection with automatic reconnection
func (s *SSEConsumer) Connect(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	go s.reconnectLoop()

	return nil
}

// Close closes the SSE connection
func (s *SSEConsumer) Close() {
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		close(s.closeChan)
		close(s.eventChan)
	})
}

// reconnectLoop handles connection and reconnection with exponential backoff
func (s *SSEConsumer) reconnectLoop() {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.closeChan:
			return
		default:
		}

		// Attempt connection
		err := s.connect()
		if err != nil {
			// Check if context is cancelled (intentional close)
			select {
			case <-s.ctx.Done():
				return
			case <-s.closeChan:
				return
			default:
			}

			// Wait before reconnecting with exponential backoff
			select {
			case <-time.After(backoff):
				// Double backoff for next attempt, cap at maxBackoff
				backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
			case <-s.ctx.Done():
				return
			case <-s.closeChan:
				return
			}
			continue
		}

		// Connection successful, reset backoff
		backoff = 1 * time.Second
	}
}

// connect establishes a single SSE connection
func (s *SSEConsumer) connect() error {
	url := s.config.BaseURL + "/event"
	if s.config.Directory != "" {
		url += "?directory=" + s.config.Directory
	}

	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect to SSE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE connection failed with status: %d", resp.StatusCode)
	}

	// Read and parse events
	return s.readEvents(resp.Body)
}

// readEvents reads and parses SSE events from the connection
func (s *SSEConsumer) readEvents(r io.Reader) error {
	scanner := bufio.NewScanner(r)

	var eventType string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line indicates end of event
		if line == "" {
			if eventType != "" && len(dataLines) > 0 {
				// Parse and send event
				data := strings.Join(dataLines, "\n")
				if err := s.parseAndSendEvent(eventType, data); err != nil {
					// Log error but continue processing
					fmt.Printf("Error parsing event: %v\n", err)
				}
			}
			// Reset for next event
			eventType = ""
			dataLines = nil
			continue
		}

		// Parse field
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(line[5:]))
		}
		// Ignore comments and other fields
	}

	return scanner.Err()
}

// parseAndSendEvent parses event data and sends it to the channel
func (s *SSEConsumer) parseAndSendEvent(eventType, data string) error {
	event := Event{
		Type:      eventType,
		Timestamp: time.Now(),
	}

	// Parse event-specific properties
	switch eventType {
	case "question.asked":
		var evt EventQuestionAsked
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal question.asked: %w", err)
		}
		event.Properties = &evt.Properties

	case "question.replied":
		var evt EventQuestionReplied
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal question.replied: %w", err)
		}
		event.Properties = &evt.Properties

	case "question.rejected":
		var evt EventQuestionRejected
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal question.rejected: %w", err)
		}
		event.Properties = &evt.Properties

	case "permission.asked":
		var evt EventPermissionAsked
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal permission.asked: %w", err)
		}
		event.Properties = &evt.Properties

	case "permission.replied":
		var evt EventPermissionReplied
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal permission.replied: %w", err)
		}
		event.Properties = &evt.Properties

	case "message.updated":
		var evt EventMessageUpdated
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal message.updated: %w", err)
		}
		event.Properties = &evt.Properties

	case "message.part.updated":
		var evt EventMessagePartUpdated
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal message.part.updated: %w", err)
		}
		event.Properties = &evt.Properties

	case "session.idle":
		var evt EventSessionIdle
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal session.idle: %w", err)
		}
		event.Properties = &evt.Properties

	case "session.error":
		var evt EventSessionError
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			return fmt.Errorf("unmarshal session.error: %w", err)
		}
		event.Properties = &evt.Properties

	default:
		// Generic event, parse as map
		var props map[string]interface{}
		if err := json.Unmarshal([]byte(data), &props); err != nil {
			return fmt.Errorf("unmarshal generic event: %w", err)
		}
		// Extract properties field if it exists
		if p, ok := props["properties"]; ok {
			event.Properties = p
		} else {
			event.Properties = props
		}
	}

	// Send event to channel (non-blocking)
	select {
	case s.eventChan <- event:
	case <-s.ctx.Done():
		return s.ctx.Err()
	case <-s.closeChan:
		return nil
	default:
		// Channel full, drop event
		fmt.Printf("Warning: event channel full, dropping event: %s\n", eventType)
	}

	return nil
}
