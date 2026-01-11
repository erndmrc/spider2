package analyzer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// AccessibilityAnalyzer analyzes basic accessibility issues.
type AccessibilityAnalyzer struct{}

func NewAccessibilityAnalyzer() *AccessibilityAnalyzer {
	return &AccessibilityAnalyzer{}
}

func (a *AccessibilityAnalyzer) Name() string {
	return "Accessibility"
}

func (a *AccessibilityAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "images_without_alt", Title: "Images w/o Alt", Width: 100, Sortable: true, DataKey: "images_without_alt"},
		{ID: "form_labels", Title: "Form Labels", Width: 90, Sortable: true, DataKey: "form_labels_status"},
		{ID: "heading_structure", Title: "Heading Structure", Width: 110, Sortable: true, DataKey: "heading_status"},
		{ID: "aria_issues", Title: "ARIA Issues", Width: 90, Sortable: true, DataKey: "aria_issues"},
		{ID: "score", Title: "Score", Width: 60, Sortable: true, DataKey: "score"},
	}
}

func (a *AccessibilityAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "missing_alt", Label: "Missing Alt Text", Description: "Images without alt attribute", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["images_without_alt"].(int); ok {
				return count > 0
			}
			return false
		}},
		{ID: "form_issues", Label: "Form Label Issues", Description: "Forms with missing labels", FilterFunc: func(r *AnalysisResult) bool {
			if status, ok := r.Data["form_labels_status"].(string); ok {
				return status != "OK" && status != "N/A"
			}
			return false
		}},
		{ID: "heading_issues", Label: "Heading Structure Issues", Description: "Skipped heading levels", FilterFunc: func(r *AnalysisResult) bool {
			if status, ok := r.Data["heading_status"].(string); ok {
				return status != "OK"
			}
			return false
		}},
		{ID: "has_issues", Label: "Has Issues", Description: "Pages with any accessibility issues", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["total_issues"].(int); ok {
				return count > 0
			}
			return false
		}},
	}
}

func (a *AccessibilityAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL

	if ctx.RawHTML == nil {
		result.Data["score"] = 0
		return result
	}

	htmlStr := string(ctx.RawHTML)
	totalIssues := 0

	// 1. Check images without alt
	imagesWithoutAlt := a.checkImagesAlt(htmlStr)
	result.Data["images_without_alt"] = imagesWithoutAlt
	if imagesWithoutAlt > 0 {
		totalIssues += imagesWithoutAlt
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			"a11y_missing_alt",
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"accessibility",
			fmt.Sprintf("%d images missing alt attribute", imagesWithoutAlt),
		))
	}

	// 2. Check form labels
	formResult := a.checkFormLabels(htmlStr)
	result.Data["form_inputs"] = formResult.totalInputs
	result.Data["form_inputs_without_label"] = formResult.inputsWithoutLabel
	if formResult.totalInputs == 0 {
		result.Data["form_labels_status"] = "N/A"
	} else if formResult.inputsWithoutLabel == 0 {
		result.Data["form_labels_status"] = "OK"
	} else {
		result.Data["form_labels_status"] = fmt.Sprintf("%d missing", formResult.inputsWithoutLabel)
		totalIssues += formResult.inputsWithoutLabel
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			"a11y_missing_form_labels",
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"accessibility",
			fmt.Sprintf("%d form inputs missing labels", formResult.inputsWithoutLabel),
		))
	}

	// 3. Check heading structure
	headingResult := a.checkHeadingStructure(htmlStr)
	result.Data["heading_levels"] = headingResult.levels
	result.Data["heading_skips"] = headingResult.skips
	if len(headingResult.skips) == 0 {
		result.Data["heading_status"] = "OK"
	} else {
		result.Data["heading_status"] = fmt.Sprintf("%d skips", len(headingResult.skips))
		totalIssues += len(headingResult.skips)
		for _, skip := range headingResult.skips {
			result.Issues = append(result.Issues, NewIssue(
				ctx.URL.ID,
				"a11y_heading_skip",
				storage.IssueTypeWarning,
				storage.SeverityLow,
				"accessibility",
				fmt.Sprintf("Heading level skipped: %s", skip),
			))
		}
	}

	// 4. Check ARIA issues
	ariaResult := a.checkARIA(htmlStr)
	result.Data["aria_issues"] = ariaResult.issueCount
	result.Data["aria_details"] = ariaResult.issues
	totalIssues += ariaResult.issueCount

	// 5. Check for lang attribute
	hasLang := a.checkLangAttribute(htmlStr)
	result.Data["has_lang"] = hasLang
	if !hasLang {
		totalIssues++
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			"a11y_missing_lang",
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"accessibility",
			"Page is missing lang attribute on html element",
		))
	}

	// 6. Check for skip links
	hasSkipLink := a.checkSkipLink(htmlStr)
	result.Data["has_skip_link"] = hasSkipLink

	// Calculate score (simplified)
	result.Data["total_issues"] = totalIssues
	score := 100
	score -= imagesWithoutAlt * 5
	score -= formResult.inputsWithoutLabel * 10
	score -= len(headingResult.skips) * 5
	score -= ariaResult.issueCount * 5
	if !hasLang {
		score -= 10
	}
	if score < 0 {
		score = 0
	}
	result.Data["score"] = score

	return result
}

