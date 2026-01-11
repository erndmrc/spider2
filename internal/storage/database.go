package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Database handles all database operations.
type Database struct {
	db        *sql.DB
	mu        sync.RWMutex
	batchSize int

	// Prepared statements cache
	stmts map[string]*sql.Stmt
}

// NewDatabase creates a new database connection.
func NewDatabase(path string) (*Database, error) {
	// SQLite connection with optimizations
	dsn := fmt.Sprintf("%s?_journal=WAL&_synchronous=NORMAL&_cache_size=10000&_busy_timeout=5000", path)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite only supports one writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	database := &Database{
		db:        db,
		batchSize: 100,
		stmts:     make(map[string]*sql.Stmt),
	}

	return database, nil
}

// Initialize creates tables and views.
func (d *Database) Initialize() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create tables
	if _, err := d.db.Exec(Schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create views
	if _, err := d.db.Exec(ViewsSchema); err != nil {
		return fmt.Errorf("failed to create views: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	// Close prepared statements
	for _, stmt := range d.stmts {
		stmt.Close()
	}
	return d.db.Close()
}

// --- URL Operations ---

// InsertURL inserts a new URL and returns its ID.
func (d *Database) InsertURL(url *URL) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`
		INSERT INTO urls (url, normalized_url, host, path, discovered_from, depth, crawl_status, is_internal, in_sitemap)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(normalized_url) DO UPDATE SET
			last_seen = CURRENT_TIMESTAMP
	`, url.URL, url.NormalizedURL, url.Host, url.Path, url.DiscoveredFrom, url.Depth, url.CrawlStatus, url.IsInternal, url.InSitemap)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// GetURLByNormalized retrieves a URL by its normalized form.
func (d *Database) GetURLByNormalized(normalizedURL string) (*URL, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var url URL
	err := d.db.QueryRow(`
		SELECT id, url, normalized_url, host, path, discovered_from, depth, first_seen, last_seen, crawl_status, is_internal, in_sitemap
		FROM urls WHERE normalized_url = ?
	`, normalizedURL).Scan(
		&url.ID, &url.URL, &url.NormalizedURL, &url.Host, &url.Path, &url.DiscoveredFrom,
		&url.Depth, &url.FirstSeen, &url.LastSeen, &url.CrawlStatus, &url.IsInternal, &url.InSitemap,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &url, nil
}

// UpdateURLStatus updates the crawl status of a URL.
func (d *Database) UpdateURLStatus(id int64, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`UPDATE urls SET crawl_status = ?, last_seen = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

// GetPendingURLs retrieves URLs pending crawl.
func (d *Database) GetPendingURLs(limit int) ([]*URL, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, url, normalized_url, host, path, discovered_from, depth, first_seen, last_seen, crawl_status, is_internal, in_sitemap
		FROM urls
		WHERE crawl_status = 'pending' AND is_internal = 1
		ORDER BY depth ASC, first_seen ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []*URL
	for rows.Next() {
		var url URL
		if err := rows.Scan(
			&url.ID, &url.URL, &url.NormalizedURL, &url.Host, &url.Path, &url.DiscoveredFrom,
			&url.Depth, &url.FirstSeen, &url.LastSeen, &url.CrawlStatus, &url.IsInternal, &url.InSitemap,
		); err != nil {
			return nil, err
		}
		urls = append(urls, &url)
	}
	return urls, rows.Err()
}

// --- Fetch Operations ---

// InsertFetch inserts a fetch record.
func (d *Database) InsertFetch(fetch *Fetch) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	headersJSON, _ := json.Marshal(fetch.Headers)

	result, err := d.db.Exec(`
		INSERT INTO fetches (url_id, status_code, status, content_type, content_length, response_time_ms, ttfb_ms,
			final_url_id, redirect_chain_id, error_message, retry_count, headers_json, tls_version, tls_issuer, tls_expiry)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fetch.URLID, fetch.StatusCode, fetch.Status, fetch.ContentType, fetch.ContentLength,
		fetch.ResponseTime.Milliseconds(), fetch.TTFB.Milliseconds(),
		fetch.FinalURLID, fetch.RedirectChainID, fetch.ErrorMessage, fetch.RetryCount,
		string(headersJSON), fetch.TLSVersion, fetch.TLSIssuer, fetch.TLSExpiry)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// --- HTML Features Operations ---

// InsertHTMLFeatures inserts or updates HTML features.
func (d *Database) InsertHTMLFeatures(features *HTMLFeatures) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`
		INSERT INTO html_features (url_id, title, title_length, meta_description, meta_desc_length,
			meta_keywords, meta_robots, canonical, canonical_url_id, h1_count, h1_first, h1_all,
			h2_count, h2_all, word_count, content_hash, language, hreflangs, og_title, og_description,
			og_image, is_indexable, index_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url_id) DO UPDATE SET
			title = excluded.title,
			title_length = excluded.title_length,
			meta_description = excluded.meta_description,
			meta_desc_length = excluded.meta_desc_length,
			meta_keywords = excluded.meta_keywords,
			meta_robots = excluded.meta_robots,
			canonical = excluded.canonical,
			canonical_url_id = excluded.canonical_url_id,
			h1_count = excluded.h1_count,
			h1_first = excluded.h1_first,
			h1_all = excluded.h1_all,
			h2_count = excluded.h2_count,
			h2_all = excluded.h2_all,
			word_count = excluded.word_count,
			content_hash = excluded.content_hash,
			language = excluded.language,
			hreflangs = excluded.hreflangs,
			og_title = excluded.og_title,
			og_description = excluded.og_description,
			og_image = excluded.og_image,
			is_indexable = excluded.is_indexable,
			index_status = excluded.index_status
	`, features.URLID, features.Title, features.TitleLength, features.MetaDescription, features.MetaDescLength,
		features.MetaKeywords, features.MetaRobots, features.Canonical, features.CanonicalURLID,
		features.H1Count, features.H1First, features.H1All, features.H2Count, features.H2All,
		features.WordCount, features.ContentHash, features.Language, features.Hreflangs,
		features.OGTitle, features.OGDescription, features.OGImage, features.IsIndexable, features.IndexStatus)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// --- Link Operations ---

// InsertLink inserts a link record.
func (d *Database) InsertLink(link *Link) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`
		INSERT INTO links (from_url_id, to_url, to_url_id, anchor_text, link_type, rel, is_internal, is_follow, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, link.FromURLID, link.ToURL, link.ToURLID, link.AnchorText, link.LinkType, link.Rel, link.IsInternal, link.IsFollow, link.Position)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// InsertLinks inserts multiple links in a batch.
func (d *Database) InsertLinks(links []*Link) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO links (from_url_id, to_url, to_url_id, anchor_text, link_type, rel, is_internal, is_follow, position)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, link := range links {
		_, err := stmt.Exec(link.FromURLID, link.ToURL, link.ToURLID, link.AnchorText, link.LinkType, link.Rel, link.IsInternal, link.IsFollow, link.Position)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetInlinks retrieves inlinks for a URL.
func (d *Database) GetInlinks(urlID int64) ([]*Link, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, from_url_id, to_url, to_url_id, anchor_text, link_type, rel, is_internal, is_follow, position
		FROM links WHERE to_url_id = ?
	`, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*Link
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.ID, &link.FromURLID, &link.ToURL, &link.ToURLID, &link.AnchorText, &link.LinkType, &link.Rel, &link.IsInternal, &link.IsFollow, &link.Position); err != nil {
			return nil, err
		}
		links = append(links, &link)
	}
	return links, rows.Err()
}

