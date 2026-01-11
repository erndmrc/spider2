package analyzer

import (
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// JavaScriptAnalyzer analyzes JavaScript resources.
type JavaScriptAnalyzer struct{}

func NewJavaScriptAnalyzer() *JavaScriptAnalyzer {
	return &JavaScriptAnalyzer{}
}

func (a *JavaScriptAnalyzer) Name() string {
	return "JavaScript"
}

func (a *JavaScriptAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Script URL", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "status_code", Title: "Status", Width: 70, Sortable: true, DataKey: "status_code"},
		{ID: "size", Title: "Size", Width: 80, Sortable: true, DataKey: "size"},
		{ID: "async", Title: "Async", Width: 60, Sortable: true, DataKey: "async"},
		{ID: "defer", Title: "Defer", Width: 60, Sortable: true, DataKey: "defer"},
		{ID: "type", Title: "Type", Width: 100, Sortable: true, DataKey: "type"},
		{ID: "found_on", Title: "Found On", Width: 200, Sortable: true, DataKey: "found_on"},
	}
}

func (a *JavaScriptAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All JavaScript files"},
		{ID: "async", Label: "Async", Description: "Scripts with async attribute", FilterFunc: func(r *AnalysisResult) bool {
			if async, ok := r.Data["is_async"].(bool); ok {
				return async
			}
			return false
		}},
		{ID: "defer", Label: "Defer", Description: "Scripts with defer attribute", FilterFunc: func(r *AnalysisResult) bool {
			if def, ok := r.Data["is_defer"].(bool); ok {
				return def
			}
			return false
		}},
		{ID: "render_blocking", Label: "Render Blocking", Description: "Scripts without async/defer", FilterFunc: func(r *AnalysisResult) bool {
			async, _ := r.Data["is_async"].(bool)
			def, _ := r.Data["is_defer"].(bool)
			return !async && !def
		}},
		{ID: "broken", Label: "Broken", Description: "Scripts returning 4xx/5xx", FilterFunc: func(r *AnalysisResult) bool {
			if code, ok := r.Data["status_code"].(int); ok {
				return code >= 400
			}
			return false
		}},
		{ID: "large", Label: "Large (>100KB)", Description: "Scripts over 100KB", FilterFunc: func(r *AnalysisResult) bool {
			if size, ok := r.Data["size_bytes"].(int64); ok {
				return size > 100*1024
			}
			return false
		}},
		{ID: "external", Label: "External", Description: "Third-party scripts", FilterFunc: func(r *AnalysisResult) bool {
			if ext, ok := r.Data["is_external"].(bool); ok {
				return ext
			}
			return false
		}},
	}
}

func (a *JavaScriptAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}
	return result
}

// AnalyzeScript analyzes a single JavaScript resource.
func (a *JavaScriptAnalyzer) AnalyzeScript(resource *storage.Resource, foundOnURL string, foundOnURLID int64, pageHost string) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  foundOnURLID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = resource.URL
	result.Data["status_code"] = resource.StatusCode
	result.Data["size_bytes"] = resource.Size
	result.Data["size"] = formatSize(resource.Size)
	result.Data["is_async"] = resource.IsAsync
	result.Data["is_defer"] = resource.IsDefer
	result.Data["found_on"] = foundOnURL

	// Async/Defer display
	if resource.IsAsync {
		result.Data["async"] = "Yes"
	} else {
		result.Data["async"] = "No"
	}
	if resource.IsDefer {
		result.Data["defer"] = "Yes"
	} else {
		result.Data["defer"] = "No"
	}

	// Determine type
	result.Data["type"] = resource.MimeType
	if resource.MimeType == "" {
		result.Data["type"] = "application/javascript"
	}

	// Check if external (different host)
	scriptHost, _ := extractHostFromURL(resource.URL)
	isExternal := scriptHost != "" && scriptHost != pageHost
	result.Data["is_external"] = isExternal

	// Issues
	if resource.StatusCode >= 400 {
		result.Issues = append(result.Issues, NewIssue(
			foundOnURLID,
			"broken_script",
			storage.IssueTypeError,
			storage.SeverityHigh,
			"javascript",
			fmt.Sprintf("Broken script (status %d): %s", resource.StatusCode, resource.URL),
		))
	}

	// Render blocking warning
	if !resource.IsAsync && !resource.IsDefer {
		result.Issues = append(result.Issues, NewIssue(
			foundOnURLID,
			"render_blocking_script",
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"javascript",
			fmt.Sprintf("Render-blocking script: %s", resource.URL),
		))
	}

	return result
}

// AnalyzePageScripts analyzes all scripts on a page.
func (a *JavaScriptAnalyzer) AnalyzePageScripts(resources []*storage.Resource, pageURL string, pageURLID int64, pageHost string) []*AnalysisResult {
	results := make([]*AnalysisResult, 0)

	for _, resource := range resources {
		if resource.ResourceType == "script" {
			results = append(results, a.AnalyzeScript(resource, pageURL, pageURLID, pageHost))
		}
	}

	return results
}

func (a *JavaScriptAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["status_code"]),
		fmt.Sprintf("%v", result.Data["size"]),
		fmt.Sprintf("%v", result.Data["async"]),
		fmt.Sprintf("%v", result.Data["defer"]),
		fmt.Sprintf("%v", result.Data["type"]),
		fmt.Sprintf("%v", result.Data["found_on"]),
	}
}

func extractHostFromURL(rawURL string) (string, error) {
	// Simple host extraction
	if len(rawURL) < 8 {
		return "", nil
	}
	start := 0
	if rawURL[:8] == "https://" {
		start = 8
	} else if rawURL[:7] == "http://" {
		start = 7
	} else {
		return "", nil
	}

	end := start
	for end < len(rawURL) && rawURL[end] != '/' && rawURL[end] != '?' && rawURL[end] != ':' {
		end++
	}
	return rawURL[start:end], nil
}
