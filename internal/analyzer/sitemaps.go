package analyzer

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/spider-crawler/spider/internal/storage"
)

// SitemapsAnalyzer analyzes sitemaps and their contents.
type SitemapsAnalyzer struct {
	sitemapURLs map[string]bool   // URLs found in sitemaps
	crawledURLs map[string]bool   // URLs found via crawling
}

func NewSitemapsAnalyzer() *SitemapsAnalyzer {
	return &SitemapsAnalyzer{
		sitemapURLs: make(map[string]bool),
		crawledURLs: make(map[string]bool),
	}
}

func (a *SitemapsAnalyzer) Name() string {
	return "Sitemaps"
}

func (a *SitemapsAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "URL", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "type", Title: "Type", Width: 80, Sortable: true, DataKey: "type"},
		{ID: "status", Title: "Status", Width: 80, Sortable: true, DataKey: "status"},
		{ID: "in_sitemap", Title: "In Sitemap", Width: 80, Sortable: true, DataKey: "in_sitemap"},
		{ID: "in_crawl", Title: "In Crawl", Width: 80, Sortable: true, DataKey: "in_crawl"},
		{ID: "lastmod", Title: "Last Modified", Width: 120, Sortable: true, DataKey: "lastmod"},
	}
}

func (a *SitemapsAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "sitemap_only", Label: "Sitemap Only", Description: "URLs only in sitemap (orphans)", FilterFunc: func(r *AnalysisResult) bool {
			inSitemap, _ := r.Data["in_sitemap"].(bool)
			inCrawl, _ := r.Data["in_crawl"].(bool)
			return inSitemap && !inCrawl
		}},
		{ID: "crawl_only", Label: "Not In Sitemap", Description: "Crawled but not in sitemap", FilterFunc: func(r *AnalysisResult) bool {
			inSitemap, _ := r.Data["in_sitemap"].(bool)
			inCrawl, _ := r.Data["in_crawl"].(bool)
			return !inSitemap && inCrawl
		}},
		{ID: "both", Label: "In Both", Description: "URLs in sitemap and crawled", FilterFunc: func(r *AnalysisResult) bool {
			inSitemap, _ := r.Data["in_sitemap"].(bool)
			inCrawl, _ := r.Data["in_crawl"].(bool)
			return inSitemap && inCrawl
		}},
		{ID: "sitemap_error", Label: "Sitemap Errors", Description: "Sitemaps with errors", FilterFunc: func(r *AnalysisResult) bool {
			if t, ok := r.Data["type"].(string); ok {
				if status, ok := r.Data["status_code"].(int); ok {
					return (t == "sitemap" || t == "sitemap_index") && status >= 400
				}
			}
			return false
		}},
	}
}

// XMLSitemap represents a parsed sitemap.xml
type XMLSitemap struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []SitemapURL `xml:"url"`
}

// XMLSitemapIndex represents a parsed sitemap index.
type XMLSitemapIndex struct {
	XMLName  xml.Name `xml:"sitemapindex"`
	Sitemaps []SitemapEntry `xml:"sitemap"`
}

// SitemapURL represents a URL entry in sitemap.
type SitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

// SitemapEntry represents a sitemap entry in sitemap index.
type SitemapEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// ParseSitemapResult holds parsed sitemap data.
type ParseSitemapResult struct {
	URL        string
	Type       string // "sitemap", "sitemap_index", "error"
	StatusCode int
	URLs       []SitemapURL
	Sitemaps   []SitemapEntry
	Error      string
}

func (a *SitemapsAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	// Track crawled URL
	a.crawledURLs[ctx.URL.URL] = true

	// Check if URL is in sitemap
	result.Data["in_sitemap"] = a.sitemapURLs[ctx.URL.URL]
	result.Data["in_crawl"] = true

	return result
}