// GetOutlinks retrieves outlinks from a URL.
func (d *Database) GetOutlinks(urlID int64) ([]*Link, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, from_url_id, to_url, to_url_id, anchor_text, link_type, rel, is_internal, is_follow, position
		FROM links WHERE from_url_id = ?
	`, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*Link
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.ID, &link.FromURLID, &link.ToURL, &link.ToURLID, &link.AnchorText, &link.LinkType, &link.Rel, &link.IsInternal, &link.IsFollow, &link.Position); err != nil {
			return nil, err
		}
		links = append(links, &link)
	}
	return links, rows.Err()
}

// --- Issue Operations ---

// InsertIssue inserts an issue record.
func (d *Database) InsertIssue(issue *Issue) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`
		INSERT INTO issues (url_id, issue_code, issue_type, severity, category, message, details)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, issue.URLID, issue.IssueCode, issue.IssueType, issue.Severity, issue.Category, issue.Message, issue.Details)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// GetIssuesByURL retrieves all issues for a URL.
func (d *Database) GetIssuesByURL(urlID int64) ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, url_id, issue_code, issue_type, severity, category, message, details, detected_at
		FROM issues WHERE url_id = ?
	`, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []*Issue
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.URLID, &issue.IssueCode, &issue.IssueType, &issue.Severity, &issue.Category, &issue.Message, &issue.Details, &issue.DetectedAt); err != nil {
			return nil, err
		}
		issues = append(issues, &issue)
	}
	return issues, rows.Err()
}

// --- Resource Operations ---

// InsertResource inserts or gets a resource.
func (d *Database) InsertResource(resource *Resource) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`
		INSERT INTO resources (url, url_id, resource_type, mime_type, status_code, size, first_seen_on, alt, width, height, is_async, is_defer)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			status_code = excluded.status_code,
			size = excluded.size,
			mime_type = excluded.mime_type
	`, resource.URL, resource.URLID, resource.ResourceType, resource.MimeType, resource.StatusCode, resource.Size,
		resource.FirstSeenOn, resource.Alt, resource.Width, resource.Height, resource.IsAsync, resource.IsDefer)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// LinkPageResource links a page to a resource.
func (d *Database) LinkPageResource(urlID, resourceID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO page_resources (url_id, resource_id)
		VALUES (?, ?)
		ON CONFLICT(url_id, resource_id) DO NOTHING
	`, urlID, resourceID)

	return err
}

