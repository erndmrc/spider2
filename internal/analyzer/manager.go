package analyzer

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/spider-crawler/spider/internal/storage"
)

// Manager coordinates all analyzers.
type Manager struct {
	mu sync.RWMutex

	// Analyzers
	ResponseCodes    *ResponseCodesAnalyzer
	PageTitles       *PageTitlesAnalyzer
	MetaDescription  *MetaDescriptionAnalyzer
	H1               *H1Analyzer
	H2               *H2Analyzer
	Content          *ContentAnalyzer
	Images           *ImagesAnalyzer
	Canonicals       *CanonicalsAnalyzer
	Directives       *DirectivesAnalyzer
	Links            *LinksAnalyzer
	Hreflang         *HreflangAnalyzer
	URLHealth        *URLHealthAnalyzer
	JavaScript       *JavaScriptAnalyzer
	AMP              *AMPAnalyzer
	StructuredData   *StructuredDataAnalyzer
	Sitemaps         *SitemapsAnalyzer
	PageSpeed        *PageSpeedAnalyzer
	Mobile           *MobileAnalyzer
	Accessibility    *AccessibilityAnalyzer
	CustomSearch     *CustomSearchAnalyzer
	CustomExtraction *CustomExtractionAnalyzer

	// Results storage
	Results map[string][]*AnalysisResult // analyzer name -> results

	// Issues collected
	AllIssues []*storage.Issue
}

// NewManager creates a new analyzer manager.
func NewManager() *Manager {
	return &Manager{
		ResponseCodes:    NewResponseCodesAnalyzer(),
		PageTitles:       NewPageTitlesAnalyzer(),
		MetaDescription:  NewMetaDescriptionAnalyzer(),
		H1:               NewH1Analyzer(),
		H2:               NewH2Analyzer(),
		Content:          NewContentAnalyzer(),
		Images:           NewImagesAnalyzer(),
		Canonicals:       NewCanonicalsAnalyzer(),
		Directives:       NewDirectivesAnalyzer(),
		Links:            NewLinksAnalyzer(),
		Hreflang:         NewHreflangAnalyzer(),
		URLHealth:        NewURLHealthAnalyzer(),
		JavaScript:       NewJavaScriptAnalyzer(),
		AMP:              NewAMPAnalyzer(),
		StructuredData:   NewStructuredDataAnalyzer(),
		Sitemaps:         NewSitemapsAnalyzer(),
		PageSpeed:        NewPageSpeedAnalyzer(),
		Mobile:           NewMobileAnalyzer(),
		Accessibility:    NewAccessibilityAnalyzer(),
		CustomSearch:     NewCustomSearchAnalyzer(),
		CustomExtraction: NewCustomExtractionAnalyzer(),
		Results:          make(map[string][]*AnalysisResult),
		AllIssues:        make([]*storage.Issue, 0),
	}
}

