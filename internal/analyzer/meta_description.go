package analyzer

import (
	"crypto/md5"
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// MetaDescriptionAnalyzer analyzes meta descriptions.
type MetaDescriptionAnalyzer struct {
	descHashes map[string][]int64
}

func NewMetaDescriptionAnalyzer() *MetaDescriptionAnalyzer {
	return &MetaDescriptionAnalyzer{
		descHashes: make(map[string][]int64),
	}
}

func (a *MetaDescriptionAnalyzer) Name() string {
	return "Meta Description"
}

func (a *MetaDescriptionAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "meta_description", Title: "Meta Description", Width: 350, Sortable: true, DataKey: "meta_description"},
		{ID: "length", Title: "Length", Width: 70, Sortable: true, DataKey: "length"},
		{ID: "pixel_width", Title: "Pixel Width", Width: 90, Sortable: true, DataKey: "pixel_width"},
		{ID: "status", Title: "Status", Width: 100, Sortable: true, DataKey: "status"},
		{ID: "occurrences", Title: "Occurrences", Width: 90, Sortable: true, DataKey: "occurrences"},
	}
}

func (a *MetaDescriptionAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "missing", Label: "Missing", Description: "Pages without meta description", FilterFunc: func(r *AnalysisResult) bool {
			desc, _ := r.Data["meta_description"].(string)
			return desc == ""
		}},
		{ID: "duplicate", Label: "Duplicate", Description: "Duplicate meta descriptions", FilterFunc: func(r *AnalysisResult) bool {
			if occ, ok := r.Data["occurrences"].(int); ok {
				return occ > 1
			}
			return false
		}},
		{ID: "too_long", Label: "Over 155 Characters", Description: "Meta description exceeds 155 characters", FilterFunc: func(r *AnalysisResult) bool {
			if length, ok := r.Data["length"].(int); ok {
				return length > Thresholds.MetaDescMaxLength
			}
			return false
		}},
		{ID: "too_short", Label: "Below 70 Characters", Description: "Meta description under 70 characters", FilterFunc: func(r *AnalysisResult) bool {
			if length, ok := r.Data["length"].(int); ok {
				return length > 0 && length < Thresholds.MetaDescMinLength
			}
			return false
		}},
		{ID: "pixel_truncated", Label: "Pixel Truncated", Description: "May be truncated in SERPs", FilterFunc: func(r *AnalysisResult) bool {
			if width, ok := r.Data["pixel_width"].(int); ok {
				return width > Thresholds.MetaDescMaxPixels
			}
			return false
		}},
	}
}

func (a *MetaDescriptionAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.HTMLFeatures == nil {
		result.Data["meta_description"] = ""
		result.Data["length"] = 0
		result.Data["status"] = "No HTML"
		return result
	}

	desc := ctx.HTMLFeatures.MetaDescription
	descLength := len(desc)

	result.Data["meta_description"] = desc
	result.Data["length"] = descLength
	result.Data["pixel_width"] = a.estimatePixelWidth(desc)

	// Track for duplicate detection
	if desc != "" {
		hash := a.hashDesc(desc)
		a.descHashes[hash] = append(a.descHashes[hash], ctx.URL.ID)
		result.Data["desc_hash"] = hash
		result.Data["occurrences"] = len(a.descHashes[hash])
	} else {
		result.Data["occurrences"] = 0
	}

	// Determine status and generate issues
	if desc == "" {
		result.Data["status"] = "Missing"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueMissingMetaDesc,
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"meta",
			"Page is missing a meta description",
		))
	} else if descLength > Thresholds.MetaDescMaxLength {
		result.Data["status"] = "Too Long"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueMetaDescTooLong,
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"meta",
			fmt.Sprintf("Meta description is too long (%d characters, recommended max: %d)", descLength, Thresholds.MetaDescMaxLength),
		))
	} else if descLength < Thresholds.MetaDescMinLength {
		result.Data["status"] = "Too Short"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueMetaDescTooShort,
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"meta",
			fmt.Sprintf("Meta description is too short (%d characters, recommended min: %d)", descLength, Thresholds.MetaDescMinLength),
		))
	} else {
		result.Data["status"] = "OK"
	}

	return result
}

// AnalyzeDuplicates detects duplicate meta descriptions.
func (a *MetaDescriptionAnalyzer) AnalyzeDuplicates() []*storage.Issue {
	issues := make([]*storage.Issue, 0)

	for hash, urlIDs := range a.descHashes {
		if len(urlIDs) > 1 {
			for _, urlID := range urlIDs {
				issues = append(issues, NewIssue(
					urlID,
					storage.IssueDuplicateMetaDesc,
					storage.IssueTypeWarning,
					storage.SeverityMedium,
					"meta",
					fmt.Sprintf("Duplicate meta description found on %d pages (hash: %s)", len(urlIDs), hash[:8]),
				))
			}
		}
	}

	return issues
}

// Reset clears tracking data.
func (a *MetaDescriptionAnalyzer) Reset() {
	a.descHashes = make(map[string][]int64)
}

func (a *MetaDescriptionAnalyzer) hashDesc(desc string) string {
	hash := md5.Sum([]byte(desc))
	return fmt.Sprintf("%x", hash)
}

func (a *MetaDescriptionAnalyzer) estimatePixelWidth(desc string) int {
	avgCharWidth := 6.0
	return int(float64(len(desc)) * avgCharWidth)
}

func (a *MetaDescriptionAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["meta_description"]),
		fmt.Sprintf("%v", result.Data["length"]),
		fmt.Sprintf("%v", result.Data["pixel_width"]),
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["occurrences"]),
	}
}
