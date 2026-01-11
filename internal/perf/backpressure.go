// Package perf provides performance optimization utilities.
package perf

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// BackpressureController manages crawl rate based on system load.
type BackpressureController struct {
	mu sync.RWMutex

	// Configuration
	config *BackpressureConfig

	// State
	currentRate     float64
	pendingRequests int64
	activeWorkers   int64
	errorCount      int64
	successCount    int64
	lastAdjustment  time.Time

	// Signals
	slowDown chan struct{}
	speedUp  chan struct{}

	// Metrics
	metrics *BackpressureMetrics
}

// BackpressureConfig defines backpressure configuration.
type BackpressureConfig struct {
	// Rate limits
	MinRate float64 // Minimum requests per second
	MaxRate float64 // Maximum requests per second

	// Thresholds
	PendingThreshold   int64   // Max pending requests before slowing
	ErrorRateThreshold float64 // Error rate threshold (0-1)
	ResponseTimeThreshold time.Duration // Response time threshold

	// Adjustment
	AdjustInterval   time.Duration // How often to adjust rate
	IncreaseFactor   float64       // Rate increase multiplier
	DecreaseFactor   float64       // Rate decrease multiplier
	CooldownDuration time.Duration // Cooldown after adjustment

	// Memory
	MemoryThreshold float64 // Memory usage threshold (0-1)
}

// DefaultBackpressureConfig returns default configuration.
func DefaultBackpressureConfig() *BackpressureConfig {
	return &BackpressureConfig{
		MinRate:              0.5,
		MaxRate:              50.0,
		PendingThreshold:     1000,
		ErrorRateThreshold:   0.1,
		ResponseTimeThreshold: 5 * time.Second,
		AdjustInterval:       time.Second,
		IncreaseFactor:       1.1,
		DecreaseFactor:       0.7,
		CooldownDuration:     5 * time.Second,
		MemoryThreshold:      0.8,
	}
}

// BackpressureMetrics tracks backpressure metrics.
type BackpressureMetrics struct {
	mu sync.RWMutex

	CurrentRate      float64
	PendingRequests  int64
	ActiveWorkers    int64
	TotalRequests    int64
	TotalErrors      int64
	AvgResponseTime  time.Duration
	SlowdownEvents   int64
	SpeedupEvents    int64
	LastAdjustment   time.Time
	MemoryUsage      float64
}

// NewBackpressureController creates a new backpressure controller.
func NewBackpressureController(config *BackpressureConfig) *BackpressureController {
	if config == nil {
		config = DefaultBackpressureConfig()
	}

	return &BackpressureController{
		config:      config,
		currentRate: config.MaxRate / 2, // Start at half max
		slowDown:    make(chan struct{}, 100),
		speedUp:     make(chan struct{}, 100),
		metrics:     &BackpressureMetrics{},
	}
}

// Start starts the backpressure controller.
func (b *BackpressureController) Start(ctx context.Context) {
	go b.adjustmentLoop(ctx)
}

// adjustmentLoop periodically adjusts the rate.
func (b *BackpressureController) adjustmentLoop(ctx context.Context) {
	ticker := time.NewTicker(b.config.AdjustInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.adjustRate()
		case <-b.slowDown:
			b.decreaseRate()
		case <-b.speedUp:
			b.increaseRate()
		}
	}
}

// adjustRate automatically adjusts rate based on conditions.
func (b *BackpressureController) adjustRate() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check cooldown
	if time.Since(b.lastAdjustment) < b.config.CooldownDuration {
		return
	}

	// Calculate error rate
	total := atomic.LoadInt64(&b.successCount) + atomic.LoadInt64(&b.errorCount)
	errorRate := 0.0
	if total > 0 {
		errorRate = float64(atomic.LoadInt64(&b.errorCount)) / float64(total)
	}

	pending := atomic.LoadInt64(&b.pendingRequests)

	// Decide action
	shouldDecrease := false
	shouldIncrease := false

	if errorRate > b.config.ErrorRateThreshold {
		shouldDecrease = true
	} else if pending > b.config.PendingThreshold {
		shouldDecrease = true
	} else if errorRate < b.config.ErrorRateThreshold/2 && pending < b.config.PendingThreshold/2 {
		shouldIncrease = true
	}

	if shouldDecrease {
		b.currentRate *= b.config.DecreaseFactor
		if b.currentRate < b.config.MinRate {
			b.currentRate = b.config.MinRate
		}
		b.metrics.SlowdownEvents++
	} else if shouldIncrease {
		b.currentRate *= b.config.IncreaseFactor
		if b.currentRate > b.config.MaxRate {
			b.currentRate = b.config.MaxRate
		}
		b.metrics.SpeedupEvents++
	}

	b.lastAdjustment = time.Now()
	b.updateMetrics()
}

