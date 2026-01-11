package analyzer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// MobileAnalyzer analyzes mobile-friendliness.
type MobileAnalyzer struct{}

func NewMobileAnalyzer() *MobileAnalyzer {
	return &MobileAnalyzer{}
}

func (a *MobileAnalyzer) Name() string {
	return "Mobile"
}

func (a *MobileAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "viewport", Title: "Viewport", Width: 80, Sortable: true, DataKey: "has_viewport"},
		{ID: "viewport_content", Title: "Viewport Content", Width: 200, Sortable: true, DataKey: "viewport_content"},
		{ID: "responsive", Title: "Responsive", Width: 80, Sortable: true, DataKey: "is_responsive"},
		{ID: "touch_friendly", Title: "Touch Friendly", Width: 90, Sortable: true, DataKey: "touch_friendly"},
		{ID: "status", Title: "Status", Width: 100, Sortable: true, DataKey: "status"},
	}
}

func (a *MobileAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "missing_viewport", Label: "Missing Viewport", Description: "Pages without viewport meta", FilterFunc: func(r *AnalysisResult) bool {
			if hasVP, ok := r.Data["has_viewport"].(bool); ok {
				return !hasVP
			}
			return true
		}},
		{ID: "not_responsive", Label: "Not Responsive", Description: "Pages without responsive design", FilterFunc: func(r *AnalysisResult) bool {
			if responsive, ok := r.Data["is_responsive"].(bool); ok {
				return !responsive
			}
			return true
		}},
		{ID: "fixed_width", Label: "Fixed Width", Description: "Pages with fixed width viewport", FilterFunc: func(r *AnalysisResult) bool {
			if fixed, ok := r.Data["has_fixed_width"].(bool); ok {
				return fixed
			}
			return false
		}},
		{ID: "mobile_friendly", Label: "Mobile Friendly", Description: "Pages passing all checks", FilterFunc: func(r *AnalysisResult) bool {
			if status, ok := r.Data["status"].(string); ok {
				return status == "Mobile Friendly"
			}
			return false
		}},
	}
}

func (a *MobileAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.RawHTML == nil {
		result.Data["status"] = "No HTML"
		return result
	}

	htmlStr := string(ctx.RawHTML)

	// Check for viewport meta tag
	viewportResult := a.checkViewport(htmlStr)
	result.Data["has_viewport"] = viewportResult.hasViewport
	result.Data["viewport_content"] = viewportResult.content
	result.Data["has_fixed_width"] = viewportResult.hasFixedWidth
	result.Data["has_user_scalable_no"] = viewportResult.userScalableNo

	// Check for responsive indicators
	responsiveResult := a.checkResponsive(htmlStr)
	result.Data["is_responsive"] = responsiveResult.isResponsive
	result.Data["has_media_queries"] = responsiveResult.hasMediaQueries
	result.Data["has_fluid_layout"] = responsiveResult.hasFluidLayout

	// Check for touch-friendly elements
	touchResult := a.checkTouchFriendly(htmlStr)
	result.Data["touch_friendly"] = touchResult.isTouchFriendly
	result.Data["small_tap_targets"] = touchResult.smallTapTargets

	// Determine overall status
	issues := make([]string, 0)

	if !viewportResult.hasViewport {
		issues = append(issues, "Missing viewport")
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			"mobile_missing_viewport",
			storage.IssueTypeError,
			storage.SeverityHigh,
			"mobile",
			"Page is missing viewport meta tag",
		))
	} else {
		if viewportResult.hasFixedWidth {
			issues = append(issues, "Fixed width")
			result.Issues = append(result.Issues, NewIssue(
				ctx.URL.ID,
				"mobile_fixed_width",
				storage.IssueTypeWarning,
				storage.SeverityMedium,
				"mobile",
				"Viewport has fixed width instead of device-width",
			))
		}
		if viewportResult.userScalableNo {
			issues = append(issues, "Zoom disabled")
			result.Issues = append(result.Issues, NewIssue(
				ctx.URL.ID,
				"mobile_zoom_disabled",
				storage.IssueTypeWarning,
				storage.SeverityMedium,
				"mobile",
				"User scaling/zoom is disabled (user-scalable=no)",
			))
		}
	}

	if !responsiveResult.isResponsive {
		issues = append(issues, "Not responsive")
	}

	// Set status
	if len(issues) == 0 {
		result.Data["status"] = "Mobile Friendly"
	} else if len(issues) <= 2 {
		result.Data["status"] = "Needs Work"
	} else {
		result.Data["status"] = "Not Mobile Friendly"
	}

	return result
}

