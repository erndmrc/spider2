package analyzer

import (
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// LinksAnalyzer analyzes page links.
type LinksAnalyzer struct{}

func NewLinksAnalyzer() *LinksAnalyzer {
	return &LinksAnalyzer{}
}

func (a *LinksAnalyzer) Name() string {
	return "Links"
}

func (a *LinksAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "from_url", Title: "From", Width: 250, Sortable: true, DataKey: "from_url"},
		{ID: "to_url", Title: "To", Width: 250, Sortable: true, DataKey: "to_url"},
		{ID: "anchor_text", Title: "Anchor Text", Width: 150, Sortable: true, DataKey: "anchor_text"},
		{ID: "rel", Title: "Rel", Width: 100, Sortable: true, DataKey: "rel"},
		{ID: "status", Title: "Status", Width: 70, Sortable: true, DataKey: "status"},
		{ID: "type", Title: "Type", Width: 80, Sortable: true, DataKey: "type"},
	}
}

func (a *LinksAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All links"},
		{ID: "internal", Label: "Internal", Description: "Internal links only", FilterFunc: func(r *AnalysisResult) bool {
			if internal, ok := r.Data["is_internal"].(bool); ok {
				return internal
			}
			return false
		}},
		{ID: "external", Label: "External", Description: "External links only", FilterFunc: func(r *AnalysisResult) bool {
			if internal, ok := r.Data["is_internal"].(bool); ok {
				return !internal
			}
			return true
		}},
		{ID: "follow", Label: "Follow", Description: "Followed links", FilterFunc: func(r *AnalysisResult) bool {
			if follow, ok := r.Data["is_follow"].(bool); ok {
				return follow
			}
			return true
		}},
		{ID: "nofollow", Label: "Nofollow", Description: "Nofollow links", FilterFunc: func(r *AnalysisResult) bool {
			if follow, ok := r.Data["is_follow"].(bool); ok {
				return !follow
			}
			return false
		}},
		{ID: "broken", Label: "Broken", Description: "Broken links (4xx/5xx)", FilterFunc: func(r *AnalysisResult) bool {
			if status, ok := r.Data["target_status"].(int); ok {
				return status >= 400
			}
			return false
		}},
		{ID: "redirect", Label: "Redirects", Description: "Links to redirects", FilterFunc: func(r *AnalysisResult) bool {
			if status, ok := r.Data["target_status"].(int); ok {
				return status >= 300 && status < 400
			}
			return false
		}},
	}
}

func (a *LinksAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	// This analyzer works per-page to analyze all links
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.Links == nil {
		result.Data["total_links"] = 0
		result.Data["internal_links"] = 0
		result.Data["external_links"] = 0
		return result
	}

	internalCount := 0
	externalCount := 0
	followCount := 0
	nofollowCount := 0

	for _, link := range ctx.Links {
		if link.IsInternal {
			internalCount++
		} else {
			externalCount++
		}
		if link.IsFollow {
			followCount++
		} else {
			nofollowCount++
		}
	}

	result.Data["total_links"] = len(ctx.Links)
	result.Data["internal_links"] = internalCount
	result.Data["external_links"] = externalCount
	result.Data["follow_links"] = followCount
	result.Data["nofollow_links"] = nofollowCount

	// Check for pages with no internal outlinks
	if internalCount == 0 && ctx.URL.IsInternal {
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueNoInternalLinks,
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"links",
			"Page has no internal outlinks",
		))
	}

	return result
}

// AnalyzeLink analyzes a single link.
func (a *LinksAnalyzer) AnalyzeLink(link *storage.Link, fromURL string, targetStatus int) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  link.FromURLID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["from_url"] = fromURL
	result.Data["to_url"] = link.ToURL
	result.Data["anchor_text"] = link.AnchorText
	result.Data["rel"] = link.Rel
	result.Data["is_internal"] = link.IsInternal
	result.Data["is_follow"] = link.IsFollow
	result.Data["target_status"] = targetStatus

	// Type classification
	if link.IsInternal {
		result.Data["type"] = "Internal"
	} else {
		result.Data["type"] = "External"
	}

	// Status classification
	if targetStatus >= 200 && targetStatus < 300 {
		result.Data["status"] = targetStatus
	} else if targetStatus >= 300 && targetStatus < 400 {
		result.Data["status"] = targetStatus
		result.Issues = append(result.Issues, NewIssue(
			link.FromURLID,
			storage.IssueRedirectLink,
			storage.IssueTypeWarning,
			storage.SeverityLow,
			"links",
			fmt.Sprintf("Link points to redirect (%d): %s", targetStatus, link.ToURL),
		))
	} else if targetStatus >= 400 {
		result.Data["status"] = targetStatus
		result.Issues = append(result.Issues, NewIssue(
			link.FromURLID,
			storage.IssueBrokenLink,
			storage.IssueTypeError,
			storage.SeverityHigh,
			"links",
			fmt.Sprintf("Broken link (%d): %s", targetStatus, link.ToURL),
		))
	} else {
		result.Data["status"] = "Unknown"
	}

	return result
}

// AnalyzeOrphanPages detects pages with no internal inlinks.
func (a *LinksAnalyzer) AnalyzeOrphanPages(allURLs map[int64]*storage.URL, allLinks []*storage.Link) []*storage.Issue {
	issues := make([]*storage.Issue, 0)

	// Build inlinks count map
	inlinksCount := make(map[int64]int)
	for _, link := range allLinks {
		if link.ToURLID != nil && link.IsInternal {
			inlinksCount[*link.ToURLID]++
		}
	}

	// Find orphan pages (no internal inlinks except from itself)
	for id, url := range allURLs {
		if !url.IsInternal {
			continue
		}
		if url.Depth == 0 {
			continue // Seed URLs are not orphans
		}

		if inlinksCount[id] == 0 {
			issues = append(issues, NewIssue(
				id,
				storage.IssueOrphanPage,
				storage.IssueTypeWarning,
				storage.SeverityMedium,
				"links",
				fmt.Sprintf("Orphan page: no internal links pointing to %s", url.URL),
			))
		}
	}

	return issues
}

func (a *LinksAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["from_url"]),
		fmt.Sprintf("%v", result.Data["to_url"]),
		fmt.Sprintf("%v", result.Data["anchor_text"]),
		fmt.Sprintf("%v", result.Data["rel"]),
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["type"]),
	}
}
