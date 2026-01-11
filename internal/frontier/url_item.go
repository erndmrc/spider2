// Package frontier implements the URL frontier (queue) for crawling.
package frontier

import "time"

// URLItem represents a URL in the frontier queue.
type URLItem struct {
	// The raw URL string
	URL string

	// Normalized version of the URL
	NormalizedURL string

	// The URL this was discovered from (empty for seeds)
	DiscoveredFrom string

	// Crawl depth (0 for seeds)
	Depth int

	// Number of retry attempts
	RetryCount int

	// When this URL was added to the queue
	AddedAt time.Time

	// When this URL should be crawled (for retry backoff)
	ScheduledAt time.Time

	// Host extracted from URL
	Host string

	// Priority (lower = higher priority)
	Priority int
}

// NewURLItem creates a new URLItem with the given URL.
func NewURLItem(url, normalizedURL, host string, depth int, discoveredFrom string) *URLItem {
	now := time.Now()
	return &URLItem{
		URL:            url,
		NormalizedURL:  normalizedURL,
		Host:           host,
		Depth:          depth,
		DiscoveredFrom: discoveredFrom,
		RetryCount:     0,
		AddedAt:        now,
		ScheduledAt:    now,
		Priority:       depth, // Default priority based on depth
	}
}

// CanCrawl checks if this URL is ready to be crawled based on schedule.
func (u *URLItem) CanCrawl() bool {
	return time.Now().After(u.ScheduledAt) || time.Now().Equal(u.ScheduledAt)
}

// IncrementRetry increases retry count and updates scheduled time with backoff.
func (u *URLItem) IncrementRetry(backoffDuration time.Duration) {
	u.RetryCount++
	// Exponential backoff: backoff * 2^retryCount
	delay := backoffDuration * time.Duration(1<<uint(u.RetryCount))
	u.ScheduledAt = time.Now().Add(delay)
}
