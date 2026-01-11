// Package scheduler handles crawl scheduling, rate limiting, and politeness.
package scheduler

import (
	"sync"
	"time"
)

// HostRateLimiter manages rate limiting per host.
type HostRateLimiter struct {
	mu            sync.RWMutex
	lastAccess    map[string]time.Time
	crawlDelay    time.Duration
	globalLimiter *TokenBucket
}

// NewHostRateLimiter creates a new per-host rate limiter.
func NewHostRateLimiter(crawlDelay time.Duration, globalRPS float64) *HostRateLimiter {
	return &HostRateLimiter{
		lastAccess:    make(map[string]time.Time),
		crawlDelay:    crawlDelay,
		globalLimiter: NewTokenBucket(globalRPS, int(globalRPS)+1),
	}
}

// Wait waits until it's safe to make a request to the given host.
func (r *HostRateLimiter) Wait(host string) {
	// First, wait for global rate limit
	r.globalLimiter.Wait()

	// Then, respect per-host crawl delay
	r.mu.Lock()
	lastTime, exists := r.lastAccess[host]
	r.mu.Unlock()

	if exists {
		elapsed := time.Since(lastTime)
		if elapsed < r.crawlDelay {
			time.Sleep(r.crawlDelay - elapsed)
		}
	}
}

// RecordAccess records that a request was made to the given host.
func (r *HostRateLimiter) RecordAccess(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastAccess[host] = time.Now()
}

// CanAccess checks if a request can be made to the host right now.
func (r *HostRateLimiter) CanAccess(host string) bool {
	r.mu.RLock()
	lastTime, exists := r.lastAccess[host]
	r.mu.RUnlock()

	if !exists {
		return true
	}

	return time.Since(lastTime) >= r.crawlDelay
}

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucket creates a new token bucket.
func NewTokenBucket(rps float64, burst int) *TokenBucket {
	if rps <= 0 {
		rps = 1000 // effectively unlimited
	}
	if burst < 1 {
		burst = 1
	}
	return &TokenBucket{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: rps,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available.
func (tb *TokenBucket) Wait() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	for tb.tokens < 1 {
		// Calculate wait time
		waitTime := time.Duration(float64(time.Second) / tb.refillRate)
		tb.mu.Unlock()
		time.Sleep(waitTime)
		tb.mu.Lock()
		tb.refill()
	}

	tb.tokens--
}

// TryAcquire attempts to acquire a token without blocking.
func (tb *TokenBucket) TryAcquire() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// refill adds tokens based on elapsed time.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate

	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}

	tb.lastRefill = now
}
