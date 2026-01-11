// Package testing provides test utilities for the spider crawler.
package testing

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

// Benchmark provides benchmarking utilities.
type Benchmark struct {
	name      string
	runs      []time.Duration
	memAllocs []uint64
	memBytes  []uint64
	startTime time.Time
	startMem  runtime.MemStats
}

// NewBenchmark creates a new benchmark.
func NewBenchmark(name string) *Benchmark {
	return &Benchmark{
		name:      name,
		runs:      make([]time.Duration, 0),
		memAllocs: make([]uint64, 0),
		memBytes:  make([]uint64, 0),
	}
}

// Start starts a benchmark run.
func (b *Benchmark) Start() {
	runtime.GC() // Clean up before measuring
	runtime.ReadMemStats(&b.startMem)
	b.startTime = time.Now()
}

// Stop stops a benchmark run.
func (b *Benchmark) Stop() {
	elapsed := time.Since(b.startTime)
	var endMem runtime.MemStats
	runtime.ReadMemStats(&endMem)

	b.runs = append(b.runs, elapsed)
	b.memAllocs = append(b.memAllocs, endMem.Mallocs-b.startMem.Mallocs)
	b.memBytes = append(b.memBytes, endMem.TotalAlloc-b.startMem.TotalAlloc)
}

// Run runs a function and measures it.
func (b *Benchmark) Run(f func()) {
	b.Start()
	f()
	b.Stop()
}

// RunN runs a function N times.
func (b *Benchmark) RunN(n int, f func()) {
	for i := 0; i < n; i++ {
		b.Run(f)
	}
}

// RunParallel runs a function in parallel.
func (b *Benchmark) RunParallel(goroutines int, f func()) {
	var wg sync.WaitGroup
	wg.Add(goroutines)

	b.Start()
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			f()
		}()
	}
	wg.Wait()
	b.Stop()
}

// BenchmarkResult contains benchmark results.
type BenchmarkResult struct {
	Name         string
	Runs         int
	TotalTime    time.Duration
	MinTime      time.Duration
	MaxTime      time.Duration
	AvgTime      time.Duration
	MedianTime   time.Duration
	P95Time      time.Duration
	P99Time      time.Duration
	StdDev       time.Duration
	OpsPerSecond float64
	MemAllocs    uint64
	MemBytes     uint64
}

