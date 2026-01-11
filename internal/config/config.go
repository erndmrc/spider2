// Package config defines crawl configuration options.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"
)

// TraversalMode defines how URLs are traversed in the queue.
type TraversalMode string

const (
	BFS TraversalMode = "bfs" // Breadth-First Search
	DFS TraversalMode = "dfs" // Depth-First Search
)

// RedirectPolicy defines how redirects are handled.
type RedirectPolicy string

const (
	RedirectFollow     RedirectPolicy = "follow"      // Follow redirects
	RedirectNoFollow   RedirectPolicy = "no_follow"   // Don't follow redirects
	RedirectFollowSame RedirectPolicy = "follow_same" // Follow only same-domain redirects
)

// RenderMode defines how pages are rendered.
type RenderMode string

const (
	RenderHTML     RenderMode = "html"     // HTML only (no JavaScript)
	RenderJS       RenderMode = "js"       // JavaScript rendering (Chromium)
	RenderAdaptive RenderMode = "adaptive" // Adaptive: JS only if needed
)

// WaitCondition defines when to consider page loaded.
type WaitCondition string

const (
	WaitDOMContentLoaded WaitCondition = "domcontentloaded" // Wait for DOMContentLoaded
	WaitLoad             WaitCondition = "load"             // Wait for load event
	WaitNetworkIdle      WaitCondition = "networkidle"      // Wait for network idle
	WaitSelector         WaitCondition = "selector"         // Wait for specific selector
)

// AuthType defines authentication method.
type AuthType string

const (
	AuthNone   AuthType = "none"   // No authentication
	AuthBasic  AuthType = "basic"  // HTTP Basic Auth
	AuthBearer AuthType = "bearer" // Bearer token
	AuthCookie AuthType = "cookie" // Cookie-based
	AuthForm   AuthType = "form"   // Form login
)

// CrawlConfig holds all configuration for a crawl session.
type CrawlConfig struct {
	// === Basic Settings ===

	// Seed URLs to start crawling from
	Seeds []string `json:"seeds"`

	// Traversal mode: BFS or DFS
	TraversalMode TraversalMode `json:"traversal_mode"`

	// User-Agent string
	UserAgent string `json:"user_agent"`

	// === Include/Exclude (5.1) ===

	// URL patterns to include (regex)
	IncludePatterns []string `json:"include_patterns"`

	// URL patterns to exclude (regex)
	ExcludePatterns []string `json:"exclude_patterns"`

	// Crawl outside of start folder
	CrawlOutsideStartFolder bool `json:"crawl_outside_start_folder"`

	// Include subdomains as internal
	IncludeSubdomains bool `json:"include_subdomains"`

	// === Limits (5.2) ===

	// Maximum crawl depth (0 = unlimited)
	MaxDepth int `json:"max_depth"`

	// Maximum number of URLs to crawl (0 = unlimited)
	MaxURLs int `json:"max_urls"`

	// Maximum query parameters to include
	MaxQueryParams int `json:"max_query_params"`

	// Maximum response size in bytes (0 = unlimited)
	MaxResponseSize int64 `json:"max_response_size"`

	// Crawl duration limit (0 = unlimited)
	CrawlDuration time.Duration `json:"crawl_duration"`

	// === Speed & Concurrency ===

	// Maximum requests per second (0 = unlimited)
	RequestsPerSecond float64 `json:"requests_per_second"`

	// Number of concurrent workers
	Concurrency int `json:"concurrency"`

	// Per-host crawl delay (politeness)
	CrawlDelay time.Duration `json:"crawl_delay"`

	// Per-host rate limit (requests per second per host)
	PerHostRateLimit float64 `json:"per_host_rate_limit"`

	// Request timeout
	Timeout time.Duration `json:"timeout"`

	// Maximum number of retries for failed requests
	MaxRetries int `json:"max_retries"`

	// Base delay for exponential backoff
	RetryBackoff time.Duration `json:"retry_backoff"`

	// === Redirects ===

	// Maximum number of redirects to follow
	MaxRedirects int `json:"max_redirects"`

	// Redirect handling policy
	RedirectPolicy RedirectPolicy `json:"redirect_policy"`

	// === Rendering (5.3) ===

	// Render mode: html, js, adaptive
	RenderMode RenderMode `json:"render_mode"`

	// Render timeout (for JS rendering)
	RenderTimeout time.Duration `json:"render_timeout"`

	// Wait condition for JS rendering
	WaitCondition WaitCondition `json:"wait_condition"`

	// Selector to wait for (when WaitCondition = selector)
	WaitSelector string `json:"wait_selector"`

	// Chromium executable path (empty = bundled)
	ChromiumPath string `json:"chromium_path"`

	// === Authentication (5.4) ===

	// Authentication type
	AuthType AuthType `json:"auth_type"`

	// Authentication configuration
	Auth *AuthConfig `json:"auth,omitempty"`

	// Custom headers to inject
	CustomHeaders map[string]string `json:"custom_headers,omitempty"`

	// Cookies to use
	Cookies []*CookieConfig `json:"cookies,omitempty"`

	// === Robots & Nofollow (5.5) ===

	// Respect robots.txt
	RespectRobotsTxt bool `json:"respect_robots_txt"`

	// Respect nofollow on links
	RespectNofollow bool `json:"respect_nofollow"`

	// Crawl canonical URLs
	FollowCanonicals bool `json:"follow_canonicals"`

	// Crawl URLs in sitemaps even if not linked
	CrawlSitemapURLs bool `json:"crawl_sitemap_urls"`

	// === URL Normalization ===

	// Query parameters to ignore (utm_*, gclid, etc.)
	IgnoreQueryParams []string `json:"ignore_query_params"`

	// Sort query parameters for normalization
	SortQueryParams bool `json:"sort_query_params"`

	// Remove trailing slash for normalization
	RemoveTrailingSlash bool `json:"remove_trailing_slash"`

	// Convert to lowercase for normalization
	LowercaseURLs bool `json:"lowercase_urls"`

	// === Content Types ===

	// Content types to process (empty = all)
	AllowedContentTypes []string `json:"allowed_content_types,omitempty"`

	// File extensions to exclude
	ExcludeExtensions []string `json:"exclude_extensions,omitempty"`

	// === Storage ===

	// Store raw HTML in database
	StoreHTML bool `json:"store_html"`

	// Store response headers
	StoreHeaders bool `json:"store_headers"`

	// === Compiled patterns (not serialized) ===
	compiledIncludes []*regexp.Regexp
	compiledExcludes []*regexp.Regexp
}

