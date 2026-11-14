package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// clientIP extracts the client IP address from a request, checking
// X-Forwarded-For and X-Real-IP headers before falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimitEntry tracks attempts and expiry for a key.
type rateLimitEntry struct {
	count       int
	firstFailed time.Time
	lastFailed  time.Time
}

// LoginRateLimiter implements an in-memory IP-based rate limiter for login attempts.
type LoginRateLimiter struct {
	mu          sync.Mutex
	entries     map[string]*rateLimitEntry
	maxAttempts int
	window      time.Duration
	stopCh      chan struct{}
}

// NewLoginRateLimiter creates a new LoginRateLimiter with the specified max attempts and time window.
// It starts a background goroutine to periodically clean up expired rate limit entries.
func NewLoginRateLimiter(maxAttempts int, window time.Duration) *LoginRateLimiter {
	rl := &LoginRateLimiter{
		entries:     make(map[string]*rateLimitEntry),
		maxAttempts: maxAttempts,
		window:      window,
		stopCh:      make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if the given key (IP) is currently allowed to attempt login.
// If not allowed, it returns false and the retry-after duration in seconds.
func (rl *LoginRateLimiter) Allow(key string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.entries[key]
	if !exists {
		return true, 0
	}

	now := time.Now()
	// If the window has elapsed since the last failed attempt, reset entry
	if now.Sub(entry.lastFailed) >= rl.window {
		delete(rl.entries, key)
		return true, 0
	}

	if entry.count >= rl.maxAttempts {
		remaining := rl.window - now.Sub(entry.lastFailed)
		secs := int(remaining.Seconds())
		if secs < 1 {
			secs = 1
		}
		return false, secs
	}

	return true, 0
}

// RecordFailure records a failed login attempt for the key.
// Returns the new failed attempt count.
func (rl *LoginRateLimiter) RecordFailure(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[key]
	if !exists || now.Sub(entry.lastFailed) >= rl.window {
		rl.entries[key] = &rateLimitEntry{
			count:       1,
			firstFailed: now,
			lastFailed:  now,
		}
		return 1
	}

	entry.count++
	entry.lastFailed = now
	return entry.count
}

// Reset clears the failure record for the key on successful login.
func (rl *LoginRateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.entries, key)
}

// Stop terminates the background cleanup goroutine.
func (rl *LoginRateLimiter) Stop() {
	select {
	case <-rl.stopCh:
		// already stopped
	default:
		close(rl.stopCh)
	}
}

// cleanupLoop periodically sweeps stale entries.
func (rl *LoginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case now := <-ticker.C:
			rl.cleanup(now)
		}
	}
}

func (rl *LoginRateLimiter) cleanup(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for k, entry := range rl.entries {
		if now.Sub(entry.lastFailed) >= rl.window {
			delete(rl.entries, k)
		}
	}
}
