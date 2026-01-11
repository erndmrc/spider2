package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// ExtractionType represents the type of extraction to perform.
type ExtractionType string

const (
	ExtractionTypeRegex    ExtractionType = "regex"
	ExtractionTypeCSS      ExtractionType = "css"
	ExtractionTypeXPath    ExtractionType = "xpath"
	ExtractionTypeJSONPath ExtractionType = "jsonpath"
)

// ExtractionRule defines a custom extraction rule.
type ExtractionRule struct {
	ID           string
	Name         string
	Type         ExtractionType
	Pattern      string
	ExtractGroup int    // For regex: which capture group to extract (0 = full match)
	Attribute    string // For CSS: which attribute to extract (empty = text content)
	compiledRe   *regexp.Regexp
}

// ExtractionResult holds the extracted data for a rule.
type ExtractionResult struct {
	RuleID   string
	RuleName string
	Values   []string
}

// CustomExtractionAnalyzer extracts custom data from pages.
type CustomExtractionAnalyzer struct {
	rules []*ExtractionRule
}

func NewCustomExtractionAnalyzer() *CustomExtractionAnalyzer {
	return &CustomExtractionAnalyzer{
		rules: make([]*ExtractionRule, 0),
	}
}

func (a *CustomExtractionAnalyzer) Name() string {
	return "Custom Extraction"
}

func (a *CustomExtractionAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 250, Sortable: true, DataKey: "url"},
		{ID: "rule_name", Title: "Extraction Rule", Width: 150, Sortable: true, DataKey: "rule_name"},
		{ID: "extracted_count", Title: "Count", Width: 60, Sortable: true, DataKey: "extracted_count"},
		{ID: "first_value", Title: "First Value", Width: 200, Sortable: true, DataKey: "first_value"},
		{ID: "all_values", Title: "All Values", Width: 300, Sortable: false, DataKey: "all_values"},
	}
}

func (a *CustomExtractionAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "has_extraction", Label: "Has Data", Description: "Pages with extracted data", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["total_extractions"].(int); ok {
				return count > 0
			}
			return false
		}},
		{ID: "no_extraction", Label: "No Data", Description: "Pages without extracted data", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["total_extractions"].(int); ok {
				return count == 0
			}
			return true
		}},
	}
}

// AddRule adds a new extraction rule.
func (a *CustomExtractionAnalyzer) AddRule(rule *ExtractionRule) error {
	// Compile regex if needed
	if rule.Type == ExtractionTypeRegex {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		rule.compiledRe = re
	}

	a.rules = append(a.rules, rule)
	return nil
}

// RemoveRule removes an extraction rule by ID.
func (a *CustomExtractionAnalyzer) RemoveRule(ruleID string) {
	newRules := make([]*ExtractionRule, 0)
	for _, rule := range a.rules {
		if rule.ID != ruleID {
			newRules = append(newRules, rule)
		}
	}
	a.rules = newRules
}

// ClearRules removes all extraction rules.
func (a *CustomExtractionAnalyzer) ClearRules() {
	a.rules = make([]*ExtractionRule, 0)
}

// GetRules returns all configured rules.
func (a *CustomExtractionAnalyzer) GetRules() []*ExtractionRule {
	return a.rules
}

func (a *CustomExtractionAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL
	result.Data["total_extractions"] = 0

	if len(a.rules) == 0 {
		result.Data["rule_name"] = "No rules configured"
		return result
	}

	htmlContent := ""
	if ctx.RawHTML != nil {
		htmlContent = string(ctx.RawHTML)
	}

	totalExtractions := 0
	extractionResults := make([]*ExtractionResult, 0)

	for _, rule := range a.rules {
		values := a.extractWithRule(rule, htmlContent)

		er := &ExtractionResult{
			RuleID:   rule.ID,
			RuleName: rule.Name,
			Values:   values,
		}
		extractionResults = append(extractionResults, er)
		totalExtractions += len(values)
	}

	result.Data["total_extractions"] = totalExtractions
	result.Data["extraction_results"] = extractionResults

	// Set first rule result for display
	if len(extractionResults) > 0 {
		result.Data["rule_name"] = extractionResults[0].RuleName
		result.Data["extracted_count"] = len(extractionResults[0].Values)
		if len(extractionResults[0].Values) > 0 {
			firstValue := extractionResults[0].Values[0]
			if len(firstValue) > 100 {
				firstValue = firstValue[:100] + "..."
			}
			result.Data["first_value"] = firstValue
			result.Data["all_values"] = strings.Join(extractionResults[0].Values, " | ")
		}
	}

	// Store all extracted data as JSON for export
	extractedData := make(map[string][]string)
	for _, er := range extractionResults {
		extractedData[er.RuleName] = er.Values
	}
	if jsonData, err := json.Marshal(extractedData); err == nil {
		result.Data["extracted_json"] = string(jsonData)
	}

	return result
}