// Result calculates and returns benchmark results.
func (b *Benchmark) Result() *BenchmarkResult {
	if len(b.runs) == 0 {
		return &BenchmarkResult{Name: b.name}
	}

	// Sort runs for percentile calculations
	sorted := make([]time.Duration, len(b.runs))
	copy(sorted, b.runs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate statistics
	var total time.Duration
	for _, d := range b.runs {
		total += d
	}
	avg := total / time.Duration(len(b.runs))

	// Calculate standard deviation
	var sumSquares float64
	for _, d := range b.runs {
		diff := float64(d - avg)
		sumSquares += diff * diff
	}
	stdDev := time.Duration(0)
	if len(b.runs) > 1 {
		variance := sumSquares / float64(len(b.runs)-1)
		stdDev = time.Duration(variance)
	}

	// Calculate memory stats
	var totalAllocs, totalBytes uint64
	for i := range b.memAllocs {
		totalAllocs += b.memAllocs[i]
		totalBytes += b.memBytes[i]
	}

	return &BenchmarkResult{
		Name:         b.name,
		Runs:         len(b.runs),
		TotalTime:    total,
		MinTime:      sorted[0],
		MaxTime:      sorted[len(sorted)-1],
		AvgTime:      avg,
		MedianTime:   sorted[len(sorted)/2],
		P95Time:      sorted[int(float64(len(sorted))*0.95)],
		P99Time:      sorted[int(float64(len(sorted))*0.99)],
		StdDev:       stdDev,
		OpsPerSecond: float64(len(b.runs)) / total.Seconds(),
		MemAllocs:    totalAllocs / uint64(len(b.runs)),
		MemBytes:     totalBytes / uint64(len(b.runs)),
	}
}

// String returns a formatted result string.
func (r *BenchmarkResult) String() string {
	return fmt.Sprintf(`Benchmark: %s
  Runs:        %d
  Total Time:  %v
  Min Time:    %v
  Max Time:    %v
  Avg Time:    %v
  Median Time: %v
  P95 Time:    %v
  P99 Time:    %v
  Std Dev:     %v
  Ops/Second:  %.2f
  Avg Allocs:  %d
  Avg Bytes:   %d`,
		r.Name, r.Runs, r.TotalTime,
		r.MinTime, r.MaxTime, r.AvgTime, r.MedianTime,
		r.P95Time, r.P99Time, r.StdDev, r.OpsPerSecond,
		r.MemAllocs, r.MemBytes)
}

// BenchmarkSuite runs multiple benchmarks.
type BenchmarkSuite struct {
	name       string
	benchmarks []*Benchmark
	results    []*BenchmarkResult
}

// NewBenchmarkSuite creates a new benchmark suite.
func NewBenchmarkSuite(name string) *BenchmarkSuite {
	return &BenchmarkSuite{
		name:       name,
		benchmarks: make([]*Benchmark, 0),
		results:    make([]*BenchmarkResult, 0),
	}
}

// Add adds a benchmark to the suite.
func (s *BenchmarkSuite) Add(name string, n int, f func()) {
	b := NewBenchmark(name)
	b.RunN(n, f)
	s.benchmarks = append(s.benchmarks, b)
	s.results = append(s.results, b.Result())
}

// Results returns all benchmark results.
func (s *BenchmarkSuite) Results() []*BenchmarkResult {
	return s.results
}

// Report generates a comparison report.
func (s *BenchmarkSuite) Report() string {
	var sb fmt.Stringer = &reportBuilder{}
	rb := sb.(*reportBuilder)

	rb.writeLine("=== Benchmark Suite: %s ===\n", s.name)

	for _, r := range s.results {
		rb.writeLine("\n%s\n", r.String())
	}

	// Comparison table
	if len(s.results) > 1 {
		rb.writeLine("\n=== Comparison ===\n")
		rb.writeLine("%-30s %15s %15s %15s\n", "Name", "Avg Time", "Ops/Sec", "Allocs")
		rb.writeLine("%s\n", "--------------------------------------------------------------------------------")

		for _, r := range s.results {
			rb.writeLine("%-30s %15v %15.2f %15d\n",
				r.Name, r.AvgTime, r.OpsPerSecond, r.MemAllocs)
		}
	}

	return rb.String()
}

type reportBuilder struct {
	lines []string
}

func (rb *reportBuilder) writeLine(format string, args ...interface{}) {
	rb.lines = append(rb.lines, fmt.Sprintf(format, args...))
}

func (rb *reportBuilder) String() string {
	result := ""
	for _, line := range rb.lines {
		result += line
	}
	return result
}

// Timer provides simple timing utilities.
type Timer struct {
	start time.Time
	laps  []time.Duration
}

// NewTimer creates a new timer.
func NewTimer() *Timer {
	return &Timer{
		start: time.Now(),
		laps:  make([]time.Duration, 0),
	}
}

// Lap records a lap time.
func (t *Timer) Lap() time.Duration {
	elapsed := time.Since(t.start)
	t.laps = append(t.laps, elapsed)
	return elapsed
}

// Reset resets the timer.
func (t *Timer) Reset() {
	t.start = time.Now()
	t.laps = t.laps[:0]
}

// Elapsed returns total elapsed time.
func (t *Timer) Elapsed() time.Duration {
	return time.Since(t.start)
}

// Laps returns all recorded laps.
func (t *Timer) Laps() []time.Duration {
	return t.laps
}

// ProgressTracker tracks operation progress.
type ProgressTracker struct {
	mu sync.RWMutex

	total     int
	completed int
	startTime time.Time
	lastTick  time.Time
	rate      float64
}

// NewProgressTracker creates a progress tracker.
func NewProgressTracker(total int) *ProgressTracker {
	now := time.Now()
	return &ProgressTracker{
		total:     total,
		startTime: now,
		lastTick:  now,
	}
}

// Tick increments progress.
func (p *ProgressTracker) Tick() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.completed++
	now := time.Now()
	elapsed := now.Sub(p.startTime).Seconds()
	if elapsed > 0 {
		p.rate = float64(p.completed) / elapsed
	}
	p.lastTick = now
}

// TickN increments progress by n.
func (p *ProgressTracker) TickN(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.completed += n
	now := time.Now()
	elapsed := now.Sub(p.startTime).Seconds()
	if elapsed > 0 {
		p.rate = float64(p.completed) / elapsed
	}
	p.lastTick = now
}

// Progress returns current progress (0-100).
func (p *ProgressTracker) Progress() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.total == 0 {
		return 0
	}
	return float64(p.completed) / float64(p.total) * 100
}

// Rate returns operations per second.
func (p *ProgressTracker) Rate() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.rate
}

// ETA returns estimated time remaining.
func (p *ProgressTracker) ETA() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.rate == 0 || p.completed >= p.total {
		return 0
	}

	remaining := p.total - p.completed
	return time.Duration(float64(remaining)/p.rate) * time.Second
}

// Stats returns progress statistics.
type ProgressStats struct {
	Total     int
	Completed int
	Progress  float64
	Rate      float64
	ETA       time.Duration
	Elapsed   time.Duration
}

// Stats returns current progress statistics.
func (p *ProgressTracker) Stats() ProgressStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProgressStats{
		Total:     p.total,
		Completed: p.completed,
		Progress:  float64(p.completed) / float64(p.total) * 100,
		Rate:      p.rate,
		ETA:       p.ETA(),
		Elapsed:   time.Since(p.startTime),
	}
}
