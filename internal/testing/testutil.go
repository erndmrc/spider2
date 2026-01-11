// Package testing provides test utilities for the spider crawler.
package testing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TestServer provides a configurable test HTTP server.
type TestServer struct {
	Server    *httptest.Server
	mu        sync.RWMutex
	pages     map[string]*TestPage
	delays    map[string]time.Duration
	errors    map[string]int // URL -> status code
	hits      map[string]int
	redirects map[string]string
}

// TestPage represents a test page.
type TestPage struct {
	Content     string
	ContentType string
	StatusCode  int
	Headers     map[string]string
}

// NewTestServer creates a new test server.
func NewTestServer() *TestServer {
	ts := &TestServer{
		pages:     make(map[string]*TestPage),
		delays:    make(map[string]time.Duration),
		errors:    make(map[string]int),
		hits:      make(map[string]int),
		redirects: make(map[string]string),
	}

	ts.Server = httptest.NewServer(http.HandlerFunc(ts.handler))
	return ts
}

// handler handles test HTTP requests.
func (ts *TestServer) handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	ts.mu.Lock()
	ts.hits[path]++
	ts.mu.Unlock()

	ts.mu.RLock()
	delay := ts.delays[path]
	errorCode := ts.errors[path]
	redirect := ts.redirects[path]
	page := ts.pages[path]
	ts.mu.RUnlock()

	// Apply delay
	if delay > 0 {
		time.Sleep(delay)
	}

	// Handle redirect
	if redirect != "" {
		http.Redirect(w, r, redirect, http.StatusMovedPermanently)
		return
	}

	// Handle error
	if errorCode > 0 {
		w.WriteHeader(errorCode)
		return
	}

	// Handle page
	if page != nil {
		for k, v := range page.Headers {
			w.Header().Set(k, v)
		}
		if page.ContentType != "" {
			w.Header().Set("Content-Type", page.ContentType)
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		if page.StatusCode > 0 {
			w.WriteHeader(page.StatusCode)
		}
		io.WriteString(w, page.Content)
		return
	}

	// Default 404
	w.WriteHeader(http.StatusNotFound)
}

// AddPage adds a test page.
func (ts *TestServer) AddPage(path, content string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.pages[path] = &TestPage{
		Content:     content,
		ContentType: "text/html; charset=utf-8",
		StatusCode:  200,
	}
}

// AddPageWithType adds a page with specific content type.
func (ts *TestServer) AddPageWithType(path, content, contentType string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.pages[path] = &TestPage{
		Content:     content,
		ContentType: contentType,
		StatusCode:  200,
	}
}

// AddPageWithStatus adds a page with specific status code.
func (ts *TestServer) AddPageWithStatus(path, content string, status int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.pages[path] = &TestPage{
		Content:     content,
		ContentType: "text/html; charset=utf-8",
		StatusCode:  status,
	}
}

// SetDelay sets response delay for a path.
func (ts *TestServer) SetDelay(path string, delay time.Duration) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.delays[path] = delay
}

// SetError sets error status for a path.
func (ts *TestServer) SetError(path string, statusCode int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.errors[path] = statusCode
}

// SetRedirect sets redirect for a path.
func (ts *TestServer) SetRedirect(from, to string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.redirects[from] = to
}

// GetHits returns hit count for a path.
func (ts *TestServer) GetHits(path string) int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.hits[path]
}

// GetAllHits returns all hit counts.
func (ts *TestServer) GetAllHits() map[string]int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make(map[string]int)
	for k, v := range ts.hits {
		result[k] = v
	}
	return result
}

// URL returns the server URL.
func (ts *TestServer) URL() string {
	return ts.Server.URL
}

// Close closes the test server.
func (ts *TestServer) Close() {
	ts.Server.Close()
}

// Reset clears all state.
func (ts *TestServer) Reset() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.pages = make(map[string]*TestPage)
	ts.delays = make(map[string]time.Duration)
	ts.errors = make(map[string]int)
	ts.hits = make(map[string]int)
	ts.redirects = make(map[string]string)
}