func (a *AccessibilityAnalyzer) checkImagesAlt(html string) int {
	// Find all img tags
	imgRe := regexp.MustCompile(`<img[^>]*>`)
	imgs := imgRe.FindAllString(html, -1)

	count := 0
	for _, img := range imgs {
		// Check if alt attribute exists
		if !strings.Contains(img, "alt=") {
			count++
		} else if strings.Contains(img, `alt=""`) || strings.Contains(img, `alt=''`) {
			// Empty alt is valid for decorative images, don't count
		}
	}

	return count
}

type formLabelResult struct {
	totalInputs        int
	inputsWithoutLabel int
}

func (a *AccessibilityAnalyzer) checkFormLabels(html string) formLabelResult {
	result := formLabelResult{}

	// Find all input elements (except hidden, submit, button)
	inputRe := regexp.MustCompile(`<input[^>]*type=["']?(text|email|password|tel|number|search|url|date|time)["']?[^>]*>`)
	inputs := inputRe.FindAllString(html, -1)
	result.totalInputs = len(inputs)

	// Also count textareas and selects
	result.totalInputs += strings.Count(html, "<textarea")
	result.totalInputs += strings.Count(html, "<select")

	// Check for labels (simplified - real check would match for/id)
	labelCount := strings.Count(html, "<label")
	ariaLabelCount := len(regexp.MustCompile(`aria-label=`).FindAllString(html, -1))
	ariaLabelledByCount := len(regexp.MustCompile(`aria-labelledby=`).FindAllString(html, -1))

	totalLabels := labelCount + ariaLabelCount + ariaLabelledByCount
	if result.totalInputs > totalLabels {
		result.inputsWithoutLabel = result.totalInputs - totalLabels
	}

	return result
}

type headingResult struct {
	levels []int
	skips  []string
}

func (a *AccessibilityAnalyzer) checkHeadingStructure(html string) headingResult {
	result := headingResult{
		levels: make([]int, 0),
		skips:  make([]string, 0),
	}

	// Find all headings
	headingRe := regexp.MustCompile(`<h([1-6])[^>]*>`)
	matches := headingRe.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			level := int(match[1][0] - '0')
			result.levels = append(result.levels, level)
		}
	}

	// Check for skips
	if len(result.levels) > 0 {
		prevLevel := 0
		for _, level := range result.levels {
			if prevLevel > 0 && level > prevLevel+1 {
				result.skips = append(result.skips, fmt.Sprintf("H%d to H%d", prevLevel, level))
			}
			prevLevel = level
		}

		// Check if first heading is not H1
		if result.levels[0] != 1 {
			result.skips = append(result.skips, fmt.Sprintf("First heading is H%d, not H1", result.levels[0]))
		}
	}

	return result
}

type ariaResult struct {
	issueCount int
	issues     []string
}

func (a *AccessibilityAnalyzer) checkARIA(html string) ariaResult {
	result := ariaResult{
		issues: make([]string, 0),
	}

	// Check for invalid ARIA roles
	roleRe := regexp.MustCompile(`role=["']([^"']+)["']`)
	roles := roleRe.FindAllStringSubmatch(html, -1)

	validRoles := map[string]bool{
		"alert": true, "alertdialog": true, "application": true, "article": true,
		"banner": true, "button": true, "checkbox": true, "complementary": true,
		"contentinfo": true, "dialog": true, "document": true, "feed": true,
		"figure": true, "form": true, "grid": true, "gridcell": true,
		"group": true, "heading": true, "img": true, "link": true,
		"list": true, "listbox": true, "listitem": true, "main": true,
		"menu": true, "menubar": true, "menuitem": true, "navigation": true,
		"none": true, "note": true, "option": true, "presentation": true,
		"progressbar": true, "radio": true, "region": true, "row": true,
		"rowgroup": true, "scrollbar": true, "search": true, "searchbox": true,
		"separator": true, "slider": true, "spinbutton": true, "status": true,
		"switch": true, "tab": true, "table": true, "tablist": true,
		"tabpanel": true, "textbox": true, "timer": true, "toolbar": true,
		"tooltip": true, "tree": true, "treegrid": true, "treeitem": true,
	}

	for _, match := range roles {
		if len(match) >= 2 {
			role := strings.ToLower(match[1])
			if !validRoles[role] {
				result.issueCount++
				result.issues = append(result.issues, fmt.Sprintf("Invalid role: %s", role))
			}
		}
	}

	// Check for aria-hidden on focusable elements (simplified)
	if strings.Contains(html, `aria-hidden="true"`) {
		// This is a simplified check
		// Real implementation would check if focusable elements are inside
	}

	return result
}

func (a *AccessibilityAnalyzer) checkLangAttribute(html string) bool {
	return regexp.MustCompile(`<html[^>]*lang=["'][^"']+["']`).MatchString(html)
}

func (a *AccessibilityAnalyzer) checkSkipLink(html string) bool {
	// Check for skip link patterns
	return strings.Contains(html, "skip-to-content") ||
		strings.Contains(html, "skip-link") ||
		strings.Contains(html, "skip to content") ||
		strings.Contains(html, "skip to main") ||
		regexp.MustCompile(`<a[^>]*href=["']#(main|content|maincontent)["']`).MatchString(html)
}

func (a *AccessibilityAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["images_without_alt"]),
		fmt.Sprintf("%v", result.Data["form_labels_status"]),
		fmt.Sprintf("%v", result.Data["heading_status"]),
		fmt.Sprintf("%v", result.Data["aria_issues"]),
		fmt.Sprintf("%v", result.Data["score"]),
	}
}
