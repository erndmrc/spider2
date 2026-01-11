// Package report provides pre-built reports and export functionality.
package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// ReportType defines the type of report.
type ReportType string

const (
	ReportAllRedirects        ReportType = "all_redirects"
	ReportRedirectChains      ReportType = "redirect_chains"
	ReportClientErrors        ReportType = "client_errors_4xx"
	ReportServerErrors        ReportType = "server_errors_5xx"
	ReportCanonicalErrors     ReportType = "canonical_errors"
	ReportMissingTitles       ReportType = "missing_titles"
	ReportMissingMetaDesc     ReportType = "missing_meta_desc"
	ReportMissingH1           ReportType = "missing_h1"
	ReportDuplicateTitles     ReportType = "duplicate_titles"
	ReportDuplicateMetaDesc   ReportType = "duplicate_meta_desc"
	ReportDuplicateContent    ReportType = "duplicate_content"
	ReportMissingAlt          ReportType = "missing_alt"
	ReportOrphanURLs          ReportType = "orphan_urls"
	ReportNonIndexable        ReportType = "non_indexable"
	ReportNoInternalInlinks   ReportType = "no_internal_inlinks"
	ReportBrokenLinks         ReportType = "broken_links"
	ReportAllIssues           ReportType = "all_issues"
	ReportSEOOverview         ReportType = "seo_overview"
	ReportCrawlSummary        ReportType = "crawl_summary"
)

// ReportDefinition defines a report type.
type ReportDefinition struct {
	Type        ReportType
	Name        string
	Description string
	Category    string
	Columns     []string
}

// AllReports returns all available report definitions.
func AllReports() []*ReportDefinition {
	return []*ReportDefinition{
		// Response Codes
		{ReportAllRedirects, "All Redirects", "All URLs returning 3xx status codes", "Response Codes", []string{"URL", "Status Code", "Redirect URL", "Redirect Type"}},
		{ReportRedirectChains, "Redirect Chains", "URLs with redirect chains", "Response Codes", []string{"Source URL", "Chain Length", "Final URL", "Chain"}},
		{ReportClientErrors, "Client Errors (4xx)", "All URLs returning 4xx status codes", "Response Codes", []string{"URL", "Status Code", "Found On", "Anchor Text"}},
		{ReportServerErrors, "Server Errors (5xx)", "All URLs returning 5xx status codes", "Response Codes", []string{"URL", "Status Code", "Found On"}},

		// On-Page
		{ReportMissingTitles, "Missing Titles", "Pages without title tags", "On-Page", []string{"URL", "Status Code", "Indexability"}},
		{ReportMissingMetaDesc, "Missing Meta Descriptions", "Pages without meta descriptions", "On-Page", []string{"URL", "Status Code", "Title"}},
		{ReportMissingH1, "Missing H1", "Pages without H1 headings", "On-Page", []string{"URL", "Status Code", "Title"}},
		{ReportDuplicateTitles, "Duplicate Titles", "Pages with duplicate title tags", "On-Page", []string{"Title", "Count", "URLs"}},
		{ReportDuplicateMetaDesc, "Duplicate Meta Descriptions", "Pages with duplicate meta descriptions", "On-Page", []string{"Meta Description", "Count", "URLs"}},
		{ReportDuplicateContent, "Duplicate Content", "Pages with duplicate content", "On-Page", []string{"Content Hash", "Count", "URLs"}},

		// Canonicals
		{ReportCanonicalErrors, "Canonical Errors", "Pages with canonical issues", "Canonicals", []string{"URL", "Canonical", "Issue Type", "Details"}},

		// Images
		{ReportMissingAlt, "Missing Alt Text", "Images without alt attributes", "Images", []string{"Image URL", "Found On", "Occurrences"}},

		// Links
		{ReportBrokenLinks, "Broken Links", "All broken internal and external links", "Links", []string{"Link URL", "Status Code", "Found On", "Anchor Text"}},
		{ReportOrphanURLs, "Orphan URLs", "Pages not linked from other pages", "Links", []string{"URL", "Source", "Status Code"}},
		{ReportNoInternalInlinks, "No Internal Inlinks", "Pages with no internal links pointing to them", "Links", []string{"URL", "Status Code", "External Inlinks"}},

		// Indexability
		{ReportNonIndexable, "Non-Indexable Pages", "Pages blocked from indexing", "Indexability", []string{"URL", "Reason", "Meta Robots", "X-Robots-Tag"}},

		// Summary
		{ReportAllIssues, "All Issues", "Complete list of all detected issues", "Summary", []string{"URL", "Issue Type", "Severity", "Category", "Message"}},
		{ReportSEOOverview, "SEO Overview", "High-level SEO metrics", "Summary", []string{"Metric", "Value", "Status"}},
		{ReportCrawlSummary, "Crawl Summary", "Summary of crawl statistics", "Summary", []string{"Metric", "Value"}},
	}
}