// --- Crawl Session Operations ---

// CreateSession creates a new crawl session.
func (d *Database) CreateSession(session *CrawlSession) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`
		INSERT INTO crawl_sessions (start_url, status, config_json)
		VALUES (?, ?, ?)
	`, session.StartURL, session.Status, session.ConfigJSON)

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// UpdateSessionProgress updates crawl session progress.
func (d *Database) UpdateSessionProgress(id int64, crawled, failed int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		UPDATE crawl_sessions
		SET crawled_urls = ?, failed_urls = ?, last_checkpoint = CURRENT_TIMESTAMP
		WHERE id = ?
	`, crawled, failed, id)

	return err
}

// CompleteSession marks a session as completed.
func (d *Database) CompleteSession(id int64, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		UPDATE crawl_sessions
		SET status = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, id)

	return err
}

// --- Statistics ---

// Stats holds database statistics.
type Stats struct {
	TotalURLs      int
	InternalURLs   int
	ExternalURLs   int
	CrawledURLs    int
	PendingURLs    int
	FailedURLs     int
	TotalLinks     int
	TotalResources int
	TotalIssues    int
	StatusCodes    map[int]int
}

// GetStats retrieves database statistics.
func (d *Database) GetStats() (*Stats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := &Stats{
		StatusCodes: make(map[int]int),
	}

	// URL counts
	d.db.QueryRow(`SELECT COUNT(*) FROM urls`).Scan(&stats.TotalURLs)
	d.db.QueryRow(`SELECT COUNT(*) FROM urls WHERE is_internal = 1`).Scan(&stats.InternalURLs)
	d.db.QueryRow(`SELECT COUNT(*) FROM urls WHERE is_internal = 0`).Scan(&stats.ExternalURLs)
	d.db.QueryRow(`SELECT COUNT(*) FROM urls WHERE crawl_status = 'crawled'`).Scan(&stats.CrawledURLs)
	d.db.QueryRow(`SELECT COUNT(*) FROM urls WHERE crawl_status = 'pending'`).Scan(&stats.PendingURLs)
	d.db.QueryRow(`SELECT COUNT(*) FROM urls WHERE crawl_status = 'failed'`).Scan(&stats.FailedURLs)

	// Other counts
	d.db.QueryRow(`SELECT COUNT(*) FROM links`).Scan(&stats.TotalLinks)
	d.db.QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&stats.TotalResources)
	d.db.QueryRow(`SELECT COUNT(*) FROM issues`).Scan(&stats.TotalIssues)

	// Status codes
	rows, err := d.db.Query(`SELECT status_code, COUNT(*) FROM fetches GROUP BY status_code`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var code, count int
			rows.Scan(&code, &count)
			stats.StatusCodes[code] = count
		}
	}

	return stats, nil
}

// --- Additional Query Methods for Reports ---

// GetAllURLs retrieves all URLs.
func (d *Database) GetAllURLs() ([]*URL, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, url, normalized_url, host, path, discovered_from, depth, first_seen, last_seen, crawl_status, is_internal, in_sitemap
		FROM urls
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []*URL
	for rows.Next() {
		var url URL
		if err := rows.Scan(
			&url.ID, &url.URL, &url.NormalizedURL, &url.Host, &url.Path, &url.DiscoveredFrom,
			&url.Depth, &url.FirstSeen, &url.LastSeen, &url.CrawlStatus, &url.IsInternal, &url.InSitemap,
		); err != nil {
			return nil, err
		}
		urls = append(urls, &url)
	}
	return urls, rows.Err()
}

// GetURLByID retrieves a URL by ID.
func (d *Database) GetURLByID(id int64) (*URL, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var url URL
	err := d.db.QueryRow(`
		SELECT id, url, normalized_url, host, path, discovered_from, depth, first_seen, last_seen, crawl_status, is_internal, in_sitemap
		FROM urls WHERE id = ?
	`, id).Scan(
		&url.ID, &url.URL, &url.NormalizedURL, &url.Host, &url.Path, &url.DiscoveredFrom,
		&url.Depth, &url.FirstSeen, &url.LastSeen, &url.CrawlStatus, &url.IsInternal, &url.InSitemap,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &url, nil
}

// GetURLByAddress retrieves a URL by its address.
func (d *Database) GetURLByAddress(urlStr string) (*URL, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var url URL
	err := d.db.QueryRow(`
		SELECT id, url, normalized_url, host, path, discovered_from, depth, first_seen, last_seen, crawl_status, is_internal, in_sitemap
		FROM urls WHERE url = ?
	`, urlStr).Scan(
		&url.ID, &url.URL, &url.NormalizedURL, &url.Host, &url.Path, &url.DiscoveredFrom,
		&url.Depth, &url.FirstSeen, &url.LastSeen, &url.CrawlStatus, &url.IsInternal, &url.InSitemap,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &url, nil
}

// GetLatestFetch retrieves the latest fetch for a URL.
func (d *Database) GetLatestFetch(urlID int64) (*Fetch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var fetch Fetch
	var headersJSON string
	var responseTimeMs, ttfbMs int64

	err := d.db.QueryRow(`
		SELECT id, url_id, status_code, status, content_type, content_length, response_time_ms, ttfb_ms,
			final_url_id, redirect_chain_id, error_message, retry_count, headers_json, tls_version, tls_issuer, tls_expiry, fetched_at
		FROM fetches
		WHERE url_id = ?
		ORDER BY fetched_at DESC
		LIMIT 1
	`, urlID).Scan(
		&fetch.ID, &fetch.URLID, &fetch.StatusCode, &fetch.Status, &fetch.ContentType, &fetch.ContentLength,
		&responseTimeMs, &ttfbMs, &fetch.FinalURLID, &fetch.RedirectChainID, &fetch.ErrorMessage, &fetch.RetryCount,
		&headersJSON, &fetch.TLSVersion, &fetch.TLSIssuer, &fetch.TLSExpiry, &fetch.FetchedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	fetch.ResponseTime = time.Duration(responseTimeMs) * time.Millisecond
	fetch.TTFB = time.Duration(ttfbMs) * time.Millisecond

	if headersJSON != "" {
		json.Unmarshal([]byte(headersJSON), &fetch.Headers)
	}

	return &fetch, nil
}

// GetHTMLFeatures retrieves HTML features for a URL.
func (d *Database) GetHTMLFeatures(urlID int64) (*HTMLFeatures, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var features HTMLFeatures
	err := d.db.QueryRow(`
		SELECT id, url_id, title, title_length, meta_description, meta_desc_length, meta_keywords, meta_robots,
			canonical, canonical_url_id, h1_count, h1_first, h1_all, h2_count, h2_all, word_count, content_hash,
			language, hreflangs, og_title, og_description, og_image, is_indexable, index_status
		FROM html_features
		WHERE url_id = ?
	`, urlID).Scan(
		&features.ID, &features.URLID, &features.Title, &features.TitleLength, &features.MetaDescription,
		&features.MetaDescLength, &features.MetaKeywords, &features.MetaRobots, &features.Canonical,
		&features.CanonicalURLID, &features.H1Count, &features.H1First, &features.H1All, &features.H2Count,
		&features.H2All, &features.WordCount, &features.ContentHash, &features.Language, &features.Hreflangs,
		&features.OGTitle, &features.OGDescription, &features.OGImage, &features.IsIndexable, &features.IndexStatus,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &features, nil
}

// GetRedirectChains retrieves all redirect chains.
func (d *Database) GetRedirectChains() ([]*RedirectChain, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, source_url_id, final_url_id, chain_length, chain_json
		FROM redirect_chains
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chains []*RedirectChain
	for rows.Next() {
		var chain RedirectChain
		var chainJSON string
		if err := rows.Scan(&chain.ID, &chain.SourceURLID, &chain.FinalURLID, &chain.ChainLength, &chainJSON); err != nil {
			return nil, err
		}
		// Get source and final URLs
		sourceURL, _ := d.GetURLByID(chain.SourceURLID)
		finalURL, _ := d.GetURLByID(chain.FinalURLID)
		if sourceURL != nil {
			chain.SourceURL = sourceURL.URL
		}
		if finalURL != nil {
			chain.FinalURL = finalURL.URL
		}
		chain.Chain = chainJSON
		chains = append(chains, &chain)
	}
	return chains, rows.Err()
}

// GetLinksToURL retrieves all links pointing to a URL.
func (d *Database) GetLinksToURL(urlID int64) ([]*LinkWithSource, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT l.id, l.from_url_id, l.to_url, l.anchor_text, l.is_internal, u.url as from_url
		FROM links l
		JOIN urls u ON l.from_url_id = u.id
		WHERE l.to_url_id = ?
	`, urlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*LinkWithSource
	for rows.Next() {
		var link LinkWithSource
		if err := rows.Scan(&link.ID, &link.FromURLID, &link.ToURL, &link.AnchorText, &link.IsInternal, &link.FromURL); err != nil {
			return nil, err
		}
		links = append(links, &link)
	}
	return links, rows.Err()
}

