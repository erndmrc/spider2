package analyzer

import (
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// ImagesAnalyzer analyzes images on pages.
type ImagesAnalyzer struct{}

func NewImagesAnalyzer() *ImagesAnalyzer {
	return &ImagesAnalyzer{}
}

func (a *ImagesAnalyzer) Name() string {
	return "Images"
}

func (a *ImagesAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Image URL", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "status_code", Title: "Status", Width: 70, Sortable: true, DataKey: "status_code"},
		{ID: "alt", Title: "Alt Text", Width: 200, Sortable: true, DataKey: "alt"},
		{ID: "size", Title: "Size", Width: 80, Sortable: true, DataKey: "size"},
		{ID: "found_on", Title: "Found On", Width: 200, Sortable: true, DataKey: "found_on"},
		{ID: "dimensions", Title: "Dimensions", Width: 100, Sortable: true, DataKey: "dimensions"},
	}
}

func (a *ImagesAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All images"},
		{ID: "missing_alt", Label: "Missing Alt", Description: "Images without alt text", FilterFunc: func(r *AnalysisResult) bool {
			alt, _ := r.Data["alt"].(string)
			return alt == ""
		}},
		{ID: "over_100kb", Label: "Over 100KB", Description: "Large images > 100KB", FilterFunc: func(r *AnalysisResult) bool {
			if size, ok := r.Data["size_bytes"].(int64); ok {
				return size > Thresholds.LargeImageSize
			}
			return false
		}},
		{ID: "broken", Label: "Broken (4xx)", Description: "Broken images", FilterFunc: func(r *AnalysisResult) bool {
			if code, ok := r.Data["status_code"].(int); ok {
				return code >= 400
			}
			return false
		}},
		{ID: "missing_alt_large", Label: "Missing Alt Over 100KB", Description: "Large images without alt", FilterFunc: func(r *AnalysisResult) bool {
			alt, _ := r.Data["alt"].(string)
			size, _ := r.Data["size_bytes"].(int64)
			return alt == "" && size > Thresholds.LargeImageSize
		}},
	}
}

func (a *ImagesAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	// This analyzer works per-resource, not per-page
	// So we expect the resource to be passed in context
	return result
}

// AnalyzeResource analyzes a single image resource.
func (a *ImagesAnalyzer) AnalyzeResource(resource *storage.Resource, foundOnURL string, foundOnURLID int64) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  foundOnURLID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = resource.URL
	result.Data["alt"] = resource.Alt
	result.Data["status_code"] = resource.StatusCode
	result.Data["size_bytes"] = resource.Size
	result.Data["size"] = formatSize(resource.Size)
	result.Data["found_on"] = foundOnURL

	if resource.Width > 0 && resource.Height > 0 {
		result.Data["dimensions"] = fmt.Sprintf("%dx%d", resource.Width, resource.Height)
	} else {
		result.Data["dimensions"] = ""
	}

	// Generate issues
	if resource.Alt == "" {
		result.Issues = append(result.Issues, NewIssue(
			foundOnURLID,
			storage.IssueMissingAlt,
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"images",
			fmt.Sprintf("Image missing alt text: %s", resource.URL),
		))
	}

	if resource.StatusCode >= 400 {
		result.Issues = append(result.Issues, NewIssue(
			foundOnURLID,
			storage.IssueBrokenImage,
			storage.IssueTypeError,
			storage.SeverityHigh,
			"images",
			fmt.Sprintf("Broken image (status %d): %s", resource.StatusCode, resource.URL),
		))
	}

	if resource.Size > Thresholds.LargeImageSize {
		result.Issues = append(result.Issues, NewIssue(
			foundOnURLID,
			storage.IssueLargeImage,
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"images",
			fmt.Sprintf("Large image (%s): %s", formatSize(resource.Size), resource.URL),
		))
	}

	return result
}

// AnalyzePageImages analyzes all images on a page.
func (a *ImagesAnalyzer) AnalyzePageImages(resources []*storage.Resource, pageURL string, pageURLID int64) []*AnalysisResult {
	results := make([]*AnalysisResult, 0)

	for _, resource := range resources {
		if resource.ResourceType == "image" {
			results = append(results, a.AnalyzeResource(resource, pageURL, pageURLID))
		}
	}

	return results
}

func (a *ImagesAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["status_code"]),
		fmt.Sprintf("%v", result.Data["alt"]),
		fmt.Sprintf("%v", result.Data["size"]),
		fmt.Sprintf("%v", result.Data["found_on"]),
		fmt.Sprintf("%v", result.Data["dimensions"]),
	}
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
}
