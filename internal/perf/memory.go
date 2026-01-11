// Package perf provides performance optimization utilities.
package perf

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryManager manages memory usage for large crawls.
type MemoryManager struct {
	mu sync.RWMutex

	// Configuration
	config *MemoryConfig

	// State
	currentAlloc   uint64
	peakAlloc      uint64
	gcCount        uint32
	lastGC         time.Time
	pressureLevel  PressureLevel
	pauseRequested int32

	// Callbacks
	onPressure func(PressureLevel)

	// Monitoring
	stopChan chan struct{}
}

// MemoryConfig defines memory management configuration.
type MemoryConfig struct {
	// Limits
	SoftLimit uint64 // Soft memory limit (trigger GC)
	HardLimit uint64 // Hard memory limit (pause crawling)

	// GC settings
	GCPercent       int           // GOGC percentage
	MinGCInterval   time.Duration // Minimum time between forced GCs
	MonitorInterval time.Duration // Memory monitoring interval

	// Thresholds
	LowPressureThreshold    float64 // Fraction of soft limit for low pressure
	MediumPressureThreshold float64 // Fraction of soft limit for medium pressure
	HighPressureThreshold   float64 // Fraction of hard limit for high pressure
}

// PressureLevel indicates memory pressure.
type PressureLevel int

const (
	PressureNone   PressureLevel = iota
	PressureLow                  // Under soft limit, no action needed
	PressureMedium               // Approaching soft limit, reduce allocations
	PressureHigh                 // Over soft limit, trigger GC
	PressureCritical             // Near hard limit, pause crawling
)

// DefaultMemoryConfig returns default configuration.
func DefaultMemoryConfig() *MemoryConfig {
	// Get system memory
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	totalMem := m.Sys
	if totalMem == 0 {
		totalMem = 1 << 30 // Default 1GB if unknown
	}

	return &MemoryConfig{
		SoftLimit:               totalMem / 2,        // 50% of system memory
		HardLimit:               totalMem * 3 / 4,    // 75% of system memory
		GCPercent:               100,
		MinGCInterval:           5 * time.Second,
		MonitorInterval:         time.Second,
		LowPressureThreshold:    0.5,
		MediumPressureThreshold: 0.75,
		HighPressureThreshold:   0.9,
	}
}

// NewMemoryManager creates a new memory manager.
func NewMemoryManager(config *MemoryConfig) *MemoryManager {
	if config == nil {
		config = DefaultMemoryConfig()
	}

	// Set GOGC
	if config.GCPercent > 0 {
		debug.SetGCPercent(config.GCPercent)
	}

	return &MemoryManager{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Start starts memory monitoring.
func (m *MemoryManager) Start(ctx context.Context) {
	go m.monitorLoop(ctx)
}

// Stop stops memory monitoring.
func (m *MemoryManager) Stop() {
	close(m.stopChan)
}

// monitorLoop continuously monitors memory usage.
func (m *MemoryManager) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkMemory()
		}
	}
}

// checkMemory checks current memory usage.
func (m *MemoryManager) checkMemory() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentAlloc = stats.Alloc
	if stats.Alloc > m.peakAlloc {
		m.peakAlloc = stats.Alloc
	}
	m.gcCount = stats.NumGC

	// Determine pressure level
	oldLevel := m.pressureLevel
	m.pressureLevel = m.calculatePressure(stats.Alloc)

	// Take action based on pressure
	if m.pressureLevel != oldLevel {
		if m.onPressure != nil {
			m.onPressure(m.pressureLevel)
		}
	}

	switch m.pressureLevel {
	case PressureHigh:
		// Trigger GC if enough time has passed
		if time.Since(m.lastGC) > m.config.MinGCInterval {
			runtime.GC()
			m.lastGC = time.Now()
		}
	case PressureCritical:
		// Request pause
		atomic.StoreInt32(&m.pauseRequested, 1)
		// Force GC
		runtime.GC()
		debug.FreeOSMemory()
		m.lastGC = time.Now()
	default:
		atomic.StoreInt32(&m.pauseRequested, 0)
	}
}

// calculatePressure calculates pressure level from allocation.
func (m *MemoryManager) calculatePressure(alloc uint64) PressureLevel {
	softRatio := float64(alloc) / float64(m.config.SoftLimit)
	hardRatio := float64(alloc) / float64(m.config.HardLimit)

	if hardRatio >= m.config.HighPressureThreshold {
		return PressureCritical
	}
	if softRatio >= 1.0 {
		return PressureHigh
	}
	if softRatio >= m.config.MediumPressureThreshold {
		return PressureMedium
	}
	if softRatio >= m.config.LowPressureThreshold {
		return PressureLow
	}
	return PressureNone
}

// SetPressureCallback sets the pressure change callback.
func (m *MemoryManager) SetPressureCallback(cb func(PressureLevel)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPressure = cb
}

// GetPressureLevel returns the current pressure level.
func (m *MemoryManager) GetPressureLevel() PressureLevel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pressureLevel
}