func (a *CustomExtractionAnalyzer) extractWithRule(rule *ExtractionRule, html string) []string {
	values := make([]string, 0)

	switch rule.Type {
	case ExtractionTypeRegex:
		values = a.extractRegex(rule, html)
	case ExtractionTypeCSS:
		values = a.extractCSS(rule, html)
	case ExtractionTypeXPath:
		values = a.extractXPath(rule, html)
	case ExtractionTypeJSONPath:
		values = a.extractJSONPath(rule, html)
	}

	return values
}

func (a *CustomExtractionAnalyzer) extractRegex(rule *ExtractionRule, html string) []string {
	values := make([]string, 0)

	if rule.compiledRe == nil {
		return values
	}

	matches := rule.compiledRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if rule.ExtractGroup >= 0 && rule.ExtractGroup < len(match) {
			value := strings.TrimSpace(match[rule.ExtractGroup])
			if value != "" {
				values = append(values, value)
			}
		} else if len(match) > 0 {
			value := strings.TrimSpace(match[0])
			if value != "" {
				values = append(values, value)
			}
		}
	}

	return values
}

func (a *CustomExtractionAnalyzer) extractCSS(rule *ExtractionRule, html string) []string {
	values := make([]string, 0)
	selector := strings.TrimSpace(rule.Pattern)

	// Simple CSS selector extraction
	// Supports: tag, .class, #id, tag[attr]
	var tagPattern string
	var attrToExtract string

	// Check for attribute selector: tag[attr]
	if strings.Contains(selector, "[") && strings.Contains(selector, "]") {
		parts := strings.Split(selector, "[")
		tagPattern = parts[0]
		attrPart := strings.TrimSuffix(parts[1], "]")
		attrToExtract = attrPart
	} else {
		tagPattern = selector
		attrToExtract = rule.Attribute
	}

	// Build regex based on selector type
	var re *regexp.Regexp
	var contentRe *regexp.Regexp

	if strings.HasPrefix(tagPattern, "#") {
		// ID selector
		id := tagPattern[1:]
		re = regexp.MustCompile(fmt.Sprintf(`<([a-zA-Z0-9]+)[^>]*id=["']%s["'][^>]*>([\s\S]*?)</\1>`, regexp.QuoteMeta(id)))
	} else if strings.HasPrefix(tagPattern, ".") {
		// Class selector
		class := tagPattern[1:]
		re = regexp.MustCompile(fmt.Sprintf(`<([a-zA-Z0-9]+)[^>]*class=["'][^"']*\b%s\b[^"']*["'][^>]*>([\s\S]*?)</\1>`, regexp.QuoteMeta(class)))
	} else if tagPattern != "" {
		// Tag selector
		re = regexp.MustCompile(fmt.Sprintf(`<%s([^>]*)>([\s\S]*?)</%s>`, regexp.QuoteMeta(tagPattern), regexp.QuoteMeta(tagPattern)))
		// Also match self-closing tags for attribute extraction
		contentRe = regexp.MustCompile(fmt.Sprintf(`<%s([^>]*)/?>`, regexp.QuoteMeta(tagPattern)))
	}

	if re == nil && contentRe == nil {
		return values
	}

	// Extract based on what we need
	if attrToExtract != "" {
		// Extract attribute value
		attrRe := regexp.MustCompile(fmt.Sprintf(`%s=["']([^"']*)["']`, regexp.QuoteMeta(attrToExtract)))

		if re != nil {
			matches := re.FindAllStringSubmatch(html, -1)
			for _, match := range matches {
				if len(match) > 1 {
					attrMatches := attrRe.FindStringSubmatch(match[0])
					if len(attrMatches) > 1 {
						values = append(values, attrMatches[1])
					}
				}
			}
		}
		if contentRe != nil {
			matches := contentRe.FindAllStringSubmatch(html, -1)
			for _, match := range matches {
				if len(match) > 1 {
					attrMatches := attrRe.FindStringSubmatch(match[0])
					if len(attrMatches) > 1 {
						values = append(values, attrMatches[1])
					}
				}
			}
		}
	} else {
		// Extract text content
		if re != nil {
			matches := re.FindAllStringSubmatch(html, -1)
			for _, match := range matches {
				if len(match) > 2 {
					// Get text content (last capture group)
					content := match[len(match)-1]
					// Strip nested HTML tags
					content = stripHTMLTags(content)
					if content != "" {
						values = append(values, content)
					}
				}
			}
		}
	}

	return values
}

