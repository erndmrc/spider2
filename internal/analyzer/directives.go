package analyzer

import (
	"fmt"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// DirectivesAnalyzer analyzes robots directives (meta robots, X-Robots-Tag).
type DirectivesAnalyzer struct{}

func NewDirectivesAnalyzer() *DirectivesAnalyzer {
	return &DirectivesAnalyzer{}
}

func (a *DirectivesAnalyzer) Name() string {
	return "Directives"
}

func (a *DirectivesAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "meta_robots", Title: "Meta Robots", Width: 150, Sortable: true, DataKey: "meta_robots"},
		{ID: "x_robots", Title: "X-Robots-Tag", Width: 150, Sortable: true, DataKey: "x_robots"},
		{ID: "indexability", Title: "Indexability", Width: 100, Sortable: true, DataKey: "indexability"},
		{ID: "reason", Title: "Reason", Width: 150, Sortable: true, DataKey: "reason"},
	}
}

func (a *DirectivesAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "indexable", Label: "Indexable", Description: "Pages that can be indexed", FilterFunc: func(r *AnalysisResult) bool {
			if indexable, ok := r.Data["is_indexable"].(bool); ok {
				return indexable
			}
			return true
		}},
		{ID: "noindex", Label: "Noindex", Description: "Pages with noindex", FilterFunc: func(r *AnalysisResult) bool {
			if noindex, ok := r.Data["noindex"].(bool); ok {
				return noindex
			}
			return false
		}},
		{ID: "nofollow", Label: "Nofollow", Description: "Pages with nofollow", FilterFunc: func(r *AnalysisResult) bool {
			if nofollow, ok := r.Data["nofollow"].(bool); ok {
				return nofollow
			}
			return false
		}},
		{ID: "non_indexable", Label: "Non-Indexable", Description: "All non-indexable pages", FilterFunc: func(r *AnalysisResult) bool {
			if indexable, ok := r.Data["is_indexable"].(bool); ok {
				return !indexable
			}
			return false
		}},
	}
}

func (a *DirectivesAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	// Get X-Robots-Tag from headers
	xRobots := ""
	if ctx.Fetch != nil && ctx.Fetch.Headers != nil {
		if xr, ok := ctx.Fetch.Headers["X-Robots-Tag"]; ok {
			xRobots = xr
		}
	}
	result.Data["x_robots"] = xRobots

	// Get meta robots
	metaRobots := ""
	if ctx.HTMLFeatures != nil {
		metaRobots = ctx.HTMLFeatures.MetaRobots
	}
	result.Data["meta_robots"] = metaRobots

	// Parse directives
	noindex := false
	nofollow := false
	noarchive := false
	nosnippet := false

	// Parse meta robots
	if metaRobots != "" {
		directives := strings.ToLower(metaRobots)
		noindex = strings.Contains(directives, "noindex") || directives == "none"
		nofollow = strings.Contains(directives, "nofollow") || directives == "none"
		noarchive = strings.Contains(directives, "noarchive")
		nosnippet = strings.Contains(directives, "nosnippet")
	}

	// Parse X-Robots-Tag (overrides meta robots)
	if xRobots != "" {
		directives := strings.ToLower(xRobots)
		if strings.Contains(directives, "noindex") || directives == "none" {
			noindex = true
		}
		if strings.Contains(directives, "nofollow") || directives == "none" {
			nofollow = true
		}
	}

	result.Data["noindex"] = noindex
	result.Data["nofollow"] = nofollow
	result.Data["noarchive"] = noarchive
	result.Data["nosnippet"] = nosnippet

	// Determine indexability
	isIndexable := !noindex
	result.Data["is_indexable"] = isIndexable

	// Build indexability status and reason
	reasons := make([]string, 0)
	if noindex {
		reasons = append(reasons, "noindex")
	}
	if nofollow {
		reasons = append(reasons, "nofollow")
	}

	if isIndexable {
		result.Data["indexability"] = "Indexable"
		result.Data["reason"] = ""
	} else {
		result.Data["indexability"] = "Non-Indexable"
		result.Data["reason"] = strings.Join(reasons, ", ")

		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueNoindex,
			storage.IssueTypeNotice,
			storage.SeverityLow,
			"directives",
			fmt.Sprintf("Page is non-indexable: %s", strings.Join(reasons, ", ")),
		))
	}

	// Check for canonical + noindex combination
	if noindex && ctx.HTMLFeatures != nil && ctx.HTMLFeatures.Canonical != "" && ctx.HTMLFeatures.Canonical != ctx.URL.URL {
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			"canonical_noindex_conflict",
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"directives",
			"Page has both noindex and a non-self canonical (conflicting signals)",
		))
	}

	return result
}

func (a *DirectivesAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["meta_robots"]),
		fmt.Sprintf("%v", result.Data["x_robots"]),
		fmt.Sprintf("%v", result.Data["indexability"]),
		fmt.Sprintf("%v", result.Data["reason"]),
	}
}