// AnalyzePage runs all analyzers on a single page.
func (m *Manager) AnalyzePage(ctx *AnalysisContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Response Codes
	rcResult := m.ResponseCodes.Analyze(ctx)
	m.Results["response_codes"] = append(m.Results["response_codes"], rcResult)
	m.AllIssues = append(m.AllIssues, rcResult.Issues...)

	// Page Titles
	ptResult := m.PageTitles.Analyze(ctx)
	m.Results["page_titles"] = append(m.Results["page_titles"], ptResult)
	m.AllIssues = append(m.AllIssues, ptResult.Issues...)

	// Meta Description
	mdResult := m.MetaDescription.Analyze(ctx)
	m.Results["meta_description"] = append(m.Results["meta_description"], mdResult)
	m.AllIssues = append(m.AllIssues, mdResult.Issues...)

	// H1
	h1Result := m.H1.Analyze(ctx)
	m.Results["h1"] = append(m.Results["h1"], h1Result)
	m.AllIssues = append(m.AllIssues, h1Result.Issues...)

	// H2
	h2Result := m.H2.Analyze(ctx)
	m.Results["h2"] = append(m.Results["h2"], h2Result)
	m.AllIssues = append(m.AllIssues, h2Result.Issues...)

	// Content
	contentResult := m.Content.Analyze(ctx)
	m.Results["content"] = append(m.Results["content"], contentResult)
	m.AllIssues = append(m.AllIssues, contentResult.Issues...)

	// Canonicals
	canResult := m.Canonicals.Analyze(ctx)
	m.Results["canonicals"] = append(m.Results["canonicals"], canResult)
	m.AllIssues = append(m.AllIssues, canResult.Issues...)

	// Directives
	dirResult := m.Directives.Analyze(ctx)
	m.Results["directives"] = append(m.Results["directives"], dirResult)
	m.AllIssues = append(m.AllIssues, dirResult.Issues...)

	// Links (page-level summary)
	linksResult := m.Links.Analyze(ctx)
	m.Results["links_summary"] = append(m.Results["links_summary"], linksResult)
	m.AllIssues = append(m.AllIssues, linksResult.Issues...)

	// Hreflang
	hreflangResult := m.Hreflang.Analyze(ctx)
	m.Results["hreflang"] = append(m.Results["hreflang"], hreflangResult)
	m.AllIssues = append(m.AllIssues, hreflangResult.Issues...)

	// URL Health
	urlResult := m.URLHealth.Analyze(ctx)
	m.Results["url"] = append(m.Results["url"], urlResult)
	m.AllIssues = append(m.AllIssues, urlResult.Issues...)

	// JavaScript
	jsResult := m.JavaScript.Analyze(ctx)
	m.Results["javascript"] = append(m.Results["javascript"], jsResult)
	m.AllIssues = append(m.AllIssues, jsResult.Issues...)

	// AMP
	ampResult := m.AMP.Analyze(ctx)
	m.Results["amp"] = append(m.Results["amp"], ampResult)
	m.AllIssues = append(m.AllIssues, ampResult.Issues...)

	// Structured Data
	sdResult := m.StructuredData.Analyze(ctx)
	m.Results["structured_data"] = append(m.Results["structured_data"], sdResult)
	m.AllIssues = append(m.AllIssues, sdResult.Issues...)

	// Mobile
	mobileResult := m.Mobile.Analyze(ctx)
	m.Results["mobile"] = append(m.Results["mobile"], mobileResult)
	m.AllIssues = append(m.AllIssues, mobileResult.Issues...)

	// Accessibility
	a11yResult := m.Accessibility.Analyze(ctx)
	m.Results["accessibility"] = append(m.Results["accessibility"], a11yResult)
	m.AllIssues = append(m.AllIssues, a11yResult.Issues...)

	// Custom Search (if rules configured)
	if len(m.CustomSearch.GetRules()) > 0 {
		csResult := m.CustomSearch.Analyze(ctx)
		m.Results["custom_search"] = append(m.Results["custom_search"], csResult)
	}

	// Custom Extraction (if rules configured)
	if len(m.CustomExtraction.GetRules()) > 0 {
		ceResult := m.CustomExtraction.Analyze(ctx)
		m.Results["custom_extraction"] = append(m.Results["custom_extraction"], ceResult)
	}
}

// AnalyzeImages analyzes images for a page.
func (m *Manager) AnalyzeImages(resources []*storage.Resource, pageURL string, pageURLID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := m.Images.AnalyzePageImages(resources, pageURL, pageURLID)
	m.Results["images"] = append(m.Results["images"], results...)

	for _, r := range results {
		m.AllIssues = append(m.AllIssues, r.Issues...)
	}
}

// AnalyzeLink analyzes a single link.
func (m *Manager) AnalyzeLink(link *storage.Link, fromURL string, targetStatus int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := m.Links.AnalyzeLink(link, fromURL, targetStatus)
	m.Results["links"] = append(m.Results["links"], result)
	m.AllIssues = append(m.AllIssues, result.Issues...)
}

