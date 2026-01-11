package analyzer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// SearchType represents the type of search to perform.
type SearchType string

const (
	SearchTypeText     SearchType = "text"
	SearchTypeRegex    SearchType = "regex"
	SearchTypeContains SearchType = "contains"
	SearchTypeCSS      SearchType = "css" // CSS selector based
)

// SearchRule defines a custom search rule.
type SearchRule struct {
	ID          string
	Name        string
	Type        SearchType
	Pattern     string
	CaseSensitive bool
	SearchIn    string // "html", "text", "url", "title", "meta"
	compiledRe  *regexp.Regexp
}

// CustomSearchAnalyzer performs custom searches in page content.
type CustomSearchAnalyzer struct {
	rules []*SearchRule
}

func NewCustomSearchAnalyzer() *CustomSearchAnalyzer {
	return &CustomSearchAnalyzer{
		rules: make([]*SearchRule, 0),
	}
}

func (a *CustomSearchAnalyzer) Name() string {
	return "Custom Search"
}

func (a *CustomSearchAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 250, Sortable: true, DataKey: "url"},
		{ID: "rule_name", Title: "Search Rule", Width: 150, Sortable: true, DataKey: "rule_name"},
		{ID: "matches", Title: "Matches", Width: 80, Sortable: true, DataKey: "match_count"},
		{ID: "first_match", Title: "First Match", Width: 200, Sortable: true, DataKey: "first_match"},
		{ID: "context", Title: "Context", Width: 250, Sortable: false, DataKey: "context"},
	}
}

func (a *CustomSearchAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "has_matches", Label: "Has Matches", Description: "Pages with matches", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["total_matches"].(int); ok {
				return count > 0
			}
			return false
		}},
		{ID: "no_matches", Label: "No Matches", Description: "Pages without matches", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["total_matches"].(int); ok {
				return count == 0
			}
			return true
		}},
	}
}

