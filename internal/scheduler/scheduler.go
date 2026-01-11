package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spider-crawler/spider/internal/config"
	"github.com/spider-crawler/spider/internal/frontier"
	"github.com/spider-crawler/spider/internal/urlutil"
)

// WorkerFunc is the function signature for URL processing workers.
type WorkerFunc func(ctx context.Context, item *frontier.URLItem) (*CrawlResult, error)

// CrawlResult represents the result of crawling a URL.
type CrawlResult struct {
	Item          *frontier.URLItem
	StatusCode    int
	ContentType   string
	ContentLength int64
	ResponseTime  time.Duration
	FinalURL      string
	RedirectChain []string
	Error         error
	Retry         bool
	DiscoveredURLs []string
}

// SchedulerStats holds scheduler statistics.
type SchedulerStats struct {
	URLsProcessed  int64
	URLsSucceeded  int64
	URLsFailed     int64
	URLsRetried    int64
	URLsInQueue    int
	URLsVisited    int
	ActiveWorkers  int32
	TotalDuplicates int
	StartTime      time.Time
	ElapsedTime    time.Duration
}

// Scheduler orchestrates the crawling process.
type Scheduler struct {
	config      *config.CrawlConfig
	frontier    *frontier.MemoryFrontier
	normalizer  *urlutil.Normalizer
	rateLimiter *HostRateLimiter
	workerFunc  WorkerFunc

	// State
	running       atomic.Bool
	paused        atomic.Bool
	activeWorkers atomic.Int32

	// Statistics
	urlsProcessed atomic.Int64
	urlsSucceeded atomic.Int64
	urlsFailed    atomic.Int64
	urlsRetried   atomic.Int64
	startTime     time.Time

	// Synchronization
	wg       sync.WaitGroup
	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}

	// Results channel
	resultsCh chan *CrawlResult
}

// NewScheduler creates a new scheduler.
func NewScheduler(cfg *config.CrawlConfig) *Scheduler {
	return &Scheduler{
		config:     cfg,
		frontier:   frontier.NewMemoryFrontier(cfg.TraversalMode, cfg.MaxDepth, cfg.MaxURLs),
		normalizer: urlutil.DefaultNormalizer(cfg.IgnoreQueryParams),
		rateLimiter: NewHostRateLimiter(cfg.CrawlDelay, cfg.RequestsPerSecond),
		pauseCh:    make(chan struct{}),
		resumeCh:   make(chan struct{}),
		stopCh:     make(chan struct{}),
		resultsCh:  make(chan *CrawlResult, cfg.Concurrency*2),
	}
}

// SetWorkerFunc sets the worker function for processing URLs.
func (s *Scheduler) SetWorkerFunc(fn WorkerFunc) {
	s.workerFunc = fn
}

// AddSeed adds a seed URL to the frontier.
func (s *Scheduler) AddSeed(rawURL string) error {
	normalized, err := s.normalizer.Normalize(rawURL)
	if err != nil {
		return err
	}

	host, err := urlutil.ExtractHost(rawURL)
	if err != nil {
		return err
	}

	item := frontier.NewURLItem(rawURL, normalized, host, 0, "")
	s.frontier.Push(item)
	return nil
}

// AddURL adds a discovered URL to the frontier.
func (s *Scheduler) AddURL(rawURL, discoveredFrom string, depth int) error {
	normalized, err := s.normalizer.Normalize(rawURL)
	if err != nil {
		return err
	}

	host, err := urlutil.ExtractHost(rawURL)
	if err != nil {
		return err
	}

	item := frontier.NewURLItem(rawURL, normalized, host, depth, discoveredFrom)
	s.frontier.Push(item)
	return nil
}

// Start begins the crawling process.
func (s *Scheduler) Start(ctx context.Context) error {
	if s.workerFunc == nil {
		panic("worker function not set")
	}

	s.running.Store(true)
	s.startTime = time.Now()

	// Start worker goroutines
	for i := 0; i < s.config.Concurrency; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}

	return nil
}