type viewportResult struct {
	hasViewport    bool
	content        string
	hasFixedWidth  bool
	userScalableNo bool
}

func (a *MobileAnalyzer) checkViewport(html string) viewportResult {
	result := viewportResult{}

	// Find viewport meta tag
	re := regexp.MustCompile(`<meta[^>]*name=["']viewport["'][^>]*content=["']([^"']+)["'][^>]*>`)
	matches := re.FindStringSubmatch(html)

	if len(matches) < 2 {
		// Try alternate order (content before name)
		re = regexp.MustCompile(`<meta[^>]*content=["']([^"']+)["'][^>]*name=["']viewport["'][^>]*>`)
		matches = re.FindStringSubmatch(html)
	}

	if len(matches) >= 2 {
		result.hasViewport = true
		result.content = matches[1]

		content := strings.ToLower(result.content)

		// Check for device-width
		if !strings.Contains(content, "device-width") {
			// Check for fixed width
			if regexp.MustCompile(`width=\d+`).MatchString(content) {
				result.hasFixedWidth = true
			}
		}

		// Check for user-scalable=no
		if strings.Contains(content, "user-scalable=no") ||
			strings.Contains(content, "user-scalable=0") {
			result.userScalableNo = true
		}

		// Check for maximum-scale=1 (also prevents zooming)
		if strings.Contains(content, "maximum-scale=1") {
			result.userScalableNo = true
		}
	}

	return result
}

type responsiveResult struct {
	isResponsive    bool
	hasMediaQueries bool
	hasFluidLayout  bool
}

func (a *MobileAnalyzer) checkResponsive(html string) responsiveResult {
	result := responsiveResult{}

	// Check for media queries in inline styles
	if strings.Contains(html, "@media") {
		result.hasMediaQueries = true
	}

	// Check for percentage-based widths (fluid layout indicators)
	percentWidthRe := regexp.MustCompile(`width:\s*\d+%`)
	if percentWidthRe.MatchString(html) {
		result.hasFluidLayout = true
	}

	// Check for max-width usage
	if strings.Contains(html, "max-width") {
		result.hasFluidLayout = true
	}

	// Check for responsive framework classes
	responsiveClasses := []string{
		"container-fluid", "col-xs", "col-sm", "col-md", "col-lg",
		"grid", "flex", "responsive", "mobile",
	}
	for _, class := range responsiveClasses {
		if strings.Contains(html, class) {
			result.isResponsive = true
			break
		}
	}

	// Consider responsive if has media queries or fluid layout
	if result.hasMediaQueries || result.hasFluidLayout {
		result.isResponsive = true
	}

	return result
}

type touchResult struct {
	isTouchFriendly bool
	smallTapTargets int
}

func (a *MobileAnalyzer) checkTouchFriendly(html string) touchResult {
	result := touchResult{
		isTouchFriendly: true, // Assume friendly unless we find issues
	}

	// Count potentially small tap targets
	// This is a heuristic - real check would need CSS parsing

	// Check for very small font sizes
	smallFontRe := regexp.MustCompile(`font-size:\s*([0-9]+)px`)
	matches := smallFontRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			// Font sizes under 12px are hard to tap
			// This is simplified - real implementation would check tap target size
		}
	}

	// Check for touch-action CSS
	if strings.Contains(html, "touch-action") {
		result.isTouchFriendly = true
	}

	return result
}

func (a *MobileAnalyzer) ExportRow(result *AnalysisResult) []string {
	hasVP := "No"
	if v, ok := result.Data["has_viewport"].(bool); ok && v {
		hasVP = "Yes"
	}
	isResp := "No"
	if r, ok := result.Data["is_responsive"].(bool); ok && r {
		isResp = "Yes"
	}
	touchFr := "Unknown"
	if t, ok := result.Data["touch_friendly"].(bool); ok {
		if t {
			touchFr = "Yes"
		} else {
			touchFr = "No"
		}
	}
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		hasVP,
		fmt.Sprintf("%v", result.Data["viewport_content"]),
		isResp,
		touchFr,
		fmt.Sprintf("%v", result.Data["status"]),
	}
}