// ReportRow represents a single row in a report.
type ReportRow struct {
	Values map[string]interface{}
}

// Report represents a generated report.
type Report struct {
	Definition *ReportDefinition
	Rows       []*ReportRow
	TotalCount int
	Generated  string // Timestamp
}

// Generator generates reports from crawl data.
type Generator struct {
	db *storage.Database
}

// NewGenerator creates a new report generator.
func NewGenerator(db *storage.Database) *Generator {
	return &Generator{db: db}
}

// Generate generates a report of the specified type.
func (g *Generator) Generate(reportType ReportType) (*Report, error) {
	def := g.getDefinition(reportType)
	if def == nil {
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}

	report := &Report{
		Definition: def,
		Rows:       make([]*ReportRow, 0),
	}

	var err error
	switch reportType {
	case ReportAllRedirects:
		err = g.generateAllRedirects(report)
	case ReportRedirectChains:
		err = g.generateRedirectChains(report)
	case ReportClientErrors:
		err = g.generateClientErrors(report)
	case ReportServerErrors:
		err = g.generateServerErrors(report)
	case ReportCanonicalErrors:
		err = g.generateCanonicalErrors(report)
	case ReportMissingTitles:
		err = g.generateMissingTitles(report)
	case ReportMissingMetaDesc:
		err = g.generateMissingMetaDesc(report)
	case ReportMissingH1:
		err = g.generateMissingH1(report)
	case ReportDuplicateTitles:
		err = g.generateDuplicateTitles(report)
	case ReportDuplicateMetaDesc:
		err = g.generateDuplicateMetaDesc(report)
	case ReportDuplicateContent:
		err = g.generateDuplicateContent(report)
	case ReportMissingAlt:
		err = g.generateMissingAlt(report)
	case ReportOrphanURLs:
		err = g.generateOrphanURLs(report)
	case ReportNonIndexable:
		err = g.generateNonIndexable(report)
	case ReportNoInternalInlinks:
		err = g.generateNoInternalInlinks(report)
	case ReportBrokenLinks:
		err = g.generateBrokenLinks(report)
	case ReportAllIssues:
		err = g.generateAllIssues(report)
	case ReportSEOOverview:
		err = g.generateSEOOverview(report)
	case ReportCrawlSummary:
		err = g.generateCrawlSummary(report)
	default:
		err = fmt.Errorf("report generator not implemented: %s", reportType)
	}

	if err != nil {
		return nil, err
	}

	report.TotalCount = len(report.Rows)
	return report, nil
}

func (g *Generator) getDefinition(reportType ReportType) *ReportDefinition {
	for _, def := range AllReports() {
		if def.Type == reportType {
			return def
		}
	}
	return nil
}