// ShouldPause returns true if crawling should pause.
func (m *MemoryManager) ShouldPause() bool {
	return atomic.LoadInt32(&m.pauseRequested) == 1
}

// WaitForResume blocks until memory pressure is relieved.
func (m *MemoryManager) WaitForResume(ctx context.Context) error {
	for m.ShouldPause() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
	return nil
}

// ForceGC forces garbage collection.
func (m *MemoryManager) ForceGC() {
	runtime.GC()
	debug.FreeOSMemory()
	m.mu.Lock()
	m.lastGC = time.Now()
	m.mu.Unlock()
}

// MemoryStats returns current memory statistics.
type MemoryStats struct {
	Alloc         uint64        // Current allocation
	TotalAlloc    uint64        // Total allocation over lifetime
	Sys           uint64        // System memory obtained
	HeapAlloc     uint64        // Heap allocation
	HeapInuse     uint64        // Heap in use
	HeapIdle      uint64        // Heap idle
	HeapReleased  uint64        // Heap released to OS
	StackInuse    uint64        // Stack in use
	GCCount       uint32        // Number of GC cycles
	LastGC        time.Time     // Last GC time
	PeakAlloc     uint64        // Peak allocation
	SoftLimit     uint64        // Soft limit
	HardLimit     uint64        // Hard limit
	PressureLevel PressureLevel // Current pressure level
}

// GetStats returns current memory statistics.
func (m *MemoryManager) GetStats() *MemoryStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	m.mu.RLock()
	defer m.mu.RUnlock()

	lastGC := time.Time{}
	if stats.LastGC > 0 {
		lastGC = time.Unix(0, int64(stats.LastGC))
	}

	return &MemoryStats{
		Alloc:         stats.Alloc,
		TotalAlloc:    stats.TotalAlloc,
		Sys:           stats.Sys,
		HeapAlloc:     stats.HeapAlloc,
		HeapInuse:     stats.HeapInuse,
		HeapIdle:      stats.HeapIdle,
		HeapReleased:  stats.HeapReleased,
		StackInuse:    stats.StackInuse,
		GCCount:       stats.NumGC,
		LastGC:        lastGC,
		PeakAlloc:     m.peakAlloc,
		SoftLimit:     m.config.SoftLimit,
		HardLimit:     m.config.HardLimit,
		PressureLevel: m.pressureLevel,
	}
}

// SetLimits updates memory limits.
func (m *MemoryManager) SetLimits(soft, hard uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.SoftLimit = soft
	m.config.HardLimit = hard
}

// ObjectPool provides a generic object pool.
type ObjectPool[T any] struct {
	pool sync.Pool
	new  func() T
}

// NewObjectPool creates a new object pool.
func NewObjectPool[T any](newFunc func() T) *ObjectPool[T] {
	return &ObjectPool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFunc()
			},
		},
		new: newFunc,
	}
}

// Get retrieves an object from the pool.
func (p *ObjectPool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put returns an object to the pool.
func (p *ObjectPool[T]) Put(obj T) {
	p.pool.Put(obj)
}

// BufferPool provides a pool for byte slices.
type BufferPool struct {
	pool *sync.Pool
	size int
}

// NewBufferPool creates a new buffer pool.
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: &sync.Pool{
			New: func() any {
				return make([]byte, size)
			},
		},
		size: size,
	}
}

// Get retrieves a buffer from the pool.
func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a buffer to the pool.
func (p *BufferPool) Put(buf []byte) {
	// Only return buffers of the expected size
	if cap(buf) == p.size {
		p.pool.Put(buf[:p.size])
	}
}

// MemoryLimiter limits concurrent operations based on memory.
type MemoryLimiter struct {
	manager     *MemoryManager
	maxWorkers  int
	minWorkers  int
	current     int32
	adjustMu    sync.Mutex
}

// NewMemoryLimiter creates a new memory limiter.
func NewMemoryLimiter(manager *MemoryManager, min, max int) *MemoryLimiter {
	return &MemoryLimiter{
		manager:    manager,
		minWorkers: min,
		maxWorkers: max,
		current:    int32(max),
	}
}

// GetWorkerCount returns the current recommended worker count.
func (l *MemoryLimiter) GetWorkerCount() int {
	l.adjustMu.Lock()
	defer l.adjustMu.Unlock()

	pressure := l.manager.GetPressureLevel()

	var target int
	switch pressure {
	case PressureNone:
		target = l.maxWorkers
	case PressureLow:
		target = l.maxWorkers * 3 / 4
	case PressureMedium:
		target = l.maxWorkers / 2
	case PressureHigh:
		target = l.maxWorkers / 4
	case PressureCritical:
		target = l.minWorkers
	}

	if target < l.minWorkers {
		target = l.minWorkers
	}

	atomic.StoreInt32(&l.current, int32(target))
	return target
}

// Acquire checks if a worker can proceed.
func (l *MemoryLimiter) Acquire(ctx context.Context) error {
	// Wait if at critical pressure
	if err := l.manager.WaitForResume(ctx); err != nil {
		return err
	}
	return nil
}
