package subtitles

import (
	"sync"
	"time"
)

// RateLimiter implements a simple token bucket rate limiter
// Memory-efficient: no external libraries, minimal allocations
type RateLimiter struct {
	tokens    int
	maxTokens int
	refillRate time.Duration
	mu        sync.Mutex
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter
// maxRequests: maximum requests allowed in the time window
// window: time window (e.g., 1 minute)
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:     maxRequests,
		maxTokens:  maxRequests,
		refillRate: window / time.Duration(maxRequests),
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}

	// Wait until a token is available
	for rl.tokens == 0 {
		rl.mu.Unlock()
		time.Sleep(rl.refillRate)
		rl.mu.Lock()

		// Refill after sleep
		now = time.Now()
		elapsed = now.Sub(rl.lastRefill)
		tokensToAdd = int(elapsed / rl.refillRate)
		if tokensToAdd > 0 {
			rl.tokens += tokensToAdd
			if rl.tokens > rl.maxTokens {
				rl.tokens = rl.maxTokens
			}
			rl.lastRefill = now
		}
	}

	// Consume a token
	rl.tokens--
}

// TryAcquire attempts to acquire a token without blocking
// Returns true if successful, false if no tokens available
func (rl *RateLimiter) TryAcquire() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}