func (a *CustomExtractionAnalyzer) extractXPath(rule *ExtractionRule, html string) []string {
	values := make([]string, 0)

	// Simplified XPath support
	// Full XPath would require a proper XML/HTML parser
	xpath := rule.Pattern

	// Simple XPath patterns:
	// //tag - all tags
	// //tag/@attr - attribute of all tags
	// //tag/text() - text content of all tags

	xpath = strings.TrimPrefix(xpath, "//")

	if strings.Contains(xpath, "/@") {
		// Attribute extraction: //tag/@attr
		parts := strings.Split(xpath, "/@")
		tag := parts[0]
		attr := parts[1]

		re := regexp.MustCompile(fmt.Sprintf(`<%s[^>]*%s=["']([^"']*)["']`, regexp.QuoteMeta(tag), regexp.QuoteMeta(attr)))
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			if len(match) > 1 {
				values = append(values, match[1])
			}
		}
	} else if strings.HasSuffix(xpath, "/text()") {
		// Text content: //tag/text()
		tag := strings.TrimSuffix(xpath, "/text()")
		re := regexp.MustCompile(fmt.Sprintf(`<%s[^>]*>([\s\S]*?)</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			if len(match) > 1 {
				content := stripHTMLTags(match[1])
				if content != "" {
					values = append(values, content)
				}
			}
		}
	} else {
		// Just tag - extract full element
		tag := xpath
		re := regexp.MustCompile(fmt.Sprintf(`<%s[^>]*>([\s\S]*?)</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			if len(match) > 0 {
				values = append(values, match[0])
			}
		}
	}

	return values
}

func (a *CustomExtractionAnalyzer) extractJSONPath(rule *ExtractionRule, html string) []string {
	values := make([]string, 0)

	// Find JSON-LD blocks in HTML
	jsonRe := regexp.MustCompile(`<script[^>]*type=["']application/ld\+json["'][^>]*>([\s\S]*?)</script>`)
	jsonBlocks := jsonRe.FindAllStringSubmatch(html, -1)

	for _, block := range jsonBlocks {
		if len(block) < 2 {
			continue
		}

		jsonStr := strings.TrimSpace(block[1])

		// Parse JSON
		var data interface{}
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue
		}

		// Simple JSONPath extraction
		// Supports: $.key, $.key.subkey, $[0].key
		extracted := a.extractJSONPathValue(data, rule.Pattern)
		values = append(values, extracted...)
	}

	return values
}

func (a *CustomExtractionAnalyzer) extractJSONPathValue(data interface{}, path string) []string {
	values := make([]string, 0)

	// Remove leading $. or $
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")

	if path == "" {
		if str, ok := data.(string); ok {
			values = append(values, str)
		} else if jsonBytes, err := json.Marshal(data); err == nil {
			values = append(values, string(jsonBytes))
		}
		return values
	}

	// Split path into parts
	parts := strings.Split(path, ".")

	current := data
	for _, part := range parts {
		if current == nil {
			break
		}

		// Handle array index: [0], [1], etc.
		if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
			indexStr := strings.TrimPrefix(strings.TrimSuffix(part, "]"), "[")
			if arr, ok := current.([]interface{}); ok {
				if indexStr == "*" {
					// All elements
					for _, item := range arr {
						if str, ok := item.(string); ok {
							values = append(values, str)
						}
					}
					return values
				}
				var index int
				fmt.Sscanf(indexStr, "%d", &index)
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					current = nil
				}
			} else {
				current = nil
			}
		} else if obj, ok := current.(map[string]interface{}); ok {
			current = obj[part]
		} else {
			current = nil
		}
	}

	if current != nil {
		if str, ok := current.(string); ok {
			values = append(values, str)
		} else if num, ok := current.(float64); ok {
			values = append(values, fmt.Sprintf("%v", num))
		} else if arr, ok := current.([]interface{}); ok {
			for _, item := range arr {
				if str, ok := item.(string); ok {
					values = append(values, str)
				}
			}
		}
	}

	return values
}

// ExpandExtractionResults expands results to show one row per rule.
func (a *CustomExtractionAnalyzer) ExpandExtractionResults(ctx *AnalysisContext) []*AnalysisResult {
	results := make([]*AnalysisResult, 0)

	mainResult := a.Analyze(ctx)
	extractionResults, ok := mainResult.Data["extraction_results"].([]*ExtractionResult)
	if !ok {
		return results
	}

	for _, er := range extractionResults {
		firstValue := ""
		if len(er.Values) > 0 {
			firstValue = er.Values[0]
			if len(firstValue) > 100 {
				firstValue = firstValue[:100] + "..."
			}
		}

		result := &AnalysisResult{
			URLID:  ctx.URL.ID,
			Issues: make([]*storage.Issue, 0),
			Data: map[string]interface{}{
				"url":             ctx.URL.URL,
				"rule_name":       er.RuleName,
				"extracted_count": len(er.Values),
				"first_value":     firstValue,
				"all_values":      strings.Join(er.Values, " | "),
			},
		}
		results = append(results, result)
	}

	return results
}

func (a *CustomExtractionAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["rule_name"]),
		fmt.Sprintf("%v", result.Data["extracted_count"]),
		fmt.Sprintf("%v", result.Data["first_value"]),
		fmt.Sprintf("%v", result.Data["all_values"]),
	}
}