// FinalizeDuplicateAnalysis runs duplicate detection after all pages are analyzed.
func (m *Manager) FinalizeDuplicateAnalysis() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Title duplicates
	m.AllIssues = append(m.AllIssues, m.PageTitles.AnalyzeDuplicates()...)

	// Meta description duplicates
	m.AllIssues = append(m.AllIssues, m.MetaDescription.AnalyzeDuplicates()...)

	// H1 duplicates
	m.AllIssues = append(m.AllIssues, m.H1.AnalyzeDuplicates()...)

	// Content duplicates
	m.AllIssues = append(m.AllIssues, m.Content.AnalyzeDuplicates()...)

	// URL duplicates
	m.AllIssues = append(m.AllIssues, m.URLHealth.AnalyzeDuplicates()...)

	// Hreflang return links
	m.AllIssues = append(m.AllIssues, m.Hreflang.AnalyzeReturnLinks()...)
}

// GetResults returns results for a specific analyzer.
func (m *Manager) GetResults(analyzerName string) []*AnalysisResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Results[analyzerName]
}

// GetFilteredResults returns filtered results.
func (m *Manager) GetFilteredResults(analyzerName string, filterID string) []*AnalysisResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := m.Results[analyzerName]
	if filterID == "all" || filterID == "" {
		return results
	}

	// Get filter function based on analyzer
	var filterFunc func(*AnalysisResult) bool

	switch analyzerName {
	case "response_codes":
		for _, f := range m.ResponseCodes.Filters() {
			if f.ID == filterID && f.FilterFunc != nil {
				filterFunc = f.FilterFunc
				break
			}
		}
	case "page_titles":
		for _, f := range m.PageTitles.Filters() {
			if f.ID == filterID && f.FilterFunc != nil {
				filterFunc = f.FilterFunc
				break
			}
		}
	// Add other analyzers...
	}

	if filterFunc == nil {
		return results
	}

	filtered := make([]*AnalysisResult, 0)
	for _, r := range results {
		if filterFunc(r) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// GetAllIssues returns all collected issues.
func (m *Manager) GetAllIssues() []*storage.Issue {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.AllIssues
}

// GetIssuesByCategory returns issues grouped by category.
func (m *Manager) GetIssuesByCategory() map[string][]*storage.Issue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	grouped := make(map[string][]*storage.Issue)
	for _, issue := range m.AllIssues {
		grouped[issue.Category] = append(grouped[issue.Category], issue)
	}
	return grouped
}

// GetIssuesBySeverity returns issues grouped by severity.
func (m *Manager) GetIssuesBySeverity() map[string][]*storage.Issue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	grouped := make(map[string][]*storage.Issue)
	for _, issue := range m.AllIssues {
		grouped[issue.Severity] = append(grouped[issue.Severity], issue)
	}
	return grouped
}

// Reset clears all results and resets analyzers.
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Results = make(map[string][]*AnalysisResult)
	m.AllIssues = make([]*storage.Issue, 0)

	m.PageTitles.Reset()
	m.MetaDescription.Reset()
	m.H1.Reset()
	m.Content.Reset()
	m.Canonicals.Reset()
	m.Hreflang.Reset()
	m.URLHealth.Reset()
	m.Sitemaps.Reset()
	m.CustomSearch.ClearRules()
	m.CustomExtraction.ClearRules()
}

