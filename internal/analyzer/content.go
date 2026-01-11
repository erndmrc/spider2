package analyzer

import (
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// ContentAnalyzer analyzes page content.
type ContentAnalyzer struct {
	contentHashes map[string][]int64 // For exact duplicate detection
}

func NewContentAnalyzer() *ContentAnalyzer {
	return &ContentAnalyzer{
		contentHashes: make(map[string][]int64),
	}
}

func (a *ContentAnalyzer) Name() string {
	return "Content"
}

func (a *ContentAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "word_count", Title: "Word Count", Width: 90, Sortable: true, DataKey: "word_count"},
		{ID: "content_hash", Title: "Content Hash", Width: 120, Sortable: true, DataKey: "content_hash"},
		{ID: "status", Title: "Status", Width: 100, Sortable: true, DataKey: "status"},
		{ID: "occurrences", Title: "Duplicates", Width: 80, Sortable: true, DataKey: "occurrences"},
	}
}

func (a *ContentAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "thin", Label: "Low Word Count", Description: "Pages with < 200 words", FilterFunc: func(r *AnalysisResult) bool {
			if wc, ok := r.Data["word_count"].(int); ok {
				return wc > 0 && wc < Thresholds.ThinContentWordCount
			}
			return false
		}},
		{ID: "duplicate", Label: "Duplicate Content", Description: "Exact duplicate content", FilterFunc: func(r *AnalysisResult) bool {
			if occ, ok := r.Data["occurrences"].(int); ok {
				return occ > 1
			}
			return false
		}},
		{ID: "empty", Label: "No Content", Description: "Pages with no content", FilterFunc: func(r *AnalysisResult) bool {
			if wc, ok := r.Data["word_count"].(int); ok {
				return wc == 0
			}
			return true
		}},
	}
}

func (a *ContentAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.HTMLFeatures == nil {
		result.Data["word_count"] = 0
		result.Data["content_hash"] = ""
		result.Data["status"] = "No HTML"
		return result
	}

	wordCount := ctx.HTMLFeatures.WordCount
	contentHash := ctx.HTMLFeatures.ContentHash

	result.Data["word_count"] = wordCount
	result.Data["content_hash"] = contentHash

	// Track for duplicate detection
	if contentHash != "" {
		a.contentHashes[contentHash] = append(a.contentHashes[contentHash], ctx.URL.ID)
		result.Data["occurrences"] = len(a.contentHashes[contentHash])
	} else {
		result.Data["occurrences"] = 0
	}

	// Determine status and generate issues
	if wordCount == 0 {
		result.Data["status"] = "Empty"
	} else if wordCount < Thresholds.ThinContentWordCount {
		result.Data["status"] = "Thin"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueThinContent,
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"content",
			fmt.Sprintf("Thin content: only %d words (recommended min: %d)", wordCount, Thresholds.ThinContentWordCount),
		))
	} else {
		result.Data["status"] = "OK"
	}

	return result
}

// AnalyzeDuplicates detects duplicate content.
func (a *ContentAnalyzer) AnalyzeDuplicates() []*storage.Issue {
	issues := make([]*storage.Issue, 0)

	for hash, urlIDs := range a.contentHashes {
		if len(urlIDs) > 1 {
			for _, urlID := range urlIDs {
				issues = append(issues, NewIssue(
					urlID,
					storage.IssueDuplicateContent,
					storage.IssueTypeWarning,
					storage.SeverityHigh,
					"content",
					fmt.Sprintf("Duplicate content found on %d pages (hash: %s)", len(urlIDs), hash[:8]),
				))
			}
		}
	}

	return issues
}

func (a *ContentAnalyzer) Reset() {
	a.contentHashes = make(map[string][]int64)
}

func (a *ContentAnalyzer) ExportRow(result *AnalysisResult) []string {
	hash := ""
	if h, ok := result.Data["content_hash"].(string); ok && len(h) > 8 {
		hash = h[:8] + "..."
	}
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["word_count"]),
		hash,
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["occurrences"]),
	}
}