// AuthConfig holds authentication credentials.
type AuthConfig struct {
	// Basic/Bearer auth
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`

	// Form login
	LoginURL    string            `json:"login_url,omitempty"`
	FormFields  map[string]string `json:"form_fields,omitempty"`
	SuccessURL  string            `json:"success_url,omitempty"`
	SuccessText string            `json:"success_text,omitempty"`
}

// CookieConfig holds cookie information.
type CookieConfig struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"http_only"`
}

// DefaultConfig returns a CrawlConfig with sensible defaults.
func DefaultConfig() *CrawlConfig {
	return &CrawlConfig{
		// Basic
		TraversalMode: BFS,
		UserAgent:     "SpiderCrawler/1.0 (+https://github.com/spider-crawler)",

		// Include/Exclude
		CrawlOutsideStartFolder: false,
		IncludeSubdomains:       true,

		// Limits
		MaxDepth:        0, // unlimited
		MaxURLs:         0, // unlimited
		MaxQueryParams:  0, // unlimited
		MaxResponseSize: 10 * 1024 * 1024, // 10MB
		CrawlDuration:   0, // unlimited

		// Speed & Concurrency
		RequestsPerSecond: 10,
		Concurrency:       5,
		CrawlDelay:        time.Second,
		PerHostRateLimit:  2,
		Timeout:           30 * time.Second,
		MaxRetries:        3,
		RetryBackoff:      time.Second,

		// Redirects
		MaxRedirects:   10,
		RedirectPolicy: RedirectFollow,

		// Rendering
		RenderMode:    RenderHTML,
		RenderTimeout: 30 * time.Second,
		WaitCondition: WaitDOMContentLoaded,

		// Authentication
		AuthType: AuthNone,

		// Robots & Nofollow
		RespectRobotsTxt: true,
		RespectNofollow:  false,
		FollowCanonicals: true,
		CrawlSitemapURLs: false,

		// URL Normalization
		IgnoreQueryParams: []string{
			"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
			"gclid", "fbclid", "msclkid", "ref", "source",
		},
		SortQueryParams:     true,
		RemoveTrailingSlash: true,
		LowercaseURLs:       true,

		// Content Types
		ExcludeExtensions: []string{
			".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
			".zip", ".rar", ".tar", ".gz", ".7z",
			".mp3", ".mp4", ".avi", ".mov", ".wmv", ".flv",
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".ico", ".svg", ".webp",
			".css", ".js", ".woff", ".woff2", ".ttf", ".eot",
		},

		// Storage
		StoreHTML:    true,
		StoreHeaders: true,
	}
}

// Validate checks if the configuration is valid.
func (c *CrawlConfig) Validate() error {
	if c.Concurrency < 1 {
		c.Concurrency = 1
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.Timeout < time.Second {
		c.Timeout = time.Second
	}
	if c.MaxRedirects < 0 {
		c.MaxRedirects = 0
	}
	if c.RenderTimeout < time.Second {
		c.RenderTimeout = time.Second
	}
	return nil
}

// CompilePatterns compiles include/exclude regex patterns.
func (c *CrawlConfig) CompilePatterns() error {
	c.compiledIncludes = make([]*regexp.Regexp, 0, len(c.IncludePatterns))
	c.compiledExcludes = make([]*regexp.Regexp, 0, len(c.ExcludePatterns))

	for _, pattern := range c.IncludePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid include pattern '%s': %w", pattern, err)
		}
		c.compiledIncludes = append(c.compiledIncludes, re)
	}

	for _, pattern := range c.ExcludePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid exclude pattern '%s': %w", pattern, err)
		}
		c.compiledExcludes = append(c.compiledExcludes, re)
	}

	return nil
}