// BuildTestSite creates a test site with common pages.
func (ts *TestServer) BuildTestSite() {
	// Home page
	ts.AddPage("/", `<!DOCTYPE html>
<html>
<head>
	<title>Test Site Home</title>
	<meta name="description" content="This is the test site home page">
	<link rel="canonical" href="`+ts.URL()+`/">
</head>
<body>
	<h1>Welcome to Test Site</h1>
	<nav>
		<a href="/about">About</a>
		<a href="/products">Products</a>
		<a href="/blog">Blog</a>
		<a href="/contact">Contact</a>
	</nav>
</body>
</html>`)

	// About page
	ts.AddPage("/about", `<!DOCTYPE html>
<html>
<head>
	<title>About Us</title>
	<meta name="description" content="About our company">
</head>
<body>
	<h1>About Us</h1>
	<p>We are a test company.</p>
	<a href="/">Home</a>
</body>
</html>`)

	// Products page
	ts.AddPage("/products", `<!DOCTYPE html>
<html>
<head>
	<title>Our Products</title>
</head>
<body>
	<h1>Products</h1>
	<ul>
		<li><a href="/products/1">Product 1</a></li>
		<li><a href="/products/2">Product 2</a></li>
		<li><a href="/products/3">Product 3</a></li>
	</ul>
</body>
</html>`)

	// Product pages
	for i := 1; i <= 3; i++ {
		ts.AddPage(fmt.Sprintf("/products/%d", i), fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Product %d</title>
</head>
<body>
	<h1>Product %d</h1>
	<p>Description of product %d</p>
	<img src="/images/product%d.jpg" alt="Product %d image">
	<a href="/products">Back to Products</a>
</body>
</html>`, i, i, i, i, i))
	}

	// Blog page
	ts.AddPage("/blog", `<!DOCTYPE html>
<html>
<head>
	<title>Blog</title>
</head>
<body>
	<h1>Blog</h1>
	<article>
		<h2><a href="/blog/post-1">First Post</a></h2>
	</article>
	<article>
		<h2><a href="/blog/post-2">Second Post</a></h2>
	</article>
</body>
</html>`)

	// Contact page
	ts.AddPage("/contact", `<!DOCTYPE html>
<html>
<head>
	<title>Contact Us</title>
</head>
<body>
	<h1>Contact</h1>
	<p>Email: test@example.com</p>
</body>
</html>`)

	// robots.txt
	ts.AddPageWithType("/robots.txt", `User-agent: *
Disallow: /private/
Sitemap: `+ts.URL()+`/sitemap.xml`, "text/plain")

	// sitemap.xml
	ts.AddPageWithType("/sitemap.xml", `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>`+ts.URL()+`/</loc></url>
	<url><loc>`+ts.URL()+`/about</loc></url>
	<url><loc>`+ts.URL()+`/products</loc></url>
	<url><loc>`+ts.URL()+`/contact</loc></url>
</urlset>`, "application/xml")
}

// HTMLBuilder helps build test HTML content.
type HTMLBuilder struct {
	title       string
	metaDesc    string
	canonical   string
	h1          string
	h2s         []string
	links       []Link
	images      []Image
	scripts     []string
	styles      []string
	bodyContent string
}

// Link represents a link for testing.
type Link struct {
	Href   string
	Text   string
	Rel    string
}

// Image represents an image for testing.
type Image struct {
	Src string
	Alt string
}

// NewHTMLBuilder creates a new HTML builder.
func NewHTMLBuilder() *HTMLBuilder {
	return &HTMLBuilder{
		h2s:     make([]string, 0),
		links:   make([]Link, 0),
		images:  make([]Image, 0),
		scripts: make([]string, 0),
		styles:  make([]string, 0),
	}
}

// Title sets the page title.
func (b *HTMLBuilder) Title(title string) *HTMLBuilder {
	b.title = title
	return b
}

// MetaDescription sets the meta description.
func (b *HTMLBuilder) MetaDescription(desc string) *HTMLBuilder {
	b.metaDesc = desc
	return b
}

// Canonical sets the canonical URL.
func (b *HTMLBuilder) Canonical(url string) *HTMLBuilder {
	b.canonical = url
	return b
}

// H1 sets the H1 heading.
func (b *HTMLBuilder) H1(text string) *HTMLBuilder {
	b.h1 = text
	return b
}

// H2 adds an H2 heading.
func (b *HTMLBuilder) H2(text string) *HTMLBuilder {
	b.h2s = append(b.h2s, text)
	return b
}

// Link adds a link.
func (b *HTMLBuilder) Link(href, text string) *HTMLBuilder {
	b.links = append(b.links, Link{Href: href, Text: text})
	return b
}

// LinkWithRel adds a link with rel attribute.
func (b *HTMLBuilder) LinkWithRel(href, text, rel string) *HTMLBuilder {
	b.links = append(b.links, Link{Href: href, Text: text, Rel: rel})
	return b
}

// Img adds an image.
func (b *HTMLBuilder) Img(src, alt string) *HTMLBuilder {
	b.images = append(b.images, Image{Src: src, Alt: alt})
	return b
}

// Script adds a script.
func (b *HTMLBuilder) Script(src string) *HTMLBuilder {
	b.scripts = append(b.scripts, src)
	return b
}

// Style adds a stylesheet.
func (b *HTMLBuilder) Style(href string) *HTMLBuilder {
	b.styles = append(b.styles, href)
	return b
}

// Body sets body content.
func (b *HTMLBuilder) Body(content string) *HTMLBuilder {
	b.bodyContent = content
	return b
}

// Build generates the HTML.
func (b *HTMLBuilder) Build() string {
	var sb strings.Builder

	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")

	if b.title != "" {
		sb.WriteString(fmt.Sprintf("  <title>%s</title>\n", b.title))
	}
	if b.metaDesc != "" {
		sb.WriteString(fmt.Sprintf("  <meta name=\"description\" content=\"%s\">\n", b.metaDesc))
	}
	if b.canonical != "" {
		sb.WriteString(fmt.Sprintf("  <link rel=\"canonical\" href=\"%s\">\n", b.canonical))
	}
	for _, style := range b.styles {
		sb.WriteString(fmt.Sprintf("  <link rel=\"stylesheet\" href=\"%s\">\n", style))
	}

	sb.WriteString("</head>\n<body>\n")

	if b.h1 != "" {
		sb.WriteString(fmt.Sprintf("  <h1>%s</h1>\n", b.h1))
	}
	for _, h2 := range b.h2s {
		sb.WriteString(fmt.Sprintf("  <h2>%s</h2>\n", h2))
	}

	if b.bodyContent != "" {
		sb.WriteString(b.bodyContent)
		sb.WriteString("\n")
	}

	for _, link := range b.links {
		if link.Rel != "" {
			sb.WriteString(fmt.Sprintf("  <a href=\"%s\" rel=\"%s\">%s</a>\n", link.Href, link.Rel, link.Text))
		} else {
			sb.WriteString(fmt.Sprintf("  <a href=\"%s\">%s</a>\n", link.Href, link.Text))
		}
	}

	for _, img := range b.images {
		sb.WriteString(fmt.Sprintf("  <img src=\"%s\" alt=\"%s\">\n", img.Src, img.Alt))
	}

	for _, script := range b.scripts {
		sb.WriteString(fmt.Sprintf("  <script src=\"%s\"></script>\n", script))
	}

	sb.WriteString("</body>\n</html>")

	return sb.String()
}

// Snapshot provides export comparison for regression testing.
type Snapshot struct {
	Name      string
	Data      interface{}
	Timestamp time.Time
	FilePath  string
}

// SnapshotManager handles snapshot creation and comparison.
type SnapshotManager struct {
	baseDir string
}

// NewSnapshotManager creates a new snapshot manager.
func NewSnapshotManager(baseDir string) *SnapshotManager {
	os.MkdirAll(baseDir, 0755)
	return &SnapshotManager{baseDir: baseDir}
}

// Save saves a snapshot.
func (sm *SnapshotManager) Save(name string, data interface{}) (*Snapshot, error) {
	snapshot := &Snapshot{
		Name:      name,
		Data:      data,
		Timestamp: time.Now(),
		FilePath:  filepath.Join(sm.baseDir, name+".snapshot.json"),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(snapshot.FilePath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write snapshot: %w", err)
	}

	return snapshot, nil
}

// Load loads a snapshot.
func (sm *SnapshotManager) Load(name string, target interface{}) error {
	filePath := filepath.Join(sm.baseDir, name+".snapshot.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read snapshot: %w", err)
	}

	return json.Unmarshal(data, target)
}

// Compare compares current data with a snapshot.
func (sm *SnapshotManager) Compare(name string, current interface{}) (*SnapshotDiff, error) {
	currentJSON, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return nil, err
	}

	snapshotPath := filepath.Join(sm.baseDir, name+".snapshot.json")
	snapshotJSON, err := os.ReadFile(snapshotPath)
	if err != nil {
		// Snapshot doesn't exist, create it
		sm.Save(name, current)
		return &SnapshotDiff{IsNew: true}, nil
	}

	diff := &SnapshotDiff{
		Current:  string(currentJSON),
		Snapshot: string(snapshotJSON),
		Match:    string(currentJSON) == string(snapshotJSON),
	}

	return diff, nil
}

// SnapshotDiff represents the difference between current and snapshot.
type SnapshotDiff struct {
	IsNew    bool
	Match    bool
	Current  string
	Snapshot string
}

// Update updates a snapshot with current data.
func (sm *SnapshotManager) Update(name string, data interface{}) error {
	_, err := sm.Save(name, data)
	return err
}

// List lists all snapshots.
func (sm *SnapshotManager) List() ([]string, error) {
	entries, err := os.ReadDir(sm.baseDir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".snapshot.json") {
			name := strings.TrimSuffix(entry.Name(), ".snapshot.json")
			names = append(names, name)
		}
	}

	return names, nil
}
