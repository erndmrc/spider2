// Package perf provides performance optimization utilities.
package perf

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DiskCache provides disk-based caching for crawl data.
type DiskCache struct {
	mu sync.RWMutex

	// Configuration
	baseDir      string
	maxSize      int64 // Maximum cache size in bytes
	maxAge       time.Duration
	compression  bool

	// State
	currentSize  int64
	entries      map[string]*CacheEntry
	accessOrder  []string // LRU tracking
}

// CacheEntry represents a cached item.
type CacheEntry struct {
	Key        string
	Size       int64
	CreatedAt  time.Time
	AccessedAt time.Time
	FilePath   string
	Hits       int64
}

// DiskCacheConfig defines cache configuration.
type DiskCacheConfig struct {
	BaseDir     string
	MaxSize     int64         // Maximum size in bytes (default 1GB)
	MaxAge      time.Duration // Maximum entry age (default 24h)
	Compression bool          // Enable compression
}

// DefaultDiskCacheConfig returns default configuration.
func DefaultDiskCacheConfig() *DiskCacheConfig {
	return &DiskCacheConfig{
		BaseDir:     ".spider_cache",
		MaxSize:     1 << 30, // 1GB
		MaxAge:      24 * time.Hour,
		Compression: false,
	}
}

// NewDiskCache creates a new disk cache.
func NewDiskCache(config *DiskCacheConfig) (*DiskCache, error) {
	if config == nil {
		config = DefaultDiskCacheConfig()
	}

	// Create cache directory
	if err := os.MkdirAll(config.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &DiskCache{
		baseDir:     config.BaseDir,
		maxSize:     config.MaxSize,
		maxAge:      config.MaxAge,
		compression: config.Compression,
		entries:     make(map[string]*CacheEntry),
		accessOrder: make([]string, 0),
	}

	// Load existing entries
	if err := cache.loadIndex(); err != nil {
		// Index load failed, start fresh
		cache.entries = make(map[string]*CacheEntry)
	}

	return cache, nil
}

// keyToPath converts a key to a file path.
func (c *DiskCache) keyToPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])

	// Use first 2 chars for subdirectory to avoid too many files in one dir
	subDir := hashStr[:2]
	fileName := hashStr[2:] + ".cache"

	return filepath.Join(c.baseDir, subDir, fileName)
}

// Set stores data in the cache.
func (c *DiskCache) Set(key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := int64(len(data))

	// Evict if necessary
	for c.currentSize+size > c.maxSize && len(c.accessOrder) > 0 {
		c.evictOldest()
	}

	// Create file path
	filePath := c.keyToPath(key)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache subdirectory: %w", err)
	}

	// Write data
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Update entry
	now := time.Now()
	entry := &CacheEntry{
		Key:        key,
		Size:       size,
		CreatedAt:  now,
		AccessedAt: now,
		FilePath:   filePath,
		Hits:       0,
	}

	// Remove old entry if exists
	if old, ok := c.entries[key]; ok {
		c.currentSize -= old.Size
		c.removeFromAccessOrder(key)
	}

	c.entries[key] = entry
	c.accessOrder = append(c.accessOrder, key)
	c.currentSize += size

	return nil
}

// Get retrieves data from the cache.
func (c *DiskCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	// Check age
	if time.Since(entry.CreatedAt) > c.maxAge {
		c.deleteEntry(key)
		return nil, false
	}

	// Read file
	data, err := os.ReadFile(entry.FilePath)
	if err != nil {
		c.deleteEntry(key)
		return nil, false
	}

	// Update access time
	entry.AccessedAt = time.Now()
	entry.Hits++

	// Move to end of access order (most recently used)
	c.removeFromAccessOrder(key)
	c.accessOrder = append(c.accessOrder, key)

	return data, true
}

// Has checks if a key exists in the cache.
func (c *DiskCache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return false
	}

	// Check age
	if time.Since(entry.CreatedAt) > c.maxAge {
		return false
	}

	return true
}

// Delete removes an entry from the cache.
func (c *DiskCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.deleteEntry(key)
}

// deleteEntry removes an entry (internal, must hold lock).
func (c *DiskCache) deleteEntry(key string) error {
	entry, ok := c.entries[key]
	if !ok {
		return nil
	}

	// Delete file
	os.Remove(entry.FilePath)

	// Update state
	c.currentSize -= entry.Size
	delete(c.entries, key)
	c.removeFromAccessOrder(key)

	return nil
}

// evictOldest removes the oldest entry.
func (c *DiskCache) evictOldest() {
	if len(c.accessOrder) == 0 {
		return
	}

	oldest := c.accessOrder[0]
	c.deleteEntry(oldest)
}

// removeFromAccessOrder removes a key from the access order.
func (c *DiskCache) removeFromAccessOrder(key string) {
	for i, k := range c.accessOrder {
		if k == key {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			return
		}
	}
}

// Clear removes all entries from the cache.
func (c *DiskCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove all files
	for key := range c.entries {
		c.deleteEntry(key)
	}

	c.entries = make(map[string]*CacheEntry)
	c.accessOrder = make([]string, 0)
	c.currentSize = 0

	return nil
}