func (g *Generator) generateAllRedirects(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	for _, url := range urls {
		fetch, err := g.db.GetLatestFetch(url.ID)
		if err != nil || fetch == nil {
			continue
		}

		if fetch.StatusCode >= 300 && fetch.StatusCode < 400 {
			redirectType := "Permanent"
			if fetch.StatusCode == 302 || fetch.StatusCode == 307 {
				redirectType = "Temporary"
			}

			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":           url.URL,
					"Status Code":   fetch.StatusCode,
					"Redirect URL":  fetch.RedirectURL,
					"Redirect Type": redirectType,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateRedirectChains(report *Report) error {
	chains, err := g.db.GetRedirectChains()
	if err != nil {
		return err
	}

	for _, chain := range chains {
		if chain.ChainLength > 1 {
			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"Source URL":   chain.SourceURL,
					"Chain Length": chain.ChainLength,
					"Final URL":    chain.FinalURL,
					"Chain":        chain.Chain,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateClientErrors(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	for _, url := range urls {
		fetch, err := g.db.GetLatestFetch(url.ID)
		if err != nil || fetch == nil {
			continue
		}

		if fetch.StatusCode >= 400 && fetch.StatusCode < 500 {
			// Find where this URL was linked from
			links, _ := g.db.GetLinksToURL(url.ID)
			foundOn := ""
			anchorText := ""
			if len(links) > 0 {
				foundOn = links[0].FromURL
				anchorText = links[0].AnchorText
			}

			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":         url.URL,
					"Status Code": fetch.StatusCode,
					"Found On":    foundOn,
					"Anchor Text": anchorText,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateServerErrors(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	for _, url := range urls {
		fetch, err := g.db.GetLatestFetch(url.ID)
		if err != nil || fetch == nil {
			continue
		}

		if fetch.StatusCode >= 500 {
			links, _ := g.db.GetLinksToURL(url.ID)
			foundOn := ""
			if len(links) > 0 {
				foundOn = links[0].FromURL
			}

			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":         url.URL,
					"Status Code": fetch.StatusCode,
					"Found On":    foundOn,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateCanonicalErrors(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	for _, url := range urls {
		features, err := g.db.GetHTMLFeatures(url.ID)
		if err != nil || features == nil {
			continue
		}

		var issueType, details string

		// Check for missing canonical
		if features.Canonical == "" {
			issueType = "Missing"
			details = "No canonical tag found"
		} else if features.Canonical != url.URL {
			// Check if canonical points to non-existent page
			canonicalURL, _ := g.db.GetURLByAddress(features.Canonical)
			if canonicalURL == nil {
				issueType = "Points to non-crawled URL"
				details = "Canonical URL was not crawled"
			} else {
				fetch, _ := g.db.GetLatestFetch(canonicalURL.ID)
				if fetch != nil && fetch.StatusCode >= 400 {
					issueType = "Points to error page"
					details = fmt.Sprintf("Canonical returns %d", fetch.StatusCode)
				}
			}
		}

		// Check for canonical chain
		if features.Canonical != "" && features.Canonical != url.URL {
			canonicalURL, _ := g.db.GetURLByAddress(features.Canonical)
			if canonicalURL != nil {
				canonicalFeatures, _ := g.db.GetHTMLFeatures(canonicalURL.ID)
				if canonicalFeatures != nil && canonicalFeatures.Canonical != "" && canonicalFeatures.Canonical != features.Canonical {
					issueType = "Canonical chain"
					details = fmt.Sprintf("Chains to: %s", canonicalFeatures.Canonical)
				}
			}
		}

		if issueType != "" {
			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":        url.URL,
					"Canonical":  features.Canonical,
					"Issue Type": issueType,
					"Details":    details,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateMissingTitles(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		features, err := g.db.GetHTMLFeatures(url.ID)
		if err != nil {
			continue
		}

		if features == nil || features.Title == "" {
			fetch, _ := g.db.GetLatestFetch(url.ID)
			statusCode := 0
			if fetch != nil {
				statusCode = fetch.StatusCode
			}

			indexability := "Indexable"
			if features != nil && strings.Contains(features.MetaRobots, "noindex") {
				indexability = "Non-indexable"
			}

			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":          url.URL,
					"Status Code":  statusCode,
					"Indexability": indexability,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateMissingMetaDesc(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		features, err := g.db.GetHTMLFeatures(url.ID)
		if err != nil {
			continue
		}

		if features == nil || features.MetaDescription == "" {
			fetch, _ := g.db.GetLatestFetch(url.ID)
			statusCode := 0
			title := ""
			if fetch != nil {
				statusCode = fetch.StatusCode
			}
			if features != nil {
				title = features.Title
			}

			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":         url.URL,
					"Status Code": statusCode,
					"Title":       title,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateMissingH1(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		features, err := g.db.GetHTMLFeatures(url.ID)
		if err != nil {
			continue
		}

		if features == nil || features.H1 == "" {
			fetch, _ := g.db.GetLatestFetch(url.ID)
			statusCode := 0
			title := ""
			if fetch != nil {
				statusCode = fetch.StatusCode
			}
			if features != nil {
				title = features.Title
			}

			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":         url.URL,
					"Status Code": statusCode,
					"Title":       title,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateDuplicateTitles(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	// Group by title
	titleURLs := make(map[string][]string)
	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		features, err := g.db.GetHTMLFeatures(url.ID)
		if err != nil || features == nil || features.Title == "" {
			continue
		}

		titleURLs[features.Title] = append(titleURLs[features.Title], url.URL)
	}

	// Find duplicates
	for title, urlList := range titleURLs {
		if len(urlList) > 1 {
			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"Title": title,
					"Count": len(urlList),
					"URLs":  strings.Join(urlList, "\n"),
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateDuplicateMetaDesc(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	// Group by meta description
	metaURLs := make(map[string][]string)
	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		features, err := g.db.GetHTMLFeatures(url.ID)
		if err != nil || features == nil || features.MetaDescription == "" {
			continue
		}

		metaURLs[features.MetaDescription] = append(metaURLs[features.MetaDescription], url.URL)
	}

	// Find duplicates
	for meta, urlList := range metaURLs {
		if len(urlList) > 1 {
			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"Meta Description": meta,
					"Count":            len(urlList),
					"URLs":             strings.Join(urlList, "\n"),
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateDuplicateContent(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	// Group by content hash
	hashURLs := make(map[string][]string)
	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		features, err := g.db.GetHTMLFeatures(url.ID)
		if err != nil || features == nil || features.ContentHash == "" {
			continue
		}

		hashURLs[features.ContentHash] = append(hashURLs[features.ContentHash], url.URL)
	}

	// Find duplicates
	for hash, urlList := range hashURLs {
		if len(urlList) > 1 {
			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"Content Hash": hash[:16] + "...", // Truncate for display
					"Count":        len(urlList),
					"URLs":         strings.Join(urlList, "\n"),
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateMissingAlt(report *Report) error {
	resources, err := g.db.GetAllResources()
	if err != nil {
		return err
	}

	// Group by image URL
	imagePages := make(map[string][]string)
	for _, res := range resources {
		if res.Type != "image" || res.AltText != "" {
			continue
		}
		if res.AltText == "" {
			pageURL, _ := g.db.GetURLByID(res.URLID)
			if pageURL != nil {
				imagePages[res.ResourceURL] = append(imagePages[res.ResourceURL], pageURL.URL)
			}
		}
	}

	for imageURL, pages := range imagePages {
		report.Rows = append(report.Rows, &ReportRow{
			Values: map[string]interface{}{
				"Image URL":   imageURL,
				"Found On":    strings.Join(pages, "\n"),
				"Occurrences": len(pages),
			},
		})
	}
	return nil
}

func (g *Generator) generateOrphanURLs(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	// Build inlinks map
	inlinksCount := make(map[int64]int)
	links, _ := g.db.GetAllLinks()
	for _, link := range links {
		if link.ToURLID != nil && link.IsInternal {
			inlinksCount[*link.ToURLID]++
		}
	}

	for _, url := range urls {
		if !url.IsInternal || url.Depth == 0 {
			continue
		}

		if inlinksCount[url.ID] == 0 {
			source := "Unknown"
			if url.FoundInSitemap {
				source = "Sitemap"
			}

			fetch, _ := g.db.GetLatestFetch(url.ID)
			statusCode := 0
			if fetch != nil {
				statusCode = fetch.StatusCode
			}

			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":         url.URL,
					"Source":      source,
					"Status Code": statusCode,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateNonIndexable(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		features, err := g.db.GetHTMLFeatures(url.ID)
		if err != nil || features == nil {
			continue
		}

		fetch, _ := g.db.GetLatestFetch(url.ID)
		xRobots := ""
		if fetch != nil && fetch.Headers != nil {
			if xr, ok := fetch.Headers["X-Robots-Tag"]; ok {
				xRobots = xr
			}
		}

		reason := ""
		if strings.Contains(strings.ToLower(features.MetaRobots), "noindex") {
			reason = "Meta robots noindex"
		}
		if strings.Contains(strings.ToLower(xRobots), "noindex") {
			if reason != "" {
				reason += ", "
			}
			reason += "X-Robots-Tag noindex"
		}

		if reason != "" {
			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":          url.URL,
					"Reason":       reason,
					"Meta Robots":  features.MetaRobots,
					"X-Robots-Tag": xRobots,
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateNoInternalInlinks(report *Report) error {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return err
	}

	// Build inlinks map
	internalInlinks := make(map[int64]int)
	externalInlinks := make(map[int64]int)
	links, _ := g.db.GetAllLinks()
	for _, link := range links {
		if link.ToURLID != nil {
			if link.IsInternal {
				internalInlinks[*link.ToURLID]++
			} else {
				externalInlinks[*link.ToURLID]++
			}
		}
	}

	for _, url := range urls {
		if !url.IsInternal || url.Depth == 0 {
			continue
		}

		if internalInlinks[url.ID] == 0 {
			fetch, _ := g.db.GetLatestFetch(url.ID)
			statusCode := 0
			if fetch != nil {
				statusCode = fetch.StatusCode
			}

			report.Rows = append(report.Rows, &ReportRow{
				Values: map[string]interface{}{
					"URL":              url.URL,
					"Status Code":      statusCode,
					"External Inlinks": externalInlinks[url.ID],
				},
			})
		}
	}
	return nil
}

func (g *Generator) generateBrokenLinks(report *Report) error {
	links, err := g.db.GetAllLinks()
	if err != nil {
		return err
	}

	for _, link := range links {
		if link.ToURLID == nil {
			continue
		}

		targetURL, _ := g.db.GetURLByID(*link.ToURLID)
		if targetURL == nil {
			continue
		}

		fetch, _ := g.db.GetLatestFetch(*link.ToURLID)
		if fetch == nil || fetch.StatusCode < 400 {
			continue
		}

		fromURL, _ := g.db.GetURLByID(link.FromURLID)
		foundOn := ""
		if fromURL != nil {
			foundOn = fromURL.URL
		}

		report.Rows = append(report.Rows, &ReportRow{
			Values: map[string]interface{}{
				"Link URL":    link.ToURL,
				"Status Code": fetch.StatusCode,
				"Found On":    foundOn,
				"Anchor Text": link.AnchorText,
			},
		})
	}
	return nil
}

func (g *Generator) generateAllIssues(report *Report) error {
	issues, err := g.db.GetAllIssues()
	if err != nil {
		return err
	}

	for _, issue := range issues {
		url, _ := g.db.GetURLByID(issue.URLID)
		urlStr := ""
		if url != nil {
			urlStr = url.URL
		}

		report.Rows = append(report.Rows, &ReportRow{
			Values: map[string]interface{}{
				"URL":        urlStr,
				"Issue Type": issue.IssueCode,
				"Severity":   issue.Severity,
				"Category":   issue.Category,
				"Message":    issue.Message,
			},
		})
	}
	return nil
}

func (g *Generator) generateSEOOverview(report *Report) error {
	urls, _ := g.db.GetAllURLs()
	issues, _ := g.db.GetAllIssues()

	// Count metrics
	totalURLs := len(urls)
	internalURLs := 0
	indexableURLs := 0
	missingTitles := 0
	missingMeta := 0
	missingH1 := 0

	for _, url := range urls {
		if url.IsInternal {
			internalURLs++
		}

		features, _ := g.db.GetHTMLFeatures(url.ID)
		if features != nil {
			if !strings.Contains(strings.ToLower(features.MetaRobots), "noindex") {
				indexableURLs++
			}
			if features.Title == "" {
				missingTitles++
			}
			if features.MetaDescription == "" {
				missingMeta++
			}
			if features.H1 == "" {
				missingH1++
			}
		}
	}

	criticalIssues := 0
	highIssues := 0
	for _, issue := range issues {
		if issue.Severity == storage.SeverityCritical {
			criticalIssues++
		}
		if issue.Severity == storage.SeverityHigh {
			highIssues++
		}
	}

	metrics := []struct {
		name   string
		value  interface{}
		status string
	}{
		{"Total URLs Crawled", totalURLs, "Info"},
		{"Internal URLs", internalURLs, "Info"},
		{"Indexable URLs", indexableURLs, "Info"},
		{"Missing Titles", missingTitles, statusForCount(missingTitles)},
		{"Missing Meta Descriptions", missingMeta, statusForCount(missingMeta)},
		{"Missing H1", missingH1, statusForCount(missingH1)},
		{"Critical Issues", criticalIssues, statusForCount(criticalIssues)},
		{"High Priority Issues", highIssues, statusForCount(highIssues)},
		{"Total Issues", len(issues), "Info"},
	}

	for _, m := range metrics {
		report.Rows = append(report.Rows, &ReportRow{
			Values: map[string]interface{}{
				"Metric": m.name,
				"Value":  m.value,
				"Status": m.status,
			},
		})
	}
	return nil
}

func (g *Generator) generateCrawlSummary(report *Report) error {
	urls, _ := g.db.GetAllURLs()
	links, _ := g.db.GetAllLinks()
	resources, _ := g.db.GetAllResources()
	issues, _ := g.db.GetAllIssues()

	// Status code breakdown
	status2xx := 0
	status3xx := 0
	status4xx := 0
	status5xx := 0

	for _, url := range urls {
		fetch, _ := g.db.GetLatestFetch(url.ID)
		if fetch == nil {
			continue
		}
		switch {
		case fetch.StatusCode >= 200 && fetch.StatusCode < 300:
			status2xx++
		case fetch.StatusCode >= 300 && fetch.StatusCode < 400:
			status3xx++
		case fetch.StatusCode >= 400 && fetch.StatusCode < 500:
			status4xx++
		case fetch.StatusCode >= 500:
			status5xx++
		}
	}

	internalLinks := 0
	externalLinks := 0
	for _, link := range links {
		if link.IsInternal {
			internalLinks++
		} else {
			externalLinks++
		}
	}

	metrics := []struct {
		name  string
		value interface{}
	}{
		{"Total URLs", len(urls)},
		{"2xx Responses", status2xx},
		{"3xx Redirects", status3xx},
		{"4xx Client Errors", status4xx},
		{"5xx Server Errors", status5xx},
		{"Internal Links", internalLinks},
		{"External Links", externalLinks},
		{"Total Resources", len(resources)},
		{"Total Issues", len(issues)},
	}

	for _, m := range metrics {
		report.Rows = append(report.Rows, &ReportRow{
			Values: map[string]interface{}{
				"Metric": m.name,
				"Value":  m.value,
			},
		})
	}
	return nil
}

func statusForCount(count int) string {
	if count == 0 {
		return "Good"
	}
	if count <= 5 {
		return "Warning"
	}
	return "Error"
}

// SortReport sorts report rows by a column.
func (r *Report) SortReport(column string, ascending bool) {
	sort.Slice(r.Rows, func(i, j int) bool {
		vi := r.Rows[i].Values[column]
		vj := r.Rows[j].Values[column]

		// Compare based on type
		switch v := vi.(type) {
		case int:
			vji := vj.(int)
			if ascending {
				return v < vji
			}
			return v > vji
		case string:
			vjs := vj.(string)
			if ascending {
				return v < vjs
			}
			return v > vjs
		}

		return false
	})
}

// FilterReport filters report rows.
func (r *Report) FilterReport(column string, value interface{}) *Report {
	filtered := &Report{
		Definition: r.Definition,
		Rows:       make([]*ReportRow, 0),
		Generated:  r.Generated,
	}

	for _, row := range r.Rows {
		if row.Values[column] == value {
			filtered.Rows = append(filtered.Rows, row)
		}
	}

	filtered.TotalCount = len(filtered.Rows)
	return filtered
}
