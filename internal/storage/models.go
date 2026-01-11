// Package storage provides data persistence for crawl results.
package storage

import "time"

// URL represents a discovered URL in the database.
type URL struct {
	ID             int64     `json:"id"`
	URL            string    `json:"url"`
	NormalizedURL  string    `json:"normalized_url"`
	Host           string    `json:"host"`
	Path           string    `json:"path"`
	DiscoveredFrom *int64    `json:"discovered_from,omitempty"` // Parent URL ID
	Depth          int       `json:"depth"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	CrawlStatus    string    `json:"crawl_status"` // pending, crawled, failed, skipped
	IsInternal     bool      `json:"is_internal"`
	InSitemap      bool      `json:"in_sitemap"`
}

// Fetch represents the result of fetching a URL.
type Fetch struct {
	ID              int64         `json:"id"`
	URLID           int64         `json:"url_id"`
	StatusCode      int           `json:"status_code"`
	Status          string        `json:"status"` // OK, Redirect, Client Error, Server Error, Timeout, etc.
	ContentType     string        `json:"content_type"`
	ContentLength   int64         `json:"content_length"`
	ResponseTime    time.Duration `json:"response_time"`
	TTFB            time.Duration `json:"ttfb"`
	FinalURLID      *int64        `json:"final_url_id,omitempty"`
	RedirectChainID *int64        `json:"redirect_chain_id,omitempty"`
	FetchedAt       time.Time     `json:"fetched_at"`
	ErrorMessage    string        `json:"error_message,omitempty"`
	RetryCount      int           `json:"retry_count"`

	// Headers (stored as JSON)
	Headers map[string]string `json:"headers,omitempty"`

	// TLS info
	TLSVersion   string `json:"tls_version,omitempty"`
	TLSIssuer    string `json:"tls_issuer,omitempty"`
	TLSExpiry    string `json:"tls_expiry,omitempty"`
}

// HTMLFeatures contains SEO-relevant HTML features.
type HTMLFeatures struct {
	ID              int64  `json:"id"`
	URLID           int64  `json:"url_id"`
	Title           string `json:"title"`
	TitleLength     int    `json:"title_length"`
	MetaDescription string `json:"meta_description"`
	MetaDescLength  int    `json:"meta_desc_length"`
	MetaKeywords    string `json:"meta_keywords"`
	MetaRobots      string `json:"meta_robots"`
	Canonical       string `json:"canonical"`
	CanonicalURLID  *int64 `json:"canonical_url_id,omitempty"`

	// Headings
	H1Count int    `json:"h1_count"`
	H1First string `json:"h1_first"`
	H1All   string `json:"h1_all"` // JSON array
	H2Count int    `json:"h2_count"`
	H2All   string `json:"h2_all"` // JSON array

	// Content metrics
	WordCount   int    `json:"word_count"`
	ContentHash string `json:"content_hash"` // For duplicate detection

	// Language
	Language string `json:"language"`

	// Hreflang (JSON array)
	Hreflangs string `json:"hreflangs,omitempty"`

	// Open Graph
	OGTitle       string `json:"og_title,omitempty"`
	OGDescription string `json:"og_description,omitempty"`
	OGImage       string `json:"og_image,omitempty"`

	// Indexability
	IsIndexable  bool   `json:"is_indexable"`
	IndexStatus  string `json:"index_status"` // Indexable, Noindex, Blocked by robots, etc.
}

// Link represents a link relationship between pages.
type Link struct {
	ID         int64  `json:"id"`
	FromURLID  int64  `json:"from_url_id"`
	ToURL      string `json:"to_url"`
	ToURLID    *int64 `json:"to_url_id,omitempty"`
	AnchorText string `json:"anchor_text"`
	LinkType   string `json:"link_type"` // a, link, area, form
	Rel        string `json:"rel"`       // nofollow, sponsored, ugc, etc.
	IsInternal bool   `json:"is_internal"`
	IsFollow   bool   `json:"is_follow"`
	Position   string `json:"position"` // header, nav, main, footer, sidebar (heuristic)
}

// Resource represents external resources (images, JS, CSS).
type Resource struct {
	ID           int64  `json:"id"`
	URL          string `json:"url"`
	ResourceURL  string `json:"resource_url"` // Alias for URL for clarity
	URLID        *int64 `json:"url_id,omitempty"`
	Type         string `json:"type"`          // Alias for ResourceType
	ResourceType string `json:"resource_type"` // image, script, stylesheet, iframe, video, audio
	MimeType     string `json:"mime_type"`
	StatusCode   int    `json:"status_code"`
	Size         int64  `json:"size"`
	FirstSeenOn  int64  `json:"first_seen_on"` // URL ID where first discovered

	// Image specific
	Alt     string `json:"alt,omitempty"`
	AltText string `json:"alt_text,omitempty"` // Alias for Alt
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`

	// Script specific
	IsAsync bool `json:"is_async,omitempty"`
	IsDefer bool `json:"is_defer,omitempty"`
}