// AddRule adds a new search rule.
func (a *CustomSearchAnalyzer) AddRule(rule *SearchRule) error {
	// Compile regex if needed
	if rule.Type == SearchTypeRegex {
		flags := ""
		if !rule.CaseSensitive {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + rule.Pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		rule.compiledRe = re
	}

	a.rules = append(a.rules, rule)
	return nil
}

// RemoveRule removes a search rule by ID.
func (a *CustomSearchAnalyzer) RemoveRule(ruleID string) {
	newRules := make([]*SearchRule, 0)
	for _, rule := range a.rules {
		if rule.ID != ruleID {
			newRules = append(newRules, rule)
		}
	}
	a.rules = newRules
}

// ClearRules removes all search rules.
func (a *CustomSearchAnalyzer) ClearRules() {
	a.rules = make([]*SearchRule, 0)
}

// GetRules returns all configured rules.
func (a *CustomSearchAnalyzer) GetRules() []*SearchRule {
	return a.rules
}

func (a *CustomSearchAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL
	result.Data["total_matches"] = 0

	if len(a.rules) == 0 {
		result.Data["rule_name"] = "No rules configured"
		return result
	}

	// Get content to search
	htmlContent := ""
	textContent := ""
	if ctx.RawHTML != nil {
		htmlContent = string(ctx.RawHTML)
		textContent = stripHTMLTags(htmlContent)
	}

	totalMatches := 0
	ruleResults := make([]map[string]interface{}, 0)

	for _, rule := range a.rules {
		matches := a.searchWithRule(rule, ctx, htmlContent, textContent)

		ruleResult := map[string]interface{}{
			"rule_id":     rule.ID,
			"rule_name":   rule.Name,
			"match_count": len(matches),
			"matches":     matches,
		}

		if len(matches) > 0 {
			ruleResult["first_match"] = matches[0]
			if len(matches[0]) > 100 {
				ruleResult["first_match"] = matches[0][:100] + "..."
			}

			// Get context around first match
			context := a.getMatchContext(htmlContent, textContent, rule, matches[0])
			ruleResult["context"] = context
		}

		ruleResults = append(ruleResults, ruleResult)
		totalMatches += len(matches)
	}

	result.Data["total_matches"] = totalMatches
	result.Data["rule_results"] = ruleResults

	// Set first rule result for display
	if len(ruleResults) > 0 {
		result.Data["rule_name"] = ruleResults[0]["rule_name"]
		result.Data["match_count"] = ruleResults[0]["match_count"]
		if fm, ok := ruleResults[0]["first_match"]; ok {
			result.Data["first_match"] = fm
		}
		if ctx, ok := ruleResults[0]["context"]; ok {
			result.Data["context"] = ctx
		}
	}

	return result
}

func (a *CustomSearchAnalyzer) searchWithRule(rule *SearchRule, ctx *AnalysisContext, htmlContent, textContent string) []string {
	matches := make([]string, 0)

	// Determine content to search
	var searchContent string
	switch rule.SearchIn {
	case "html":
		searchContent = htmlContent
	case "text":
		searchContent = textContent
	case "url":
		searchContent = ctx.URL.URL
	case "title":
		if ctx.HTMLFeatures != nil {
			searchContent = ctx.HTMLFeatures.Title
		}
	case "meta":
		if ctx.HTMLFeatures != nil {
			searchContent = ctx.HTMLFeatures.MetaDescription
		}
	default:
		searchContent = htmlContent
	}

	if searchContent == "" {
		return matches
	}

	switch rule.Type {
	case SearchTypeText:
		matches = a.searchText(searchContent, rule.Pattern, rule.CaseSensitive)
	case SearchTypeContains:
		if a.containsPattern(searchContent, rule.Pattern, rule.CaseSensitive) {
			matches = append(matches, rule.Pattern)
		}
	case SearchTypeRegex:
		if rule.compiledRe != nil {
			found := rule.compiledRe.FindAllString(searchContent, -1)
			matches = append(matches, found...)
		}
	case SearchTypeCSS:
		// CSS selector search would require DOM parsing
		// For now, we'll do a simple tag-based search
		matches = a.searchCSSSelector(htmlContent, rule.Pattern)
	}

	return matches
}

func (a *CustomSearchAnalyzer) searchText(content, pattern string, caseSensitive bool) []string {
	matches := make([]string, 0)

	if !caseSensitive {
		content = strings.ToLower(content)
		pattern = strings.ToLower(pattern)
	}

	idx := 0
	for {
		pos := strings.Index(content[idx:], pattern)
		if pos == -1 {
			break
		}
		matches = append(matches, pattern)
		idx += pos + len(pattern)
		if idx >= len(content) {
			break
		}
	}

	return matches
}

func (a *CustomSearchAnalyzer) containsPattern(content, pattern string, caseSensitive bool) bool {
	if !caseSensitive {
		content = strings.ToLower(content)
		pattern = strings.ToLower(pattern)
	}
	return strings.Contains(content, pattern)
}

func (a *CustomSearchAnalyzer) searchCSSSelector(html, selector string) []string {
	matches := make([]string, 0)

	// Simple CSS selector parsing
	// Supports: tag, .class, #id, tag.class, tag#id
	selector = strings.TrimSpace(selector)

	if strings.HasPrefix(selector, "#") {
		// ID selector
		id := selector[1:]
		re := regexp.MustCompile(fmt.Sprintf(`id=["']%s["']`, regexp.QuoteMeta(id)))
		found := re.FindAllString(html, -1)
		matches = append(matches, found...)
	} else if strings.HasPrefix(selector, ".") {
		// Class selector
		class := selector[1:]
		re := regexp.MustCompile(fmt.Sprintf(`class=["'][^"']*\b%s\b[^"']*["']`, regexp.QuoteMeta(class)))
		found := re.FindAllString(html, -1)
		matches = append(matches, found...)
	} else {
		// Tag selector (possibly with class or id)
		parts := strings.Split(selector, ".")
		tag := parts[0]

		if strings.Contains(tag, "#") {
			tagParts := strings.Split(tag, "#")
			tag = tagParts[0]
		}

		if tag != "" {
			re := regexp.MustCompile(fmt.Sprintf(`<%s[^>]*>`, regexp.QuoteMeta(tag)))
			found := re.FindAllString(html, -1)
			matches = append(matches, found...)
		}
	}

	return matches
}

func (a *CustomSearchAnalyzer) getMatchContext(html, text, rule *SearchRule, match string) string {
	content := text
	if rule.SearchIn == "html" {
		content = html
	}

	idx := strings.Index(strings.ToLower(content), strings.ToLower(match))
	if idx == -1 {
		return ""
	}

	// Get context around match (50 chars before and after)
	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := idx + len(match) + 50
	if end > len(content) {
		end = len(content)
	}

	context := content[start:end]
	if start > 0 {
		context = "..." + context
	}
	if end < len(content) {
		context = context + "..."
	}

	return context
}

// stripHTMLTags removes HTML tags from content
func stripHTMLTags(html string) string {
	// Remove script and style contents
	reScript := regexp.MustCompile(`<script[^>]*>[\s\S]*?</script>`)
	html = reScript.ReplaceAllString(html, "")

	reStyle := regexp.MustCompile(`<style[^>]*>[\s\S]*?</style>`)
	html = reStyle.ReplaceAllString(html, "")

	// Remove HTML tags
	reTag := regexp.MustCompile(`<[^>]+>`)
	text := reTag.ReplaceAllString(html, " ")

	// Clean up whitespace
	reSpace := regexp.MustCompile(`\s+`)
	text = reSpace.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// ExpandSearchResults expands results to show one row per rule match.
func (a *CustomSearchAnalyzer) ExpandSearchResults(ctx *AnalysisContext) []*AnalysisResult {
	results := make([]*AnalysisResult, 0)

	mainResult := a.Analyze(ctx)
	ruleResults, ok := mainResult.Data["rule_results"].([]map[string]interface{})
	if !ok {
		return results
	}

	for _, rr := range ruleResults {
		matchCount, _ := rr["match_count"].(int)
		if matchCount == 0 {
			continue
		}

		result := &AnalysisResult{
			URLID:  ctx.URL.ID,
			Issues: make([]*storage.Issue, 0),
			Data: map[string]interface{}{
				"url":         ctx.URL.URL,
				"rule_name":   rr["rule_name"],
				"match_count": matchCount,
				"first_match": rr["first_match"],
				"context":     rr["context"],
			},
		}
		results = append(results, result)
	}

	return results
}

func (a *CustomSearchAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["rule_name"]),
		fmt.Sprintf("%v", result.Data["match_count"]),
		fmt.Sprintf("%v", result.Data["first_match"]),
		fmt.Sprintf("%v", result.Data["context"]),
	}
}