// decreaseRate forcefully decreases the rate.
func (b *BackpressureController) decreaseRate() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.currentRate *= b.config.DecreaseFactor
	if b.currentRate < b.config.MinRate {
		b.currentRate = b.config.MinRate
	}
	b.lastAdjustment = time.Now()
	b.metrics.SlowdownEvents++
	b.updateMetrics()
}

// increaseRate forcefully increases the rate.
func (b *BackpressureController) increaseRate() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.currentRate *= b.config.IncreaseFactor
	if b.currentRate > b.config.MaxRate {
		b.currentRate = b.config.MaxRate
	}
	b.lastAdjustment = time.Now()
	b.metrics.SpeedupEvents++
	b.updateMetrics()
}

// updateMetrics updates internal metrics.
func (b *BackpressureController) updateMetrics() {
	b.metrics.mu.Lock()
	defer b.metrics.mu.Unlock()

	b.metrics.CurrentRate = b.currentRate
	b.metrics.PendingRequests = atomic.LoadInt64(&b.pendingRequests)
	b.metrics.ActiveWorkers = atomic.LoadInt64(&b.activeWorkers)
	b.metrics.TotalRequests = atomic.LoadInt64(&b.successCount) + atomic.LoadInt64(&b.errorCount)
	b.metrics.TotalErrors = atomic.LoadInt64(&b.errorCount)
	b.metrics.LastAdjustment = b.lastAdjustment
}

// GetRate returns the current rate limit.
func (b *BackpressureController) GetRate() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.currentRate
}

// GetDelay returns the delay between requests.
func (b *BackpressureController) GetDelay() time.Duration {
	rate := b.GetRate()
	if rate <= 0 {
		return time.Second
	}
	return time.Duration(float64(time.Second) / rate)
}

// RequestStarted signals a request has started.
func (b *BackpressureController) RequestStarted() {
	atomic.AddInt64(&b.pendingRequests, 1)
	atomic.AddInt64(&b.activeWorkers, 1)
}

// RequestCompleted signals a request has completed.
func (b *BackpressureController) RequestCompleted(success bool, duration time.Duration) {
	atomic.AddInt64(&b.pendingRequests, -1)
	atomic.AddInt64(&b.activeWorkers, -1)

	if success {
		atomic.AddInt64(&b.successCount, 1)
	} else {
		atomic.AddInt64(&b.errorCount, 1)
	}

	// Check if response time is too high
	if duration > b.config.ResponseTimeThreshold {
		select {
		case b.slowDown <- struct{}{}:
		default:
		}
	}
}

// SignalSlowDown requests a rate decrease.
func (b *BackpressureController) SignalSlowDown() {
	select {
	case b.slowDown <- struct{}{}:
	default:
	}
}

// SignalSpeedUp requests a rate increase.
func (b *BackpressureController) SignalSpeedUp() {
	select {
	case b.speedUp <- struct{}{}:
	default:
	}
}

// GetMetrics returns current metrics.
func (b *BackpressureController) GetMetrics() *BackpressureMetrics {
	b.metrics.mu.RLock()
	defer b.metrics.mu.RUnlock()

	return &BackpressureMetrics{
		CurrentRate:     b.metrics.CurrentRate,
		PendingRequests: b.metrics.PendingRequests,
		ActiveWorkers:   b.metrics.ActiveWorkers,
		TotalRequests:   b.metrics.TotalRequests,
		TotalErrors:     b.metrics.TotalErrors,
		SlowdownEvents:  b.metrics.SlowdownEvents,
		SpeedupEvents:   b.metrics.SpeedupEvents,
		LastAdjustment:  b.metrics.LastAdjustment,
	}
}

// Reset resets counters.
func (b *BackpressureController) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	atomic.StoreInt64(&b.pendingRequests, 0)
	atomic.StoreInt64(&b.activeWorkers, 0)
	atomic.StoreInt64(&b.errorCount, 0)
	atomic.StoreInt64(&b.successCount, 0)
	b.currentRate = b.config.MaxRate / 2
}

// Acquire blocks until a request slot is available.
func (b *BackpressureController) Acquire(ctx context.Context) error {
	delay := b.GetDelay()
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		b.RequestStarted()
		return nil
	}
}

// RateLimiter provides token bucket rate limiting.
type RateLimiter struct {
	mu sync.Mutex

	rate       float64
	burst      int
	tokens     float64
	lastUpdate time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

// Allow checks if a request is allowed.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastUpdate)
	r.tokens += r.rate * elapsed.Seconds()
	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}
	r.lastUpdate = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		if r.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(1000/r.rate) * time.Millisecond):
			continue
		}
	}
}

// SetRate updates the rate limit.
func (r *RateLimiter) SetRate(rate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rate = rate
}