// LinkWithSource extends Link with source URL.
type LinkWithSource struct {
	ID         int64
	FromURLID  int64
	FromURL    string
	ToURL      string
	AnchorText string
	IsInternal bool
}

// GetAllLinks retrieves all links.
func (d *Database) GetAllLinks() ([]*Link, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, from_url_id, to_url, to_url_id, anchor_text, link_type, rel, is_internal, is_follow, position
		FROM links
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*Link
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.ID, &link.FromURLID, &link.ToURL, &link.ToURLID, &link.AnchorText,
			&link.LinkType, &link.Rel, &link.IsInternal, &link.IsFollow, &link.Position); err != nil {
			return nil, err
		}
		links = append(links, &link)
	}
	return links, rows.Err()
}

// GetAllResources retrieves all resources.
func (d *Database) GetAllResources() ([]*Resource, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, url, url_id, resource_type, mime_type, status_code, size, first_seen_on, alt, width, height, is_async, is_defer
		FROM resources
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resources []*Resource
	for rows.Next() {
		var res Resource
		if err := rows.Scan(&res.ID, &res.URL, &res.URLID, &res.ResourceType, &res.MimeType,
			&res.StatusCode, &res.Size, &res.FirstSeenOn, &res.Alt, &res.Width, &res.Height,
			&res.IsAsync, &res.IsDefer); err != nil {
			return nil, err
		}
		resources = append(resources, &res)
	}
	return resources, rows.Err()
}

// GetAllIssues retrieves all issues.
func (d *Database) GetAllIssues() ([]*Issue, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, url_id, issue_code, issue_type, severity, category, message, details, detected_at
		FROM issues
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []*Issue
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.URLID, &issue.IssueCode, &issue.IssueType,
			&issue.Severity, &issue.Category, &issue.Message, &issue.Details, &issue.DetectedAt); err != nil {
			return nil, err
		}
		issues = append(issues, &issue)
	}
	return issues, rows.Err()
}
