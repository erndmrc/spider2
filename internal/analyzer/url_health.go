package analyzer

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/spider-crawler/spider/internal/storage"
)

// URLHealthAnalyzer analyzes URL structure and health.
type URLHealthAnalyzer struct {
	normalizedURLs map[string][]int64 // For duplicate detection
}

func NewURLHealthAnalyzer() *URLHealthAnalyzer {
	return &URLHealthAnalyzer{
		normalizedURLs: make(map[string][]int64),
	}
}

func (a *URLHealthAnalyzer) Name() string {
	return "URL"
}

func (a *URLHealthAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "length", Title: "Length", Width: 70, Sortable: true, DataKey: "length"},
		{ID: "path_depth", Title: "Path Depth", Width: 80, Sortable: true, DataKey: "path_depth"},
		{ID: "params", Title: "Parameters", Width: 80, Sortable: true, DataKey: "params"},
		{ID: "status", Title: "Status", Width: 100, Sortable: true, DataKey: "status"},
		{ID: "issues", Title: "Issues", Width: 150, Sortable: true, DataKey: "issues_text"},
	}
}

func (a *URLHealthAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "over_115", Label: "Over 115 Characters", Description: "URLs exceeding 115 characters", FilterFunc: func(r *AnalysisResult) bool {
			if length, ok := r.Data["length"].(int); ok {
				return length > Thresholds.URLMaxLength
			}
			return false
		}},
		{ID: "uppercase", Label: "Contains Uppercase", Description: "URLs with uppercase characters", FilterFunc: func(r *AnalysisResult) bool {
			if hasUpper, ok := r.Data["has_uppercase"].(bool); ok {
				return hasUpper
			}
			return false
		}},
		{ID: "underscores", Label: "Contains Underscores", Description: "URLs with underscores", FilterFunc: func(r *AnalysisResult) bool {
			if hasUnder, ok := r.Data["has_underscores"].(bool); ok {
				return hasUnder
			}
			return false
		}},
		{ID: "duplicate", Label: "Duplicate", Description: "Duplicate URLs", FilterFunc: func(r *AnalysisResult) bool {
			if occ, ok := r.Data["occurrences"].(int); ok {
				return occ > 1
			}
			return false
		}},
		{ID: "parameters", Label: "Has Parameters", Description: "URLs with query parameters", FilterFunc: func(r *AnalysisResult) bool {
			if params, ok := r.Data["params"].(int); ok {
				return params > 0
			}
			return false
		}},
		{ID: "non_ascii", Label: "Non-ASCII", Description: "URLs with non-ASCII characters", FilterFunc: func(r *AnalysisResult) bool {
			if hasNonASCII, ok := r.Data["has_non_ascii"].(bool); ok {
				return hasNonASCII
			}
			return false
		}},
	}
}

func (a *URLHealthAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	rawURL := ctx.URL.URL
	result.Data["url"] = rawURL
	result.Data["length"] = len(rawURL)

	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		result.Data["status"] = "Invalid URL"
		return result
	}

	// Path depth
	pathDepth := a.calculatePathDepth(parsedURL.Path)
	result.Data["path_depth"] = pathDepth

	// Query parameters
	params := parsedURL.Query()
	result.Data["params"] = len(params)
	result.Data["query_string"] = parsedURL.RawQuery

	// Check for issues
	issuesList := make([]string, 0)

	// Uppercase check
	hasUppercase := a.hasUppercase(parsedURL.Path)
	result.Data["has_uppercase"] = hasUppercase
	if hasUppercase {
		issuesList = append(issuesList, "Uppercase")
	}

	// Underscores check
	hasUnderscores := strings.Contains(parsedURL.Path, "_")
	result.Data["has_underscores"] = hasUnderscores
	if hasUnderscores {
		issuesList = append(issuesList, "Underscores")
	}

	// Non-ASCII check
	hasNonASCII := a.hasNonASCII(rawURL)
	result.Data["has_non_ascii"] = hasNonASCII
	if hasNonASCII {
		issuesList = append(issuesList, "Non-ASCII")
	}

	// Spaces check
	hasSpaces := strings.Contains(rawURL, " ") || strings.Contains(rawURL, "%20")
	result.Data["has_spaces"] = hasSpaces
	if hasSpaces {
		issuesList = append(issuesList, "Spaces")
	}

	// Double slashes check (not counting protocol)
	pathWithQuery := parsedURL.Path + "?" + parsedURL.RawQuery
	hasDoubleSlash := strings.Contains(pathWithQuery, "//")
	result.Data["has_double_slash"] = hasDoubleSlash
	if hasDoubleSlash {
		issuesList = append(issuesList, "Double Slash")
	}

	// Length check
	if len(rawURL) > Thresholds.URLMaxLength {
		issuesList = append(issuesList, "Too Long")
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			"url_too_long",
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"url",
			fmt.Sprintf("URL is too long (%d characters, recommended max: %d)", len(rawURL), Thresholds.URLMaxLength),
		))
	}

	// Track for duplicate detection
	normalized := ctx.URL.NormalizedURL
	a.normalizedURLs[normalized] = append(a.normalizedURLs[normalized], ctx.URL.ID)
	result.Data["occurrences"] = len(a.normalizedURLs[normalized])

	// Set status
	if len(issuesList) == 0 {
		result.Data["status"] = "OK"
		result.Data["issues_text"] = ""
	} else {
		result.Data["status"] = "Issues Found"
		result.Data["issues_text"] = strings.Join(issuesList, ", ")
	}

	return result
}

// AnalyzeDuplicates detects duplicate URLs.
func (a *URLHealthAnalyzer) AnalyzeDuplicates() []*storage.Issue {
	issues := make([]*storage.Issue, 0)

	for normalized, urlIDs := range a.normalizedURLs {
		if len(urlIDs) > 1 {
			for _, urlID := range urlIDs {
				issues = append(issues, NewIssue(
					urlID,
					"duplicate_url",
					storage.IssueTypeWarning,
					storage.SeverityMedium,
					"url",
					fmt.Sprintf("Duplicate URL found (%d occurrences): %s", len(urlIDs), normalized),
				))
			}
		}
	}

	return issues
}

func (a *URLHealthAnalyzer) calculatePathDepth(path string) int {
	if path == "" || path == "/" {
		return 0
	}
	// Remove leading and trailing slashes
	path = strings.Trim(path, "/")
	if path == "" {
		return 0
	}
	return strings.Count(path, "/") + 1
}

func (a *URLHealthAnalyzer) hasUppercase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func (a *URLHealthAnalyzer) hasNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

func (a *URLHealthAnalyzer) Reset() {
	a.normalizedURLs = make(map[string][]int64)
}

func (a *URLHealthAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["length"]),
		fmt.Sprintf("%v", result.Data["path_depth"]),
		fmt.Sprintf("%v", result.Data["params"]),
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["issues_text"]),
	}
}