// worker is the main worker goroutine.
func (s *Scheduler) worker(ctx context.Context, id int) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}

		// Check if paused
		if s.paused.Load() {
			select {
			case <-s.resumeCh:
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			}
		}

		// Get next URL from frontier
		item := s.frontier.Pop()
		if item == nil {
			// No more URLs, wait a bit and check again
			time.Sleep(100 * time.Millisecond)

			// Check if all workers are idle and frontier is empty
			if s.frontier.IsEmpty() && s.activeWorkers.Load() == 0 {
				return
			}
			continue
		}

		// Check if URL is ready to be crawled (for retries with backoff)
		if !item.CanCrawl() {
			s.frontier.Requeue(item)
			time.Sleep(50 * time.Millisecond)
			continue
		}

		// Wait for rate limiter
		s.rateLimiter.Wait(item.Host)

		// Process the URL
		s.activeWorkers.Add(1)
		result, err := s.workerFunc(ctx, item)
		s.activeWorkers.Add(-1)

		// Record host access
		s.rateLimiter.RecordAccess(item.Host)

		// Mark as visited
		s.frontier.MarkVisited(item.NormalizedURL)
		s.urlsProcessed.Add(1)

		// Handle result
		if err != nil || (result != nil && result.Error != nil) {
			s.urlsFailed.Add(1)

			// Check if we should retry
			if result != nil && result.Retry && item.RetryCount < s.config.MaxRetries {
				item.IncrementRetry(s.config.RetryBackoff)
				s.frontier.Requeue(item)
				s.urlsRetried.Add(1)
			}
		} else {
			s.urlsSucceeded.Add(1)

			// Add discovered URLs to frontier
			if result != nil {
				for _, discoveredURL := range result.DiscoveredURLs {
					s.AddURL(discoveredURL, item.URL, item.Depth+1)
				}
			}
		}

		// Send result to channel
		if result != nil {
			select {
			case s.resultsCh <- result:
			default:
				// Channel full, skip (or could block)
			}
		}
	}
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.running.Store(false)
	close(s.stopCh)
}

// Wait waits for all workers to complete.
func (s *Scheduler) Wait() {
	s.wg.Wait()
	close(s.resultsCh)
}

// Pause pauses the scheduler.
func (s *Scheduler) Pause() {
	s.paused.Store(true)
}

// Resume resumes the scheduler.
func (s *Scheduler) Resume() {
	s.paused.Store(false)
	// Notify all waiting workers
	for i := 0; i < s.config.Concurrency; i++ {
		select {
		case s.resumeCh <- struct{}{}:
		default:
		}
	}
}

// IsRunning returns true if the scheduler is running.
func (s *Scheduler) IsRunning() bool {
	return s.running.Load()
}

// IsPaused returns true if the scheduler is paused.
func (s *Scheduler) IsPaused() bool {
	return s.paused.Load()
}

// Results returns the results channel.
func (s *Scheduler) Results() <-chan *CrawlResult {
	return s.resultsCh
}

// Stats returns current scheduler statistics.
func (s *Scheduler) Stats() SchedulerStats {
	frontierStats := s.frontier.Stats()
	return SchedulerStats{
		URLsProcessed:   s.urlsProcessed.Load(),
		URLsSucceeded:   s.urlsSucceeded.Load(),
		URLsFailed:      s.urlsFailed.Load(),
		URLsRetried:     s.urlsRetried.Load(),
		URLsInQueue:     frontierStats.Queued,
		URLsVisited:     frontierStats.Visited,
		ActiveWorkers:   s.activeWorkers.Load(),
		TotalDuplicates: frontierStats.Duplicates,
		StartTime:       s.startTime,
		ElapsedTime:     time.Since(s.startTime),
	}
}

// Frontier returns the frontier for direct access.
func (s *Scheduler) Frontier() *frontier.MemoryFrontier {
	return s.frontier
}
