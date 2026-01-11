package analyzer

import (
	"crypto/md5"
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// PageTitlesAnalyzer analyzes page titles.
type PageTitlesAnalyzer struct {
	// For duplicate detection across pages
	titleHashes map[string][]int64 // hash -> list of URL IDs
}

func NewPageTitlesAnalyzer() *PageTitlesAnalyzer {
	return &PageTitlesAnalyzer{
		titleHashes: make(map[string][]int64),
	}
}

func (a *PageTitlesAnalyzer) Name() string {
	return "Page Titles"
}

func (a *PageTitlesAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "title", Title: "Title", Width: 300, Sortable: true, DataKey: "title"},
		{ID: "length", Title: "Length", Width: 70, Sortable: true, DataKey: "length"},
		{ID: "pixel_width", Title: "Pixel Width", Width: 90, Sortable: true, DataKey: "pixel_width"},
		{ID: "status", Title: "Status", Width: 100, Sortable: true, DataKey: "status"},
		{ID: "occurrences", Title: "Occurrences", Width: 90, Sortable: true, DataKey: "occurrences"},
	}
}

func (a *PageTitlesAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "missing", Label: "Missing", Description: "Pages without title", FilterFunc: func(r *AnalysisResult) bool {
			title, _ := r.Data["title"].(string)
			return title == ""
		}},
		{ID: "duplicate", Label: "Duplicate", Description: "Duplicate titles", FilterFunc: func(r *AnalysisResult) bool {
			if occ, ok := r.Data["occurrences"].(int); ok {
				return occ > 1
			}
			return false
		}},
		{ID: "too_long", Label: "Over 60 Characters", Description: "Title exceeds 60 characters", FilterFunc: func(r *AnalysisResult) bool {
			if length, ok := r.Data["length"].(int); ok {
				return length > Thresholds.TitleMaxLength
			}
			return false
		}},
		{ID: "too_short", Label: "Below 30 Characters", Description: "Title under 30 characters", FilterFunc: func(r *AnalysisResult) bool {
			if length, ok := r.Data["length"].(int); ok {
				return length > 0 && length < Thresholds.TitleMinLength
			}
			return false
		}},
		{ID: "same_as_h1", Label: "Same as H1", Description: "Title matches H1", FilterFunc: func(r *AnalysisResult) bool {
			if same, ok := r.Data["same_as_h1"].(bool); ok {
				return same
			}
			return false
		}},
		{ID: "pixel_truncated", Label: "Pixel Truncated", Description: "Title may be truncated in SERPs", FilterFunc: func(r *AnalysisResult) bool {
			if width, ok := r.Data["pixel_width"].(int); ok {
				return width > Thresholds.TitleMaxPixels
			}
			return false
		}},
	}
}

func (a *PageTitlesAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.HTMLFeatures == nil {
		result.Data["title"] = ""
		result.Data["length"] = 0
		result.Data["status"] = "No HTML"
		return result
	}

	title := ctx.HTMLFeatures.Title
	titleLength := len(title)

	result.Data["title"] = title
	result.Data["length"] = titleLength
	result.Data["pixel_width"] = a.estimatePixelWidth(title)

	// Track for duplicate detection
	if title != "" {
		hash := a.hashTitle(title)
		a.titleHashes[hash] = append(a.titleHashes[hash], ctx.URL.ID)
		result.Data["title_hash"] = hash
		result.Data["occurrences"] = len(a.titleHashes[hash])
	} else {
		result.Data["occurrences"] = 0
	}

	// Check if same as H1
	if ctx.HTMLFeatures.H1First != "" && title == ctx.HTMLFeatures.H1First {
		result.Data["same_as_h1"] = true
	} else {
		result.Data["same_as_h1"] = false
	}

	// Determine status and generate issues
	if title == "" {
		result.Data["status"] = "Missing"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueMissingTitle,
			storage.IssueTypeError,
			storage.SeverityHigh,
			"title",
			"Page is missing a title tag",
		))
	} else if titleLength > Thresholds.TitleMaxLength {
		result.Data["status"] = "Too Long"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueTitleTooLong,
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"title",
			fmt.Sprintf("Title is too long (%d characters, recommended max: %d)", titleLength, Thresholds.TitleMaxLength),
		))
	} else if titleLength < Thresholds.TitleMinLength {
		result.Data["status"] = "Too Short"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueTitleTooShort,
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"title",
			fmt.Sprintf("Title is too short (%d characters, recommended min: %d)", titleLength, Thresholds.TitleMinLength),
		))
	} else {
		result.Data["status"] = "OK"
	}

	return result
}

// AnalyzeDuplicates should be called after all pages are analyzed to detect duplicates.
func (a *PageTitlesAnalyzer) AnalyzeDuplicates() []*storage.Issue {
	issues := make([]*storage.Issue, 0)

	for hash, urlIDs := range a.titleHashes {
		if len(urlIDs) > 1 {
			for _, urlID := range urlIDs {
				issues = append(issues, NewIssue(
					urlID,
					storage.IssueDuplicateTitle,
					storage.IssueTypeWarning,
					storage.SeverityMedium,
					"title",
					fmt.Sprintf("Duplicate title found on %d pages (hash: %s)", len(urlIDs), hash[:8]),
				))
			}
		}
	}

	return issues
}

// GetDuplicateGroups returns groups of URLs with duplicate titles.
func (a *PageTitlesAnalyzer) GetDuplicateGroups() map[string][]int64 {
	duplicates := make(map[string][]int64)
	for hash, urlIDs := range a.titleHashes {
		if len(urlIDs) > 1 {
			duplicates[hash] = urlIDs
		}
	}
	return duplicates
}

// Reset clears the duplicate tracking data.
func (a *PageTitlesAnalyzer) Reset() {
	a.titleHashes = make(map[string][]int64)
}

func (a *PageTitlesAnalyzer) hashTitle(title string) string {
	hash := md5.Sum([]byte(title))
	return fmt.Sprintf("%x", hash)
}

// estimatePixelWidth estimates the pixel width of a title.
// This is a rough estimation - actual width depends on font.
func (a *PageTitlesAnalyzer) estimatePixelWidth(title string) int {
	// Average character width in pixels for common serif fonts
	// This is approximate - Google uses variable width fonts
	avgCharWidth := 8.5
	return int(float64(len(title)) * avgCharWidth)
}

// ExportRow returns a row for CSV/Excel export.
func (a *PageTitlesAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["title"]),
		fmt.Sprintf("%v", result.Data["length"]),
		fmt.Sprintf("%v", result.Data["pixel_width"]),
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["occurrences"]),
	}
}