// ExportCSV exports results to CSV.
func (m *Manager) ExportCSV(analyzerName, filePath string) error {
	m.mu.RLock()
	results := m.Results[analyzerName]
	m.mu.RUnlock()

	if len(results) == 0 {
		return fmt.Errorf("no results for analyzer: %s", analyzerName)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header based on analyzer
	var headers []string
	var exportFunc func(*AnalysisResult) []string

	switch analyzerName {
	case "response_codes":
		for _, col := range m.ResponseCodes.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.ResponseCodes.ExportRow
	case "page_titles":
		for _, col := range m.PageTitles.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.PageTitles.ExportRow
	case "meta_description":
		for _, col := range m.MetaDescription.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.MetaDescription.ExportRow
	case "h1":
		for _, col := range m.H1.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.H1.ExportRow
	case "content":
		for _, col := range m.Content.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.Content.ExportRow
	case "images":
		for _, col := range m.Images.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.Images.ExportRow
	case "canonicals":
		for _, col := range m.Canonicals.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.Canonicals.ExportRow
	case "directives":
		for _, col := range m.Directives.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.Directives.ExportRow
	case "links":
		for _, col := range m.Links.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.Links.ExportRow
	case "hreflang":
		for _, col := range m.Hreflang.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.Hreflang.ExportRow
	case "url":
		for _, col := range m.URLHealth.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.URLHealth.ExportRow
	case "javascript":
		for _, col := range m.JavaScript.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.JavaScript.ExportRow
	case "amp":
		for _, col := range m.AMP.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.AMP.ExportRow
	case "structured_data":
		for _, col := range m.StructuredData.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.StructuredData.ExportRow
	case "mobile":
		for _, col := range m.Mobile.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.Mobile.ExportRow
	case "accessibility":
		for _, col := range m.Accessibility.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.Accessibility.ExportRow
	case "custom_search":
		for _, col := range m.CustomSearch.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.CustomSearch.ExportRow
	case "custom_extraction":
		for _, col := range m.CustomExtraction.Columns() {
			headers = append(headers, col.Title)
		}
		exportFunc = m.CustomExtraction.ExportRow
	default:
		return fmt.Errorf("unknown analyzer: %s", analyzerName)
	}

	writer.Write(headers)

	for _, result := range results {
		writer.Write(exportFunc(result))
	}

	return nil
}

// ExportJSON exports results to JSON.
func (m *Manager) ExportJSON(analyzerName, filePath string) error {
	m.mu.RLock()
	results := m.Results[analyzerName]
	m.mu.RUnlock()

	if len(results) == 0 {
		return fmt.Errorf("no results for analyzer: %s", analyzerName)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	// Export just the Data maps
	exportData := make([]map[string]interface{}, len(results))
	for i, r := range results {
		exportData[i] = r.Data
	}

	return encoder.Encode(exportData)
}

// ExportAllIssuesCSV exports all issues to CSV.
func (m *Manager) ExportAllIssuesCSV(filePath string) error {
	m.mu.RLock()
	issues := m.AllIssues
	m.mu.RUnlock()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header
	writer.Write([]string{"URL ID", "Issue Code", "Type", "Severity", "Category", "Message"})

	for _, issue := range issues {
		writer.Write([]string{
			fmt.Sprintf("%d", issue.URLID),
			issue.IssueCode,
			issue.IssueType,
			issue.Severity,
			issue.Category,
			issue.Message,
		})
	}

	return nil
}

// Summary returns a summary of analysis results.
type Summary struct {
	TotalURLs          int
	TotalIssues        int
	CriticalIssues     int
	HighIssues         int
	MediumIssues       int
	LowIssues          int
	IssuesByCategory   map[string]int
	MissingTitles      int
	MissingMetaDesc    int
	MissingH1          int
	DuplicateTitles    int
	DuplicateMetaDesc  int
	DuplicateContent   int
	BrokenLinks        int
	ImagesWithoutAlt   int
}

// GetSummary returns analysis summary.
func (m *Manager) GetSummary() *Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &Summary{
		IssuesByCategory: make(map[string]int),
	}

	// Count URLs
	if results, ok := m.Results["url"]; ok {
		summary.TotalURLs = len(results)
	}

	// Count issues by severity
	for _, issue := range m.AllIssues {
		summary.TotalIssues++
		switch issue.Severity {
		case storage.SeverityCritical:
			summary.CriticalIssues++
		case storage.SeverityHigh:
			summary.HighIssues++
		case storage.SeverityMedium:
			summary.MediumIssues++
		case storage.SeverityLow:
			summary.LowIssues++
		}
		summary.IssuesByCategory[issue.Category]++

		// Specific counts
		switch issue.IssueCode {
		case storage.IssueMissingTitle:
			summary.MissingTitles++
		case storage.IssueMissingMetaDesc:
			summary.MissingMetaDesc++
		case storage.IssueMissingH1:
			summary.MissingH1++
		case storage.IssueDuplicateTitle:
			summary.DuplicateTitles++
		case storage.IssueDuplicateMetaDesc:
			summary.DuplicateMetaDesc++
		case storage.IssueDuplicateContent:
			summary.DuplicateContent++
		case storage.IssueBrokenLink:
			summary.BrokenLinks++
		case storage.IssueMissingAlt:
			summary.ImagesWithoutAlt++
		}
	}

	return summary
}
