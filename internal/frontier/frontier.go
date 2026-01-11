package frontier

import (
	"container/list"
	"sync"

	"github.com/spider-crawler/spider/internal/config"
)

// Frontier is the interface for URL queue implementations.
type Frontier interface {
	// Push adds a URL to the frontier
	Push(item *URLItem) bool

	// Pop removes and returns the next URL to crawl
	Pop() *URLItem

	// Peek returns the next URL without removing it
	Peek() *URLItem

	// Size returns the number of URLs in the frontier
	Size() int

	// IsEmpty returns true if the frontier is empty
	IsEmpty() bool

	// Contains checks if a normalized URL is already in the frontier or visited
	Contains(normalizedURL string) bool

	// MarkVisited marks a URL as visited
	MarkVisited(normalizedURL string)

	// HasVisited checks if a URL has been visited
	HasVisited(normalizedURL string) bool

	// Stats returns frontier statistics
	Stats() FrontierStats
}

// FrontierStats holds statistics about the frontier.
type FrontierStats struct {
	Queued      int
	Visited     int
	TotalAdded  int
	Duplicates  int
	DepthCounts map[int]int
}

// MemoryFrontier is an in-memory implementation of Frontier.
type MemoryFrontier struct {
	mu            sync.RWMutex
	queue         *list.List            // For BFS (FIFO)
	stack         []*URLItem            // For DFS (LIFO)
	visited       map[string]struct{}   // Set of visited normalized URLs
	queued        map[string]struct{}   // Set of URLs currently in queue
	mode          config.TraversalMode
	maxDepth      int
	maxURLs       int
	totalAdded    int
	duplicates    int
	depthCounts   map[int]int
}

// NewMemoryFrontier creates a new in-memory frontier.
func NewMemoryFrontier(mode config.TraversalMode, maxDepth, maxURLs int) *MemoryFrontier {
	return &MemoryFrontier{
		queue:       list.New(),
		stack:       make([]*URLItem, 0),
		visited:     make(map[string]struct{}),
		queued:      make(map[string]struct{}),
		mode:        mode,
		maxDepth:    maxDepth,
		maxURLs:     maxURLs,
		depthCounts: make(map[int]int),
	}
}

// Push adds a URL to the frontier.
func (f *MemoryFrontier) Push(item *URLItem) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check depth limit
	if f.maxDepth > 0 && item.Depth > f.maxDepth {
		return false
	}

	// Check max URLs limit
	if f.maxURLs > 0 && f.totalAdded >= f.maxURLs {
		return false
	}

	// Check for duplicates
	if _, exists := f.visited[item.NormalizedURL]; exists {
		f.duplicates++
		return false
	}
	if _, exists := f.queued[item.NormalizedURL]; exists {
		f.duplicates++
		return false
	}

	// Add to queue based on traversal mode
	if f.mode == config.DFS {
		f.stack = append(f.stack, item)
	} else {
		f.queue.PushBack(item)
	}

	f.queued[item.NormalizedURL] = struct{}{}
	f.totalAdded++
	f.depthCounts[item.Depth]++

	return true
}

// Pop removes and returns the next URL to crawl.
func (f *MemoryFrontier) Pop() *URLItem {
	f.mu.Lock()
	defer f.mu.Unlock()

	var item *URLItem

	if f.mode == config.DFS {
		if len(f.stack) == 0 {
			return nil
		}
		// Pop from stack (LIFO)
		item = f.stack[len(f.stack)-1]
		f.stack = f.stack[:len(f.stack)-1]
	} else {
		// Pop from queue (FIFO)
		elem := f.queue.Front()
		if elem == nil {
			return nil
		}
		item = f.queue.Remove(elem).(*URLItem)
	}

	delete(f.queued, item.NormalizedURL)
	return item
}

// Peek returns the next URL without removing it.
func (f *MemoryFrontier) Peek() *URLItem {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.mode == config.DFS {
		if len(f.stack) == 0 {
			return nil
		}
		return f.stack[len(f.stack)-1]
	}

	elem := f.queue.Front()
	if elem == nil {
		return nil
	}
	return elem.Value.(*URLItem)
}

// Size returns the number of URLs in the frontier.
func (f *MemoryFrontier) Size() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.mode == config.DFS {
		return len(f.stack)
	}
	return f.queue.Len()
}

// IsEmpty returns true if the frontier is empty.
func (f *MemoryFrontier) IsEmpty() bool {
	return f.Size() == 0
}

// Contains checks if a normalized URL is already in the frontier or visited.
func (f *MemoryFrontier) Contains(normalizedURL string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if _, exists := f.visited[normalizedURL]; exists {
		return true
	}
	if _, exists := f.queued[normalizedURL]; exists {
		return true
	}
	return false
}

// MarkVisited marks a URL as visited.
func (f *MemoryFrontier) MarkVisited(normalizedURL string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.visited[normalizedURL] = struct{}{}
}

// HasVisited checks if a URL has been visited.
func (f *MemoryFrontier) HasVisited(normalizedURL string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, exists := f.visited[normalizedURL]
	return exists
}

// Stats returns frontier statistics.
func (f *MemoryFrontier) Stats() FrontierStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	depthCounts := make(map[int]int)
	for k, v := range f.depthCounts {
		depthCounts[k] = v
	}

	size := len(f.stack)
	if f.mode == config.BFS {
		size = f.queue.Len()
	}

	return FrontierStats{
		Queued:      size,
		Visited:     len(f.visited),
		TotalAdded:  f.totalAdded,
		Duplicates:  f.duplicates,
		DepthCounts: depthCounts,
	}
}

// Requeue adds a URL back to the frontier for retry.
func (f *MemoryFrontier) Requeue(item *URLItem) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// For retries, always add to front/top for immediate retry
	if f.mode == config.DFS {
		f.stack = append(f.stack, item)
	} else {
		f.queue.PushFront(item)
	}
	f.queued[item.NormalizedURL] = struct{}{}
}