// Stats returns cache statistics.
type CacheStats struct {
	EntryCount   int
	TotalSize    int64
	MaxSize      int64
	HitCount     int64
	OldestEntry  time.Time
	NewestEntry  time.Time
}

// Stats returns cache statistics.
func (c *DiskCache) Stats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &CacheStats{
		EntryCount: len(c.entries),
		TotalSize:  c.currentSize,
		MaxSize:    c.maxSize,
	}

	for _, entry := range c.entries {
		stats.HitCount += entry.Hits

		if stats.OldestEntry.IsZero() || entry.CreatedAt.Before(stats.OldestEntry) {
			stats.OldestEntry = entry.CreatedAt
		}
		if entry.CreatedAt.After(stats.NewestEntry) {
			stats.NewestEntry = entry.CreatedAt
		}
	}

	return stats
}

// Cleanup removes expired entries.
func (c *DiskCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	now := time.Now()

	for key, entry := range c.entries {
		if now.Sub(entry.CreatedAt) > c.maxAge {
			c.deleteEntry(key)
			removed++
		}
	}

	return removed
}

// saveIndex saves the cache index to disk.
func (c *DiskCache) saveIndex() error {
	indexPath := filepath.Join(c.baseDir, "index.gob")
	file, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(c.entries)
}

// loadIndex loads the cache index from disk.
func (c *DiskCache) loadIndex() error {
	indexPath := filepath.Join(c.baseDir, "index.gob")
	file, err := os.Open(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&c.entries); err != nil {
		return err
	}

	// Rebuild access order and calculate size
	c.accessOrder = make([]string, 0, len(c.entries))
	c.currentSize = 0

	for key, entry := range c.entries {
		c.accessOrder = append(c.accessOrder, key)
		c.currentSize += entry.Size
	}

	return nil
}

// Close saves the index and closes the cache.
func (c *DiskCache) Close() error {
	return c.saveIndex()
}

// ResponseCache caches HTTP responses.
type ResponseCache struct {
	cache *DiskCache
}

// CachedResponse represents a cached HTTP response.
type CachedResponse struct {
	URL         string
	StatusCode  int
	Headers     map[string]string
	Body        []byte
	ContentType string
	CachedAt    time.Time
}

// NewResponseCache creates a new response cache.
func NewResponseCache(config *DiskCacheConfig) (*ResponseCache, error) {
	if config == nil {
		config = DefaultDiskCacheConfig()
		config.BaseDir = ".spider_response_cache"
	}

	cache, err := NewDiskCache(config)
	if err != nil {
		return nil, err
	}

	return &ResponseCache{cache: cache}, nil
}

// Set caches a response.
func (r *ResponseCache) Set(url string, resp *CachedResponse) error {
	// Encode response
	var buf []byte
	// Simple encoding: could use gob for more efficiency
	data := fmt.Sprintf("%d\n%s\n%s", resp.StatusCode, resp.ContentType, string(resp.Body))
	buf = []byte(data)

	return r.cache.Set(url, buf)
}

// Get retrieves a cached response.
func (r *ResponseCache) Get(url string) (*CachedResponse, bool) {
	data, ok := r.cache.Get(url)
	if !ok {
		return nil, false
	}

	// Simple decoding
	resp := &CachedResponse{
		URL:  url,
		Body: data,
	}

	return resp, true
}

// Has checks if a URL is cached.
func (r *ResponseCache) Has(url string) bool {
	return r.cache.Has(url)
}

// Delete removes a cached response.
func (r *ResponseCache) Delete(url string) error {
	return r.cache.Delete(url)
}

// Clear clears all cached responses.
func (r *ResponseCache) Clear() error {
	return r.cache.Clear()
}

// Close closes the response cache.
func (r *ResponseCache) Close() error {
	return r.cache.Close()
}

// BodyReader provides a streaming reader that caches content.
type BodyReader struct {
	reader  io.ReadCloser
	buffer  []byte
	maxSize int64
	size    int64
	closed  bool
}

// NewBodyReader creates a new body reader.
func NewBodyReader(reader io.ReadCloser, maxSize int64) *BodyReader {
	return &BodyReader{
		reader:  reader,
		buffer:  make([]byte, 0),
		maxSize: maxSize,
	}
}

// Read reads from the body.
func (b *BodyReader) Read(p []byte) (n int, err error) {
	n, err = b.reader.Read(p)
	if n > 0 && b.size < b.maxSize {
		remaining := b.maxSize - b.size
		toStore := int64(n)
		if toStore > remaining {
			toStore = remaining
		}
		b.buffer = append(b.buffer, p[:toStore]...)
		b.size += toStore
	}
	return n, err
}

// Close closes the reader.
func (b *BodyReader) Close() error {
	b.closed = true
	return b.reader.Close()
}

// Bytes returns the cached bytes.
func (b *BodyReader) Bytes() []byte {
	return b.buffer
}

// Size returns the number of bytes read.
func (b *BodyReader) Size() int64 {
	return b.size
}

// Truncated returns true if content was truncated.
func (b *BodyReader) Truncated() bool {
	return b.size >= b.maxSize
}
