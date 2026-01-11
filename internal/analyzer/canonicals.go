package analyzer

import (
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// CanonicalsAnalyzer analyzes canonical tags.
type CanonicalsAnalyzer struct {
	urlToCanonical map[int64]string // URL ID -> canonical URL
}

func NewCanonicalsAnalyzer() *CanonicalsAnalyzer {
	return &CanonicalsAnalyzer{
		urlToCanonical: make(map[int64]string),
	}
}

func (a *CanonicalsAnalyzer) Name() string {
	return "Canonicals"
}

func (a *CanonicalsAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "canonical", Title: "Canonical", Width: 300, Sortable: true, DataKey: "canonical"},
		{ID: "status", Title: "Status", Width: 120, Sortable: true, DataKey: "status"},
		{ID: "canonical_status", Title: "Canonical Status", Width: 100, Sortable: true, DataKey: "canonical_status"},
		{ID: "is_self", Title: "Self-Ref", Width: 70, Sortable: true, DataKey: "is_self"},
	}
}

func (a *CanonicalsAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "missing", Label: "Missing", Description: "Pages without canonical", FilterFunc: func(r *AnalysisResult) bool {
			canonical, _ := r.Data["canonical"].(string)
			return canonical == ""
		}},
		{ID: "self", Label: "Self-Referencing", Description: "Pages with self-referencing canonical", FilterFunc: func(r *AnalysisResult) bool {
			if isSelf, ok := r.Data["is_self"].(bool); ok {
				return isSelf
			}
			return false
		}},
		{ID: "canonicalised", Label: "Canonicalised", Description: "Pages pointing to different URL", FilterFunc: func(r *AnalysisResult) bool {
			if isSelf, ok := r.Data["is_self"].(bool); ok {
				canonical, _ := r.Data["canonical"].(string)
				return canonical != "" && !isSelf
			}
			return false
		}},
		{ID: "non_indexable", Label: "Non-Indexable Canonical", Description: "Canonical points to non-indexable page", FilterFunc: func(r *AnalysisResult) bool {
			status, _ := r.Data["status"].(string)
			return status == "Non-Indexable Target"
		}},
		{ID: "broken", Label: "Broken Canonical", Description: "Canonical points to 4xx/5xx", FilterFunc: func(r *AnalysisResult) bool {
			if code, ok := r.Data["canonical_status_code"].(int); ok {
				return code >= 400
			}
			return false
		}},
	}
}

func (a *CanonicalsAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.HTMLFeatures == nil {
		result.Data["canonical"] = ""
		result.Data["status"] = "No HTML"
		result.Data["is_self"] = false
		return result
	}

	canonical := ctx.HTMLFeatures.Canonical
	result.Data["canonical"] = canonical

	// Track canonical for later analysis
	a.urlToCanonical[ctx.URL.ID] = canonical

	// Check if self-referencing
	isSelf := canonical == ctx.URL.URL || canonical == ctx.URL.NormalizedURL
	result.Data["is_self"] = isSelf

	// Determine status
	if canonical == "" {
		result.Data["status"] = "Missing"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueMissingCanonical,
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"canonicals",
			"Page is missing a canonical tag",
		))
	} else if isSelf {
		result.Data["status"] = "Self-Referencing"
	} else {
		result.Data["status"] = "Canonicalised"

		// Check if canonical target exists and its status
		if ctx.AllURLs != nil {
			if targetURL, exists := ctx.AllURLs[canonical]; exists {
				result.Data["canonical_url_id"] = targetURL.ID
				// Additional checks would need the target's fetch data
			}
		}
	}

	return result
}

// AnalyzeCanonicalChains detects canonical chains.
func (a *CanonicalsAnalyzer) AnalyzeCanonicalChains(allURLs map[int64]*storage.URL, allFeatures map[int64]*storage.HTMLFeatures) []*storage.Issue {
	issues := make([]*storage.Issue, 0)

	// Build canonical graph
	canonicalTargets := make(map[string]string) // URL -> canonical
	urlToID := make(map[string]int64)

	for id, url := range allURLs {
		urlToID[url.URL] = id
		urlToID[url.NormalizedURL] = id
	}

	for id, features := range allFeatures {
		if features.Canonical != "" {
			if url, exists := allURLs[id]; exists {
				canonicalTargets[url.URL] = features.Canonical
			}
		}
	}

	// Detect chains
	for url, canonical := range canonicalTargets {
		if canonical == url {
			continue // Self-referencing, skip
		}

		// Follow the chain
		visited := make(map[string]bool)
		current := canonical
		chainLength := 1

		for {
			if visited[current] {
				// Loop detected
				if urlID, ok := urlToID[url]; ok {
					issues = append(issues, NewIssue(
						urlID,
						storage.IssueCanonicalChain,
						storage.IssueTypeError,
						storage.SeverityHigh,
						"canonicals",
						"Canonical chain contains a loop",
					))
				}
				break
			}

			visited[current] = true
			next, exists := canonicalTargets[current]
			if !exists || next == current {
				break
			}

			current = next
			chainLength++

			if chainLength > 2 {
				if urlID, ok := urlToID[url]; ok {
					issues = append(issues, NewIssue(
						urlID,
						storage.IssueCanonicalChain,
						storage.IssueTypeWarning,
						storage.SeverityMedium,
						"canonicals",
						fmt.Sprintf("Canonical chain detected (length: %d)", chainLength),
					))
				}
				break
			}
		}
	}

	return issues
}

func (a *CanonicalsAnalyzer) Reset() {
	a.urlToCanonical = make(map[int64]string)
}

func (a *CanonicalsAnalyzer) ExportRow(result *AnalysisResult) []string {
	isSelf := "No"
	if s, ok := result.Data["is_self"].(bool); ok && s {
		isSelf = "Yes"
	}
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["canonical"]),
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["canonical_status"]),
		isSelf,
	}
}