// PageResource links pages to their resources (many-to-many).
type PageResource struct {
	ID         int64 `json:"id"`
	URLID      int64 `json:"url_id"`
	ResourceID int64 `json:"resource_id"`
}

// Issue represents an SEO issue found on a page.
type Issue struct {
	ID         int64     `json:"id"`
	URLID      int64     `json:"url_id"`
	IssueCode  string    `json:"issue_code"`  // e.g., "missing_title", "duplicate_meta", etc.
	IssueType  string    `json:"issue_type"`  // error, warning, notice
	Severity   string    `json:"severity"`    // critical, high, medium, low
	Category   string    `json:"category"`    // title, meta, content, links, images, etc.
	Message    string    `json:"message"`
	Details    string    `json:"details"` // JSON with additional context
	DetectedAt time.Time `json:"detected_at"`
}

// RedirectChain stores redirect sequences.
type RedirectChain struct {
	ID          int64  `json:"id"`
	SourceURLID int64  `json:"source_url_id"`
	SourceURL   string `json:"source_url"`
	FinalURLID  int64  `json:"final_url_id"`
	FinalURL    string `json:"final_url"`
	ChainLength int    `json:"chain_length"`
	Chain       string `json:"chain"` // JSON array of hops
	HasLoop     bool   `json:"has_loop"`
}

// CrawlSession stores crawl session metadata.
type CrawlSession struct {
	ID              int64     `json:"id"`
	StartURL        string    `json:"start_url"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Status          string    `json:"status"` // running, paused, completed, failed
	TotalURLs       int       `json:"total_urls"`
	CrawledURLs     int       `json:"crawled_urls"`
	FailedURLs      int       `json:"failed_urls"`
	ConfigJSON      string    `json:"config_json"` // Crawl config snapshot
	LastCheckpoint  time.Time `json:"last_checkpoint"`
}

// Sitemap stores sitemap information.
type Sitemap struct {
	ID          int64     `json:"id"`
	URL         string    `json:"url"`
	Type        string    `json:"type"` // index, urlset
	URLCount    int       `json:"url_count"`
	LastFetched time.Time `json:"last_fetched"`
	StatusCode  int       `json:"status_code"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
}

// SitemapURL links URLs found in sitemaps.
type SitemapURL struct {
	ID         int64      `json:"id"`
	SitemapID  int64      `json:"sitemap_id"`
	URLID      int64      `json:"url_id"`
	LastMod    *time.Time `json:"lastmod,omitempty"`
	ChangeFreq string     `json:"changefreq,omitempty"`
	Priority   float64    `json:"priority,omitempty"`
}

// Issue codes constants
const (
	// Title issues
	IssueMissingTitle    = "missing_title"
	IssueDuplicateTitle  = "duplicate_title"
	IssueTitleTooLong    = "title_too_long"
	IssueTitleTooShort   = "title_too_short"
	IssueMultipleTitles  = "multiple_titles"

	// Meta description issues
	IssueMissingMetaDesc   = "missing_meta_description"
	IssueDuplicateMetaDesc = "duplicate_meta_description"
	IssueMetaDescTooLong   = "meta_description_too_long"
	IssueMetaDescTooShort  = "meta_description_too_short"

	// H1 issues
	IssueMissingH1    = "missing_h1"
	IssueMultipleH1   = "multiple_h1"
	IssueDuplicateH1  = "duplicate_h1"

	// Canonical issues
	IssueMissingCanonical   = "missing_canonical"
	IssueCanonicalMismatch  = "canonical_mismatch"
	IssueCanonicalChain     = "canonical_chain"
	IssueCanonicalTo4xx     = "canonical_to_4xx"
	IssueCanonicalTo3xx     = "canonical_to_3xx"

	// Link issues
	IssueBrokenLink      = "broken_link"
	IssueRedirectLink    = "redirect_link"
	IssueOrphanPage      = "orphan_page"
	IssueNoInternalLinks = "no_internal_links"

	// Image issues
	IssueMissingAlt   = "missing_alt"
	IssueBrokenImage  = "broken_image"
	IssueLargeImage   = "large_image"

	// Content issues
	IssueThinContent      = "thin_content"
	IssueDuplicateContent = "duplicate_content"

	// Response issues
	IssueServerError    = "server_error"
	IssueClientError    = "client_error"
	IssueRedirectLoop   = "redirect_loop"
	IssueRedirectChain  = "redirect_chain"
	IssueSlowResponse   = "slow_response"

	// Indexability issues
	IssueNoindex       = "noindex"
	IssueBlockedRobots = "blocked_by_robots"
)

// Severity levels
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

// Issue types
const (
	IssueTypeError   = "error"
	IssueTypeWarning = "warning"
	IssueTypeNotice  = "notice"
)
