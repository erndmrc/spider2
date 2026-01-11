package analyzer

import (
	"crypto/md5"
	"encoding/json"
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// H1Analyzer analyzes H1 headings.
type H1Analyzer struct {
	h1Hashes map[string][]int64
}

func NewH1Analyzer() *H1Analyzer {
	return &H1Analyzer{
		h1Hashes: make(map[string][]int64),
	}
}

func (a *H1Analyzer) Name() string {
	return "H1"
}

func (a *H1Analyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "h1", Title: "H1", Width: 300, Sortable: true, DataKey: "h1"},
		{ID: "length", Title: "Length", Width: 70, Sortable: true, DataKey: "length"},
		{ID: "count", Title: "H1 Count", Width: 80, Sortable: true, DataKey: "count"},
		{ID: "status", Title: "Status", Width: 100, Sortable: true, DataKey: "status"},
		{ID: "occurrences", Title: "Occurrences", Width: 90, Sortable: true, DataKey: "occurrences"},
	}
}

func (a *H1Analyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "missing", Label: "Missing", Description: "Pages without H1", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["count"].(int); ok {
				return count == 0
			}
			return true
		}},
		{ID: "duplicate", Label: "Duplicate", Description: "Duplicate H1s across pages", FilterFunc: func(r *AnalysisResult) bool {
			if occ, ok := r.Data["occurrences"].(int); ok {
				return occ > 1
			}
			return false
		}},
		{ID: "multiple", Label: "Multiple", Description: "Pages with more than one H1", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["count"].(int); ok {
				return count > 1
			}
			return false
		}},
		{ID: "too_long", Label: "Over 70 Characters", Description: "H1 exceeds 70 characters", FilterFunc: func(r *AnalysisResult) bool {
			if length, ok := r.Data["length"].(int); ok {
				return length > Thresholds.H1MaxLength
			}
			return false
		}},
	}
}

func (a *H1Analyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.HTMLFeatures == nil {
		result.Data["h1"] = ""
		result.Data["length"] = 0
		result.Data["count"] = 0
		result.Data["status"] = "No HTML"
		return result
	}

	h1First := ctx.HTMLFeatures.H1First
	h1Count := ctx.HTMLFeatures.H1Count
	h1Length := len(h1First)

	result.Data["h1"] = h1First
	result.Data["length"] = h1Length
	result.Data["count"] = h1Count

	// Parse all H1s if available
	if ctx.HTMLFeatures.H1All != "" {
		var allH1s []string
		if err := json.Unmarshal([]byte(ctx.HTMLFeatures.H1All), &allH1s); err == nil {
			result.Data["h1_all"] = allH1s
		}
	}

	// Track for duplicate detection
	if h1First != "" {
		hash := a.hashH1(h1First)
		a.h1Hashes[hash] = append(a.h1Hashes[hash], ctx.URL.ID)
		result.Data["h1_hash"] = hash
		result.Data["occurrences"] = len(a.h1Hashes[hash])
	} else {
		result.Data["occurrences"] = 0
	}

	// Determine status and generate issues
	if h1Count == 0 {
		result.Data["status"] = "Missing"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueMissingH1,
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"headings",
			"Page is missing an H1 heading",
		))
	} else if h1Count > 1 {
		result.Data["status"] = "Multiple"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueMultipleH1,
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"headings",
			fmt.Sprintf("Page has multiple H1 headings (%d)", h1Count),
		))
	} else if h1Length > Thresholds.H1MaxLength {
		result.Data["status"] = "Too Long"
	} else {
		result.Data["status"] = "OK"
	}

	return result
}

func (a *H1Analyzer) AnalyzeDuplicates() []*storage.Issue {
	issues := make([]*storage.Issue, 0)
	for hash, urlIDs := range a.h1Hashes {
		if len(urlIDs) > 1 {
			for _, urlID := range urlIDs {
				issues = append(issues, NewIssue(
					urlID,
					storage.IssueDuplicateH1,
					storage.IssueTypeWarning,
					storage.SeverityLow,
					"headings",
					fmt.Sprintf("Duplicate H1 found on %d pages (hash: %s)", len(urlIDs), hash[:8]),
				))
			}
		}
	}
	return issues
}

func (a *H1Analyzer) Reset() {
	a.h1Hashes = make(map[string][]int64)
}

func (a *H1Analyzer) hashH1(h1 string) string {
	hash := md5.Sum([]byte(h1))
	return fmt.Sprintf("%x", hash)
}

func (a *H1Analyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["h1"]),
		fmt.Sprintf("%v", result.Data["length"]),
		fmt.Sprintf("%v", result.Data["count"]),
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["occurrences"]),
	}
}

// H2Analyzer analyzes H2 headings.
type H2Analyzer struct{}

func NewH2Analyzer() *H2Analyzer {
	return &H2Analyzer{}
}

func (a *H2Analyzer) Name() string {
	return "H2"
}

func (a *H2Analyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "h2", Title: "H2", Width: 300, Sortable: true, DataKey: "h2"},
		{ID: "count", Title: "H2 Count", Width: 80, Sortable: true, DataKey: "count"},
		{ID: "status", Title: "Status", Width: 100, Sortable: true, DataKey: "status"},
	}
}

func (a *H2Analyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "missing", Label: "Missing", Description: "Pages without H2", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["count"].(int); ok {
				return count == 0
			}
			return true
		}},
		{ID: "multiple", Label: "Multiple", Description: "Pages with H2s", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["count"].(int); ok {
				return count > 0
			}
			return false
		}},
	}
}

func (a *H2Analyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.HTMLFeatures == nil {
		result.Data["h2"] = ""
		result.Data["count"] = 0
		result.Data["status"] = "No HTML"
		return result
	}

	h2Count := ctx.HTMLFeatures.H2Count
	result.Data["count"] = h2Count

	// Parse all H2s
	if ctx.HTMLFeatures.H2All != "" {
		var allH2s []string
		if err := json.Unmarshal([]byte(ctx.HTMLFeatures.H2All), &allH2s); err == nil {
			result.Data["h2_all"] = allH2s
			if len(allH2s) > 0 {
				result.Data["h2"] = allH2s[0]
			}
		}
	}

	if h2Count == 0 {
		result.Data["status"] = "Missing"
		result.Data["h2"] = ""
	} else {
		result.Data["status"] = "OK"
	}

	return result
}

func (a *H2Analyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["h2"]),
		fmt.Sprintf("%v", result.Data["count"]),
		fmt.Sprintf("%v", result.Data["status"]),
	}
}
