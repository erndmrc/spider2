// Package checkpoint provides crawl state persistence for crash recovery.
package checkpoint

import (
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Checkpoint represents a saved crawl state.
type Checkpoint struct {
	ID           string          `json:"id"`
	CreatedAt    time.Time       `json:"created_at"`
	CrawlState   *CrawlState     `json:"crawl_state"`
	QueueState   *QueueState     `json:"queue_state"`
	ConfigJSON   string          `json:"config_json"`
	Stats        *CheckpointStats `json:"stats"`
	Version      int             `json:"version"`
}

// CrawlState holds the state of the crawl.
type CrawlState struct {
	StartURL      string            `json:"start_url"`
	StartedAt     time.Time         `json:"started_at"`
	LastActivity  time.Time         `json:"last_activity"`
	CrawledURLs   map[string]bool   `json:"crawled_urls"`
	FailedURLs    map[string]int    `json:"failed_urls"` // URL -> retry count
	SeenURLs      map[string]bool   `json:"seen_urls"`
	Depth         map[string]int    `json:"depth"`       // URL -> depth
	Status        string            `json:"status"`      // running, paused, completed
}

// QueueState holds the state of the URL queue.
type QueueState struct {
	Pending   []QueueEntry `json:"pending"`
	InFlight  []QueueEntry `json:"in_flight"`
}

// QueueEntry represents a URL in the queue.
type QueueEntry struct {
	URL          string    `json:"url"`
	Depth        int       `json:"depth"`
	Priority     int       `json:"priority"`
	DiscoveredAt time.Time `json:"discovered_at"`
	ParentURL    string    `json:"parent_url,omitempty"`
}

// CheckpointStats holds checkpoint statistics.
type CheckpointStats struct {
	TotalURLs     int   `json:"total_urls"`
	CrawledURLs   int   `json:"crawled_urls"`
	PendingURLs   int   `json:"pending_urls"`
	FailedURLs    int   `json:"failed_urls"`
	QueueSize     int   `json:"queue_size"`
}

// Manager handles checkpoint creation and recovery.
type Manager struct {
	mu sync.RWMutex

	// Configuration
	baseDir      string
	maxCheckpoints int
	autoInterval time.Duration
	compression  bool

	// State
	currentCheckpoint *Checkpoint
	lastSave          time.Time

	// Auto-save
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// ManagerConfig defines checkpoint manager configuration.
type ManagerConfig struct {
	BaseDir        string
	MaxCheckpoints int           // Maximum number of checkpoints to keep
	AutoInterval   time.Duration // Auto-save interval (0 = disabled)
	Compression    bool          // Use gzip compression
}

// DefaultManagerConfig returns default configuration.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		BaseDir:        ".spider_checkpoints",
		MaxCheckpoints: 5,
		AutoInterval:   5 * time.Minute,
		Compression:    true,
	}
}

// NewManager creates a new checkpoint manager.
func NewManager(config *ManagerConfig) (*Manager, error) {
	if config == nil {
		config = DefaultManagerConfig()
	}

	// Create directory
	if err := os.MkdirAll(config.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	return &Manager{
		baseDir:        config.BaseDir,
		maxCheckpoints: config.MaxCheckpoints,
		autoInterval:   config.AutoInterval,
		compression:    config.Compression,
		stopChan:       make(chan struct{}),
	}, nil
}

// StartAutoSave starts automatic checkpoint saving.
func (m *Manager) StartAutoSave(getState func() (*CrawlState, *QueueState)) {
	if m.autoInterval <= 0 {
		return
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		ticker := time.NewTicker(m.autoInterval)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopChan:
				return
			case <-ticker.C:
				crawlState, queueState := getState()
				if crawlState != nil {
					m.Save(crawlState, queueState, "")
				}
			}
		}
	}()
}

// Stop stops auto-save.
func (m *Manager) Stop() {
	close(m.stopChan)
	m.wg.Wait()
}

// Save creates a new checkpoint.
func (m *Manager) Save(crawlState *CrawlState, queueState *QueueState, configJSON string) (*Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create checkpoint
	checkpoint := &Checkpoint{
		ID:         fmt.Sprintf("checkpoint_%d", time.Now().UnixNano()),
		CreatedAt:  time.Now(),
		CrawlState: crawlState,
		QueueState: queueState,
		ConfigJSON: configJSON,
		Version:    1,
		Stats: &CheckpointStats{
			TotalURLs:   len(crawlState.SeenURLs),
			CrawledURLs: len(crawlState.CrawledURLs),
			FailedURLs:  len(crawlState.FailedURLs),
		},
	}

	if queueState != nil {
		checkpoint.Stats.QueueSize = len(queueState.Pending)
		checkpoint.Stats.PendingURLs = len(queueState.Pending) + len(queueState.InFlight)
	}

	// Save to file
	filename := filepath.Join(m.baseDir, checkpoint.ID+".checkpoint")
	if err := m.saveToFile(checkpoint, filename); err != nil {
		return nil, err
	}

	m.currentCheckpoint = checkpoint
	m.lastSave = time.Now()

	// Cleanup old checkpoints
	m.cleanupOld()

	return checkpoint, nil
}

