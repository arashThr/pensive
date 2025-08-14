package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter tracks attempts per IP address
type RateLimiter struct {
	attempts map[string][]time.Time // Track all attempts with timestamps
	mutex    sync.RWMutex
	limit    int
	window   time.Duration
	stopCh   chan struct{} // Channel to stop cleanup goroutine
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		stopCh:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get attempts for this IP
	attempts := rl.attempts[ip]

	// Remove expired attempts and add current attempt
	validAttempts := make([]time.Time, 0, len(attempts)+1)
	for _, attemptTime := range attempts {
		if attemptTime.After(cutoff) {
			validAttempts = append(validAttempts, attemptTime)
		}
	}
	validAttempts = append(validAttempts, now) // Add current attempt
	rl.attempts[ip] = validAttempts

	// Check if we're under the limit (excluding the current attempt we just added)
	return len(validAttempts)-1 < rl.limit
}

// GetAttemptCount returns the number of attempts within the current window for an IP
func (rl *RateLimiter) GetAttemptCount(ip string) int {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	attempts := rl.attempts[ip]

	// Count valid attempts within the window
	count := 0
	for _, attemptTime := range attempts {
		if attemptTime.After(cutoff) {
			count++
		}
	}

	return count
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// It's just demo. Simply clean it up to avoid indefinit growth
			rl.attempts = make(map[string][]time.Time)
		case <-rl.stopCh:
			return
		}
	}
}

// Stop gracefully stops the rate limiter and its cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// GetClientIP extracts the real client IP from the request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (from proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list (original client)
		if ip := parseFirstIP(xff); ip != "" {
			return ip
		}
	}

	// Check X-Real-IP header (from some proxies)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// parseFirstIP extracts the first valid IP from a comma-separated list
func parseFirstIP(ips string) string {
	// Split by comma and take first
	for i := 0; i < len(ips); i++ {
		if ips[i] == ',' {
			ips = ips[:i]
			break
		}
	}

	// Trim whitespace and validate
	if parsedIP := net.ParseIP(strings.TrimSpace(ips)); parsedIP != nil {
		return parsedIP.String()
	}

	return ""
}
