package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// HreflangAnalyzer analyzes hreflang tags.
type HreflangAnalyzer struct {
	// Map of URL -> hreflang entries for return link validation
	urlHreflangs map[string][]HreflangEntry
}

// HreflangEntry represents a single hreflang entry.
type HreflangEntry struct {
	Hreflang string
	Href     string
}

func NewHreflangAnalyzer() *HreflangAnalyzer {
	return &HreflangAnalyzer{
		urlHreflangs: make(map[string][]HreflangEntry),
	}
}

func (a *HreflangAnalyzer) Name() string {
	return "Hreflang"
}

func (a *HreflangAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 250, Sortable: true, DataKey: "url"},
		{ID: "hreflang", Title: "Hreflang", Width: 80, Sortable: true, DataKey: "hreflang"},
		{ID: "href", Title: "Href", Width: 250, Sortable: true, DataKey: "href"},
		{ID: "status", Title: "Status", Width: 100, Sortable: true, DataKey: "status"},
		{ID: "return_link", Title: "Return Link", Width: 80, Sortable: true, DataKey: "return_link"},
	}
}

func (a *HreflangAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "contains", Label: "Contains Hreflang", Description: "Pages with hreflang", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["hreflang_count"].(int); ok {
				return count > 0
			}
			return false
		}},
		{ID: "missing_return", Label: "Missing Return Links", Description: "Hreflang without return links", FilterFunc: func(r *AnalysisResult) bool {
			if missing, ok := r.Data["missing_return"].(bool); ok {
				return missing
			}
			return false
		}},
		{ID: "missing_self", Label: "Missing Self Reference", Description: "Missing x-default or self reference", FilterFunc: func(r *AnalysisResult) bool {
			if missingSelf, ok := r.Data["missing_self_ref"].(bool); ok {
				return missingSelf
			}
			return false
		}},
		{ID: "invalid_code", Label: "Invalid Language Code", Description: "Invalid hreflang code", FilterFunc: func(r *AnalysisResult) bool {
			if invalid, ok := r.Data["invalid_code"].(bool); ok {
				return invalid
			}
			return false
		}},
	}
}

func (a *HreflangAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.HTMLFeatures == nil || ctx.HTMLFeatures.Hreflangs == "" {
		result.Data["hreflang_count"] = 0
		result.Data["hreflang"] = ""
		result.Data["href"] = ""
		result.Data["status"] = "No Hreflang"
		return result
	}

	// Parse hreflangs JSON
	var entries []HreflangEntry
	if err := json.Unmarshal([]byte(ctx.HTMLFeatures.Hreflangs), &entries); err != nil {
		result.Data["status"] = "Parse Error"
		return result
	}

	result.Data["hreflang_count"] = len(entries)

	// Store for return link validation
	a.urlHreflangs[ctx.URL.URL] = entries

	// Analyze each entry
	hasSelfRef := false
	hasXDefault := false
	invalidCodes := make([]string, 0)

	for _, entry := range entries {
		// Check for self-reference
		if entry.Href == ctx.URL.URL {
			hasSelfRef = true
		}

		// Check for x-default
		if entry.Hreflang == "x-default" {
			hasXDefault = true
		}

		// Validate language code
		if !a.isValidHreflangCode(entry.Hreflang) {
			invalidCodes = append(invalidCodes, entry.Hreflang)
		}
	}

	result.Data["has_self_ref"] = hasSelfRef
	result.Data["has_x_default"] = hasXDefault
	result.Data["missing_self_ref"] = !hasSelfRef

	// First entry for display
	if len(entries) > 0 {
		result.Data["hreflang"] = entries[0].Hreflang
		result.Data["href"] = entries[0].Href
	}

	// Generate issues
	if !hasSelfRef {
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			"hreflang_missing_self",
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"hreflang",
			"Hreflang is missing self-referencing entry",
		))
	}

	if len(invalidCodes) > 0 {
		result.Data["invalid_code"] = true
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			"hreflang_invalid_code",
			storage.IssueTypeError,
			storage.SeverityHigh,
			"hreflang",
			fmt.Sprintf("Invalid hreflang codes: %s", strings.Join(invalidCodes, ", ")),
		))
	} else {
		result.Data["invalid_code"] = false
	}

	result.Data["status"] = "Has Hreflang"

	return result
}

// AnalyzeReturnLinks validates return links after all pages are analyzed.
func (a *HreflangAnalyzer) AnalyzeReturnLinks() []*storage.Issue {
	issues := make([]*storage.Issue, 0)

	for pageURL, entries := range a.urlHreflangs {
		for _, entry := range entries {
			if entry.Href == pageURL {
				continue // Skip self-reference
			}

			// Check if target page has return link
			targetEntries, exists := a.urlHreflangs[entry.Href]
			if !exists {
				// Target page not crawled or has no hreflang
				continue
			}

			hasReturnLink := false
			for _, targetEntry := range targetEntries {
				if targetEntry.Href == pageURL {
					hasReturnLink = true
					break
				}
			}

			if !hasReturnLink {
				issues = append(issues, &storage.Issue{
					IssueCode: "hreflang_missing_return",
					IssueType: storage.IssueTypeWarning,
					Severity:  storage.SeverityMedium,
					Category:  "hreflang",
					Message:   fmt.Sprintf("Missing return link: %s -> %s (lang: %s)", pageURL, entry.Href, entry.Hreflang),
				})
			}
		}
	}

	return issues
}

// isValidHreflangCode validates hreflang language/region codes.
func (a *HreflangAnalyzer) isValidHreflangCode(code string) bool {
	if code == "x-default" {
		return true
	}

	// Basic validation: 2-letter language code, optional 2-letter region
	parts := strings.Split(strings.ToLower(code), "-")
	if len(parts) == 0 || len(parts) > 2 {
		return false
	}

	// Language code should be 2-3 characters
	if len(parts[0]) < 2 || len(parts[0]) > 3 {
		return false
	}

	// Region code should be 2 characters
	if len(parts) == 2 && len(parts[1]) != 2 {
		return false
	}

	return true
}

func (a *HreflangAnalyzer) Reset() {
	a.urlHreflangs = make(map[string][]HreflangEntry)
}

func (a *HreflangAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["hreflang"]),
		fmt.Sprintf("%v", result.Data["href"]),
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["return_link"]),
	}
}

// ExpandHreflangRows expands a single page result into multiple rows (one per hreflang entry).
func (a *HreflangAnalyzer) ExpandHreflangRows(ctx *AnalysisContext) []*AnalysisResult {
	results := make([]*AnalysisResult, 0)

	if ctx.HTMLFeatures == nil || ctx.HTMLFeatures.Hreflangs == "" {
		return results
	}

	var entries []HreflangEntry
	if err := json.Unmarshal([]byte(ctx.HTMLFeatures.Hreflangs), &entries); err != nil {
		return results
	}

	for _, entry := range entries {
		result := &AnalysisResult{
			URLID:  ctx.URL.ID,
			Issues: make([]*storage.Issue, 0),
			Data: map[string]interface{}{
				"url":      ctx.URL.URL,
				"hreflang": entry.Hreflang,
				"href":     entry.Href,
				"status":   "OK",
			},
		}

		// Check if this is self-reference
		if entry.Href == ctx.URL.URL {
			result.Data["is_self"] = true
		}

		// Validate code
		if !a.isValidHreflangCode(entry.Hreflang) {
			result.Data["status"] = "Invalid Code"
		}

		results = append(results, result)
	}

	return results
}