// saveToFile saves checkpoint to file.
func (m *Manager) saveToFile(checkpoint *Checkpoint, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create checkpoint file: %w", err)
	}
	defer file.Close()

	var writer io.Writer = file

	if m.compression {
		filename += ".gz"
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = gzWriter
	}

	encoder := gob.NewEncoder(writer)
	return encoder.Encode(checkpoint)
}

// Load loads a checkpoint from file.
func (m *Manager) Load(id string) (*Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	filename := filepath.Join(m.baseDir, id+".checkpoint")
	return m.loadFromFile(filename)
}

// loadFromFile loads checkpoint from file.
func (m *Manager) loadFromFile(filename string) (*Checkpoint, error) {
	file, err := os.Open(filename)
	if err != nil {
		// Try compressed version
		file, err = os.Open(filename + ".gz")
		if err != nil {
			return nil, fmt.Errorf("failed to open checkpoint file: %w", err)
		}
	}
	defer file.Close()

	var reader io.Reader = file

	// Check if gzipped
	if filepath.Ext(filename) == ".gz" || m.compression {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			// Not gzipped, reset file
			file.Seek(0, 0)
		} else {
			defer gzReader.Close()
			reader = gzReader
		}
	}

	var checkpoint Checkpoint
	decoder := gob.NewDecoder(reader)
	if err := decoder.Decode(&checkpoint); err != nil {
		return nil, fmt.Errorf("failed to decode checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// LoadLatest loads the most recent checkpoint.
func (m *Manager) LoadLatest() (*Checkpoint, error) {
	checkpoints, err := m.List()
	if err != nil {
		return nil, err
	}

	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints found")
	}

	// Sort by creation time (newest first)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].CreatedAt.After(checkpoints[j].CreatedAt)
	})

	return m.Load(checkpoints[0].ID)
}

// List returns all available checkpoints.
func (m *Manager) List() ([]*CheckpointInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint directory: %w", err)
	}

	var checkpoints []*CheckpointInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".checkpoint" && filepath.Ext(name) != ".gz" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		id := name
		id = id[:len(id)-len(filepath.Ext(id))] // Remove extension
		if filepath.Ext(id) == ".checkpoint" {
			id = id[:len(id)-len(".checkpoint")]
		}

		checkpoints = append(checkpoints, &CheckpointInfo{
			ID:        id,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	return checkpoints, nil
}

// CheckpointInfo contains basic checkpoint information.
type CheckpointInfo struct {
	ID        string
	Size      int64
	CreatedAt time.Time
}

// cleanupOld removes old checkpoints beyond the limit.
func (m *Manager) cleanupOld() {
	checkpoints, err := m.List()
	if err != nil || len(checkpoints) <= m.maxCheckpoints {
		return
	}

	// Sort by creation time (oldest first)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].CreatedAt.Before(checkpoints[j].CreatedAt)
	})

	// Remove oldest
	toRemove := len(checkpoints) - m.maxCheckpoints
	for i := 0; i < toRemove; i++ {
		filename := filepath.Join(m.baseDir, checkpoints[i].ID+".checkpoint")
		os.Remove(filename)
		os.Remove(filename + ".gz")
	}
}

// Delete removes a checkpoint.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filename := filepath.Join(m.baseDir, id+".checkpoint")
	os.Remove(filename)
	os.Remove(filename + ".gz")
	return nil
}

// Clear removes all checkpoints.
func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			os.Remove(filepath.Join(m.baseDir, entry.Name()))
		}
	}

	return nil
}

// GetLastSave returns the time of last save.
func (m *Manager) GetLastSave() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSave
}

// Recovery provides crawl recovery functionality.
type Recovery struct {
	manager    *Manager
	checkpoint *Checkpoint
}

// NewRecovery creates a recovery handler from a checkpoint.
func NewRecovery(manager *Manager, checkpoint *Checkpoint) *Recovery {
	return &Recovery{
		manager:    manager,
		checkpoint: checkpoint,
	}
}

// GetCrawlState returns the crawl state.
func (r *Recovery) GetCrawlState() *CrawlState {
	return r.checkpoint.CrawlState
}

// GetQueueState returns the queue state.
func (r *Recovery) GetQueueState() *QueueState {
	return r.checkpoint.QueueState
}

// GetPendingURLs returns URLs that were pending.
func (r *Recovery) GetPendingURLs() []QueueEntry {
	if r.checkpoint.QueueState == nil {
		return nil
	}
	return r.checkpoint.QueueState.Pending
}

// GetInFlightURLs returns URLs that were being processed.
func (r *Recovery) GetInFlightURLs() []QueueEntry {
	if r.checkpoint.QueueState == nil {
		return nil
	}
	return r.checkpoint.QueueState.InFlight
}

// GetCrawledURLs returns already crawled URLs.
func (r *Recovery) GetCrawledURLs() map[string]bool {
	if r.checkpoint.CrawlState == nil {
		return nil
	}
	return r.checkpoint.CrawlState.CrawledURLs
}

// GetFailedURLs returns failed URLs with retry counts.
func (r *Recovery) GetFailedURLs() map[string]int {
	if r.checkpoint.CrawlState == nil {
		return nil
	}
	return r.checkpoint.CrawlState.FailedURLs
}

// GetConfig returns the saved configuration.
func (r *Recovery) GetConfig() string {
	return r.checkpoint.ConfigJSON
}

// ToJSON exports checkpoint info as JSON.
func (c *Checkpoint) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}
