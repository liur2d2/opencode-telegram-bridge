package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthStatus represents the overall health state
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
)

// HealthMonitor tracks system health metrics
type HealthMonitor struct {
	mu             sync.RWMutex
	sseConnected   bool
	lastEventTime  time.Time
	activeSessions int
	startTime      time.Time
	lastEventType  string
	eventCount     int64
	reconnectCount int
}

// HealthReport contains the current health status
type HealthReport struct {
	Status             HealthStatus `json:"status"`
	SSEConnected       bool         `json:"sse_connected"`
	LastEventTime      string       `json:"last_event_time"`
	TimeSinceLastEvent string       `json:"time_since_last_event"`
	ActiveSessions     int          `json:"active_sessions"`
	Uptime             string       `json:"uptime"`
	LastEventType      string       `json:"last_event_type,omitempty"`
	TotalEvents        int64        `json:"total_events"`
	ReconnectCount     int          `json:"reconnect_count"`
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		startTime: time.Now(),
	}
}

// SetSSEConnected updates SSE connection status
func (h *HealthMonitor) SetSSEConnected(connected bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sseConnected = connected
	if !connected {
		h.reconnectCount++
	}
}

// RecordEvent records an SSE event
func (h *HealthMonitor) RecordEvent(eventType string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastEventTime = time.Now()
	h.lastEventType = eventType
	h.eventCount++
}

// SetActiveSessions updates the active session count
func (h *HealthMonitor) SetActiveSessions(count int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.activeSessions = count
}

// GetStatus determines overall health status
func (h *HealthMonitor) GetStatus() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Unhealthy: SSE not connected
	if !h.sseConnected {
		return StatusUnhealthy
	}

	// Degraded: No events in last 5 minutes (but connected)
	if !h.lastEventTime.IsZero() && time.Since(h.lastEventTime) > 5*time.Minute {
		return StatusDegraded
	}

	// Degraded: Multiple reconnects (instability)
	if h.reconnectCount > 3 {
		return StatusDegraded
	}

	return StatusHealthy
}

// GetReport generates a health report
func (h *HealthMonitor) GetReport() HealthReport {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var lastEventStr, timeSinceLastEvent string
	if !h.lastEventTime.IsZero() {
		lastEventStr = h.lastEventTime.Format(time.RFC3339)
		timeSinceLastEvent = time.Since(h.lastEventTime).Round(time.Second).String()
	} else {
		lastEventStr = "never"
		timeSinceLastEvent = "N/A"
	}

	return HealthReport{
		Status:             h.GetStatusLocked(),
		SSEConnected:       h.sseConnected,
		LastEventTime:      lastEventStr,
		TimeSinceLastEvent: timeSinceLastEvent,
		ActiveSessions:     h.activeSessions,
		Uptime:             time.Since(h.startTime).Round(time.Second).String(),
		LastEventType:      h.lastEventType,
		TotalEvents:        h.eventCount,
		ReconnectCount:     h.reconnectCount,
	}
}

// GetStatusLocked returns status without acquiring lock (caller must hold lock)
func (h *HealthMonitor) GetStatusLocked() HealthStatus {
	if !h.sseConnected {
		return StatusUnhealthy
	}

	if !h.lastEventTime.IsZero() && time.Since(h.lastEventTime) > 5*time.Minute {
		return StatusDegraded
	}

	if h.reconnectCount > 3 {
		return StatusDegraded
	}

	return StatusHealthy
}

// ServeHTTP implements http.Handler for the /health endpoint
func (h *HealthMonitor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	report := h.GetReport()

	w.Header().Set("Content-Type", "application/json")

	// Set HTTP status based on health
	switch report.Status {
	case StatusHealthy:
		w.WriteHeader(http.StatusOK)
	case StatusDegraded:
		w.WriteHeader(http.StatusOK) // Still operational
	case StatusUnhealthy:
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(report)
}
