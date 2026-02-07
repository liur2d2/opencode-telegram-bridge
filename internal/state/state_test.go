package state

import (
	"sync"
	"testing"
	"time"
)

// TestNewState tests that AppState can be created with proper defaults
func TestNewState(t *testing.T) {
	state := NewAppState()
	if state == nil {
		t.Fatal("NewAppState returned nil")
	}

	if agent := state.GetCurrentAgent(); agent != "sisyphus" {
		t.Errorf("Default agent should be 'sisyphus', got %q", agent)
	}

	if session := state.GetCurrentSession(); session != "" {
		t.Errorf("Initial current session should be empty, got %q", session)
	}
}

// TestSetCurrentSession tests setting and getting current session ID
func TestSetCurrentSession(t *testing.T) {
	state := NewAppState()
	sessionID := "ses_abc123"

	state.SetCurrentSession(sessionID)
	if got := state.GetCurrentSession(); got != sessionID {
		t.Errorf("GetCurrentSession() = %q, want %q", got, sessionID)
	}
}

// TestSetCurrentAgent tests setting and getting current agent
func TestSetCurrentAgent(t *testing.T) {
	state := NewAppState()
	agent := "prometheus"

	state.SetCurrentAgent(agent)
	if got := state.GetCurrentAgent(); got != agent {
		t.Errorf("GetCurrentAgent() = %q, want %q", got, agent)
	}
}

// TestSessionStatus tests setting and getting session status
func TestSessionStatus(t *testing.T) {
	state := NewAppState()
	sessionID := "ses_test"

	state.SetSessionStatus(sessionID, SessionBusy)
	if got := state.GetSessionStatus(sessionID); got != SessionBusy {
		t.Errorf("GetSessionStatus() = %d, want %d", got, SessionBusy)
	}

	state.SetSessionStatus(sessionID, SessionIdle)
	if got := state.GetSessionStatus(sessionID); got != SessionIdle {
		t.Errorf("After update, GetSessionStatus() = %d, want %d", got, SessionIdle)
	}

	// Test non-existent session should return SessionIdle
	if got := state.GetSessionStatus("nonexistent"); got != SessionIdle {
		t.Errorf("Non-existent session should return SessionIdle, got %d", got)
	}
}

// TestConcurrentAccess tests that state is goroutine-safe
func TestConcurrentAccess(t *testing.T) {
	state := NewAppState()
	errors := make(chan string, 100)
	var wg sync.WaitGroup

	// Test concurrent writes and reads
	const numGoroutines = 50
	const operationsPerGoroutine = 20

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				// Concurrent writes
				state.SetCurrentSession("ses_concurrent")
				state.SetCurrentAgent("test_agent")
				state.SetSessionStatus("ses_concurrent", SessionBusy)

				// Concurrent reads
				_ = state.GetCurrentSession()
				_ = state.GetCurrentAgent()
				_ = state.GetSessionStatus("ses_concurrent")
			}
		}(i)
	}

	wg.Wait()

	if len(errors) > 0 {
		t.Fatalf("Concurrent access detected %d errors", len(errors))
	}
}

// TestRegistryRegister tests registering full IDs and getting short keys
func TestRegistryRegister(t *testing.T) {
	registry := NewIDRegistry()

	fullID := "qst_long_uuid_that_is_much_longer_than_64_bytes_to_test_abbreviation"
	shortKey := registry.Register(fullID, "q", "0")

	// Check that short key is returned
	if shortKey == "" {
		t.Error("Register should return a non-empty short key")
	}

	// Check that short key is reasonably short (≤ 64 bytes)
	if len(shortKey) > 64 {
		t.Errorf("Short key length %d exceeds 64 bytes", len(shortKey))
	}

	// Check format: should be "prefix:counter:suffix"
	// e.g., "q:1:0" or "p:2:once"
	expected := "q:1:0"
	if shortKey != expected {
		t.Errorf("First short key should be %q, got %q", expected, shortKey)
	}
}

// TestRegistryLookup tests looking up full IDs from short keys
func TestRegistryLookup(t *testing.T) {
	registry := NewIDRegistry()

	fullID := "permission_long_id_xyz"
	shortKey := registry.Register(fullID, "p", "once")

	retrieved, found := registry.Lookup(shortKey)
	if !found {
		t.Fatalf("Lookup should find registered key %q", shortKey)
	}

	if retrieved != fullID {
		t.Errorf("Lookup returned %q, want %q", retrieved, fullID)
	}
}

// TestRegistryShortKeyFormat tests that generated keys follow expected format
func TestRegistryShortKeyFormat(t *testing.T) {
	registry := NewIDRegistry()

	testCases := []struct {
		fullID string
		prefix string
		suffix string
		want   string
	}{
		{
			fullID: "question_1",
			prefix: "q",
			suffix: "0",
			want:   "q:1:0",
		},
		{
			fullID: "permission_1",
			prefix: "p",
			suffix: "once",
			want:   "p:2:once",
		},
		{
			fullID: "another_question",
			prefix: "q",
			suffix: "1",
			want:   "q:3:1",
		},
	}

	for _, tc := range testCases {
		got := registry.Register(tc.fullID, tc.prefix, tc.suffix)
		if got != tc.want {
			t.Errorf("Register(%q, %q, %q) = %q, want %q",
				tc.fullID, tc.prefix, tc.suffix, got, tc.want)
		}
	}
}

// TestRegistryCleanup tests TTL-based cleanup of expired entries
func TestRegistryCleanup(t *testing.T) {
	registry := NewIDRegistry()

	// Register an entry
	fullID := "test_id_for_cleanup"
	shortKey := registry.Register(fullID, "q", "0")

	// Verify it's there
	if _, found := registry.Lookup(shortKey); !found {
		t.Fatal("Newly registered key should be found")
	}

	// Manually set TTL to past time to simulate expiry
	registry.setTTL(shortKey, time.Now().Add(-1*time.Hour))

	// Run cleanup (should remove expired entries)
	registry.cleanup()

	// Entry should be gone
	if _, found := registry.Lookup(shortKey); found {
		t.Error("Expired entry should be removed by cleanup")
	}
}

// TestRegistryConcurrentAccess tests that registry is goroutine-safe
func TestRegistryConcurrentAccess(t *testing.T) {
	registry := NewIDRegistry()
	var wg sync.WaitGroup
	const numGoroutines = 50

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Concurrent registers
			fullID := "id_" + string(rune(id))
			shortKey := registry.Register(fullID, "q", "0")

			// Concurrent lookups
			if retrieved, found := registry.Lookup(shortKey); !found || retrieved != fullID {
				t.Errorf("Lookup failed for goroutine %d", id)
			}
		}(i)
	}

	wg.Wait()
}

// TestRegistryDeduplication tests that same fullID returns same shortKey
func TestRegistryDeduplication(t *testing.T) {
	registry := NewIDRegistry()

	fullID := "duplicate_test_id"
	first := registry.Register(fullID, "q", "0")
	second := registry.Register(fullID, "q", "0")

	if first != second {
		t.Errorf("Same fullID should return same shortKey: %q vs %q", first, second)
	}
}