// ParseSitemap parses sitemap content.
func (a *SitemapsAnalyzer) ParseSitemap(url string, content []byte, statusCode int) *ParseSitemapResult {
	result := &ParseSitemapResult{
		URL:        url,
		StatusCode: statusCode,
		URLs:       make([]SitemapURL, 0),
		Sitemaps:   make([]SitemapEntry, 0),
	}

	if statusCode >= 400 {
		result.Type = "error"
		result.Error = fmt.Sprintf("HTTP %d", statusCode)
		return result
	}

	contentStr := string(content)

	// Try parsing as sitemap index first
	if strings.Contains(contentStr, "<sitemapindex") {
		var index XMLSitemapIndex
		if err := xml.Unmarshal(content, &index); err != nil {
			result.Type = "error"
			result.Error = fmt.Sprintf("XML parse error: %s", err.Error())
			return result
		}
		result.Type = "sitemap_index"
		result.Sitemaps = index.Sitemaps
		return result
	}

	// Try parsing as urlset
	if strings.Contains(contentStr, "<urlset") {
		var sitemap XMLSitemap
		if err := xml.Unmarshal(content, &sitemap); err != nil {
			result.Type = "error"
			result.Error = fmt.Sprintf("XML parse error: %s", err.Error())
			return result
		}
		result.Type = "sitemap"
		result.URLs = sitemap.URLs

		// Register URLs
		for _, u := range sitemap.URLs {
			a.sitemapURLs[u.Loc] = true
		}

		return result
	}

	result.Type = "error"
	result.Error = "Unknown sitemap format"
	return result
}

// AnalyzeSitemapComparison compares sitemap URLs with crawled URLs.
func (a *SitemapsAnalyzer) AnalyzeSitemapComparison() []*AnalysisResult {
	results := make([]*AnalysisResult, 0)

	// URLs only in sitemap (orphans)
	for url := range a.sitemapURLs {
		if !a.crawledURLs[url] {
			result := &AnalysisResult{
				Issues: make([]*storage.Issue, 0),
				Data: map[string]interface{}{
					"url":        url,
					"type":       "page",
					"in_sitemap": true,
					"in_crawl":   false,
					"status":     "Orphan",
				},
			}
			result.Issues = append(result.Issues, &storage.Issue{
				IssueCode: "sitemap_orphan",
				IssueType: storage.IssueTypeWarning,
				Severity:  storage.SeverityMedium,
				Category:  "sitemaps",
				Message:   fmt.Sprintf("URL in sitemap but not linked from site: %s", url),
			})
			results = append(results, result)
		}
	}

	// URLs not in sitemap
	for url := range a.crawledURLs {
		if !a.sitemapURLs[url] {
			result := &AnalysisResult{
				Issues: make([]*storage.Issue, 0),
				Data: map[string]interface{}{
					"url":        url,
					"type":       "page",
					"in_sitemap": false,
					"in_crawl":   true,
					"status":     "Not in Sitemap",
				},
			}
			results = append(results, result)
		}
	}

	return results
}

// AnalyzeSitemapEntry creates an analysis result for a sitemap entry.
func (a *SitemapsAnalyzer) AnalyzeSitemapEntry(entry SitemapURL) *AnalysisResult {
	result := &AnalysisResult{
		Issues: make([]*storage.Issue, 0),
		Data: map[string]interface{}{
			"url":        entry.Loc,
			"type":       "sitemap_entry",
			"in_sitemap": true,
			"lastmod":    entry.LastMod,
			"changefreq": entry.ChangeFreq,
			"priority":   entry.Priority,
		},
	}

	// Validate lastmod format
	if entry.LastMod != "" {
		if _, err := time.Parse(time.RFC3339, entry.LastMod); err != nil {
			if _, err := time.Parse("2006-01-02", entry.LastMod); err != nil {
				result.Issues = append(result.Issues, &storage.Issue{
					IssueCode: "sitemap_invalid_lastmod",
					IssueType: storage.IssueTypeWarning,
					Severity:  storage.SeverityLow,
					Category:  "sitemaps",
					Message:   fmt.Sprintf("Invalid lastmod format: %s", entry.LastMod),
				})
			}
		}
	}

	return result
}

func (a *SitemapsAnalyzer) Reset() {
	a.sitemapURLs = make(map[string]bool)
	a.crawledURLs = make(map[string]bool)
}

func (a *SitemapsAnalyzer) GetSitemapURLs() map[string]bool {
	return a.sitemapURLs
}

func (a *SitemapsAnalyzer) ExportRow(result *AnalysisResult) []string {
	inSitemap := "No"
	if s, ok := result.Data["in_sitemap"].(bool); ok && s {
		inSitemap = "Yes"
	}
	inCrawl := "No"
	if c, ok := result.Data["in_crawl"].(bool); ok && c {
		inCrawl = "Yes"
	}
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["type"]),
		fmt.Sprintf("%v", result.Data["status"]),
		inSitemap,
		inCrawl,
		fmt.Sprintf("%v", result.Data["lastmod"]),
	}
}
