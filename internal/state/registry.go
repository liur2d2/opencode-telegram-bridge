package state

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type IDRegistry struct {
	mu       sync.RWMutex
	counter  int
	mappings map[string]string    // shortKey → fullID
	reverse  map[string]string    // fullID → shortKey (for deduplication)
	ttl      map[string]time.Time // shortKey → expiry time
}

func NewIDRegistry() *IDRegistry {
	return &IDRegistry{
		counter:  0,
		mappings: make(map[string]string),
		reverse:  make(map[string]string),
		ttl:      make(map[string]time.Time),
	}
}

// Register stores a full ID and returns a short key suitable for Telegram callback_data.
// If the same fullID is registered again, it returns the existing shortKey (deduplication).
// Format: prefix:counter:suffix (e.g., "q:1:0", "p:2:once")
func (r *IDRegistry) Register(fullID string, prefix string, suffix string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered (deduplication)
	if shortKey, exists := r.reverse[fullID]; exists {
		return shortKey
	}

	// Generate new short key with auto-incrementing counter
	r.counter++
	shortKey := fmt.Sprintf("%s:%d:%s", prefix, r.counter, suffix)

	// Store mappings
	r.mappings[shortKey] = fullID
	r.reverse[fullID] = shortKey

	// Set TTL to 1 hour from now
	r.ttl[shortKey] = time.Now().Add(1 * time.Hour)

	return shortKey
}

// Lookup retrieves the full ID from a short key.
// Returns the full ID and a boolean indicating if the key was found.
func (r *IDRegistry) Lookup(shortKey string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fullID, found := r.mappings[shortKey]
	return fullID, found
}

// cleanup removes entries that have expired (TTL passed).
// This is called internally and can be called explicitly for testing.
func (r *IDRegistry) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for shortKey, expiry := range r.ttl {
		if now.After(expiry) {
			// Remove expired entry
			if fullID, exists := r.mappings[shortKey]; exists {
				delete(r.reverse, fullID)
			}
			delete(r.mappings, shortKey)
			delete(r.ttl, shortKey)
		}
	}
}

// setTTL is an internal method for testing — allows setting custom TTL times.
func (r *IDRegistry) setTTL(shortKey string, expiry time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.ttl[shortKey]; exists {
		r.ttl[shortKey] = expiry
	}
}

// StartCleanup starts a background goroutine that removes expired entries every 5 minutes.
func (r *IDRegistry) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.cleanup()
			}
		}
	}()
}