// ShouldCrawl checks if a URL should be crawled based on include/exclude patterns.
func (c *CrawlConfig) ShouldCrawl(urlStr string) bool {
	// Check exclude patterns first
	for _, re := range c.compiledExcludes {
		if re.MatchString(urlStr) {
			return false
		}
	}

	// If no include patterns, include everything
	if len(c.compiledIncludes) == 0 {
		return true
	}

	// Check include patterns
	for _, re := range c.compiledIncludes {
		if re.MatchString(urlStr) {
			return true
		}
	}

	return false
}

// IsExtensionExcluded checks if a file extension should be excluded.
func (c *CrawlConfig) IsExtensionExcluded(ext string) bool {
	for _, excluded := range c.ExcludeExtensions {
		if ext == excluded {
			return true
		}
	}
	return false
}

// Save saves the configuration to a JSON file.
func (c *CrawlConfig) Save(filePath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Load loads configuration from a JSON file.
func Load(filePath string) (*CrawlConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if err := config.CompilePatterns(); err != nil {
		return nil, fmt.Errorf("failed to compile patterns: %w", err)
	}

	return config, nil
}

// Clone creates a deep copy of the configuration.
func (c *CrawlConfig) Clone() *CrawlConfig {
	clone := *c

	// Deep copy slices
	clone.Seeds = make([]string, len(c.Seeds))
	copy(clone.Seeds, c.Seeds)

	clone.IncludePatterns = make([]string, len(c.IncludePatterns))
	copy(clone.IncludePatterns, c.IncludePatterns)

	clone.ExcludePatterns = make([]string, len(c.ExcludePatterns))
	copy(clone.ExcludePatterns, c.ExcludePatterns)

	clone.IgnoreQueryParams = make([]string, len(c.IgnoreQueryParams))
	copy(clone.IgnoreQueryParams, c.IgnoreQueryParams)

	clone.ExcludeExtensions = make([]string, len(c.ExcludeExtensions))
	copy(clone.ExcludeExtensions, c.ExcludeExtensions)

	// Deep copy maps
	if c.CustomHeaders != nil {
		clone.CustomHeaders = make(map[string]string)
		for k, v := range c.CustomHeaders {
			clone.CustomHeaders[k] = v
		}
	}

	// Deep copy cookies
	if c.Cookies != nil {
		clone.Cookies = make([]*CookieConfig, len(c.Cookies))
		for i, cookie := range c.Cookies {
			cookieCopy := *cookie
			clone.Cookies[i] = &cookieCopy
		}
	}

	// Deep copy auth
	if c.Auth != nil {
		authCopy := *c.Auth
		if c.Auth.FormFields != nil {
			authCopy.FormFields = make(map[string]string)
			for k, v := range c.Auth.FormFields {
				authCopy.FormFields[k] = v
			}
		}
		clone.Auth = &authCopy
	}

	return &clone
}

// Presets for common crawl scenarios
var (
	// PresetFast is optimized for fast crawling
	PresetFast = &CrawlConfig{
		TraversalMode:     BFS,
		Concurrency:       20,
		RequestsPerSecond: 50,
		CrawlDelay:        100 * time.Millisecond,
		PerHostRateLimit:  10,
		Timeout:           10 * time.Second,
		RenderMode:        RenderHTML,
		RespectRobotsTxt:  false,
		StoreHTML:         false,
	}

	// PresetPolite is optimized for polite crawling
	PresetPolite = &CrawlConfig{
		TraversalMode:     BFS,
		Concurrency:       2,
		RequestsPerSecond: 1,
		CrawlDelay:        2 * time.Second,
		PerHostRateLimit:  0.5,
		Timeout:           60 * time.Second,
		RenderMode:        RenderHTML,
		RespectRobotsTxt:  true,
		RespectNofollow:   true,
		StoreHTML:         true,
	}

	// PresetJSRendering is optimized for JavaScript-heavy sites
	PresetJSRendering = &CrawlConfig{
		TraversalMode:     BFS,
		Concurrency:       5,
		RequestsPerSecond: 5,
		CrawlDelay:        time.Second,
		Timeout:           60 * time.Second,
		RenderMode:        RenderJS,
		RenderTimeout:     30 * time.Second,
		WaitCondition:     WaitNetworkIdle,
		RespectRobotsTxt:  true,
		StoreHTML:         true,
	}

	// PresetSEOAudit is optimized for SEO auditing
	PresetSEOAudit = &CrawlConfig{
		TraversalMode:     BFS,
		Concurrency:       10,
		RequestsPerSecond: 10,
		CrawlDelay:        500 * time.Millisecond,
		Timeout:           30 * time.Second,
		RenderMode:        RenderHTML,
		RespectRobotsTxt:  true,
		FollowCanonicals:  true,
		CrawlSitemapURLs:  true,
		StoreHTML:         true,
		StoreHeaders:      true,
	}
)
