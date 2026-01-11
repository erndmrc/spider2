// Package analyzer provides SEO analysis modules.
package analyzer

import (
	"github.com/spider-crawler/spider/internal/storage"
)

// AnalysisResult represents the result of analyzing a page.
type AnalysisResult struct {
	URLID  int64
	Issues []*storage.Issue
	Data   map[string]interface{}
}

// Analyzer is the interface for all analysis modules.
type Analyzer interface {
	// Name returns the analyzer name
	Name() string

	// Analyze performs analysis on the given data
	Analyze(ctx *AnalysisContext) *AnalysisResult

	// Columns returns the column definitions for this analyzer's tab
	Columns() []ColumnDef

	// Filters returns available filters for this analyzer
	Filters() []FilterDef
}

// AnalysisContext contains all data needed for analysis.
type AnalysisContext struct {
	URL          *storage.URL
	Fetch        *storage.Fetch
	HTMLFeatures *storage.HTMLFeatures
	Links        []*storage.Link
	Resources    []*storage.Resource
	RawHTML      []byte
	AllURLs      map[string]*storage.URL // For cross-page analysis (duplicates)
}

// ColumnDef defines a column for export/display.
type ColumnDef struct {
	ID       string
	Title    string
	Width    int
	Sortable bool
	DataKey  string // Key to extract data from result
}

// FilterDef defines a filter option.
type FilterDef struct {
	ID          string
	Label       string
	Description string
	FilterFunc  func(result *AnalysisResult) bool
}

// Thresholds for SEO analysis
var Thresholds = struct {
	TitleMinLength       int
	TitleMaxLength       int
	TitleMaxPixels       int
	MetaDescMinLength    int
	MetaDescMaxLength    int
	MetaDescMaxPixels    int
	H1MaxLength          int
	URLMaxLength         int
	ThinContentWordCount int
	LargeImageSize       int64
	SlowResponseTime     int64 // milliseconds
	MaxRedirectChain     int
}{
	TitleMinLength:       30,
	TitleMaxLength:       60,
	TitleMaxPixels:       580,
	MetaDescMinLength:    70,
	MetaDescMaxLength:    155,
	MetaDescMaxPixels:    920,
	H1MaxLength:          70,
	URLMaxLength:         115,
	ThinContentWordCount: 200,
	LargeImageSize:       100 * 1024, // 100KB
	SlowResponseTime:     500,        // 500ms
	MaxRedirectChain:     2,
}

// Helper function to create an issue
func NewIssue(urlID int64, code, issueType, severity, category, message string) *storage.Issue {
	return &storage.Issue{
		URLID:     urlID,
		IssueCode: code,
		IssueType: issueType,
		Severity:  severity,
		Category:  category,
		Message:   message,
	}
}
