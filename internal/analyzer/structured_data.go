package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// StructuredDataAnalyzer analyzes structured data (JSON-LD, Microdata).
type StructuredDataAnalyzer struct{}

func NewStructuredDataAnalyzer() *StructuredDataAnalyzer {
	return &StructuredDataAnalyzer{}
}

func (a *StructuredDataAnalyzer) Name() string {
	return "Structured Data"
}

func (a *StructuredDataAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "type", Title: "Schema Type", Width: 150, Sortable: true, DataKey: "type"},
		{ID: "format", Title: "Format", Width: 100, Sortable: true, DataKey: "format"},
		{ID: "valid", Title: "Valid", Width: 60, Sortable: true, DataKey: "valid"},
		{ID: "errors", Title: "Errors", Width: 200, Sortable: true, DataKey: "errors"},
		{ID: "warnings", Title: "Warnings", Width: 200, Sortable: true, DataKey: "warnings"},
	}
}

func (a *StructuredDataAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "has_sd", Label: "Contains Structured Data", Description: "Pages with structured data", FilterFunc: func(r *AnalysisResult) bool {
			if count, ok := r.Data["schema_count"].(int); ok {
				return count > 0
			}
			return false
		}},
		{ID: "json_ld", Label: "JSON-LD", Description: "Pages with JSON-LD", FilterFunc: func(r *AnalysisResult) bool {
			if format, ok := r.Data["format"].(string); ok {
				return format == "JSON-LD"
			}
			return false
		}},
		{ID: "microdata", Label: "Microdata", Description: "Pages with Microdata", FilterFunc: func(r *AnalysisResult) bool {
			if format, ok := r.Data["format"].(string); ok {
				return format == "Microdata"
			}
			return false
		}},
		{ID: "errors", Label: "Has Errors", Description: "Structured data with errors", FilterFunc: func(r *AnalysisResult) bool {
			if valid, ok := r.Data["valid"].(bool); ok {
				return !valid
			}
			return false
		}},
		{ID: "warnings", Label: "Has Warnings", Description: "Structured data with warnings", FilterFunc: func(r *AnalysisResult) bool {
			if warnings, ok := r.Data["warning_count"].(int); ok {
				return warnings > 0
			}
			return false
		}},
	}
}

// SchemaItem represents a single structured data item.
type SchemaItem struct {
	Type     string                 `json:"type"`
	Format   string                 `json:"format"` // JSON-LD, Microdata, RDFa
	Valid    bool                   `json:"valid"`
	Errors   []string               `json:"errors"`
	Warnings []string               `json:"warnings"`
	Data     map[string]interface{} `json:"data"`
}

func (a *StructuredDataAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL
	result.Data["schema_count"] = 0
	result.Data["schemas"] = make([]SchemaItem, 0)

	if ctx.RawHTML == nil {
		return result
	}

	htmlStr := string(ctx.RawHTML)
	schemas := make([]SchemaItem, 0)

	// Extract JSON-LD
	jsonLDSchemas := a.extractJSONLD(htmlStr)
	schemas = append(schemas, jsonLDSchemas...)

	// Detect Microdata
	if strings.Contains(htmlStr, "itemscope") && strings.Contains(htmlStr, "itemtype") {
		microdataTypes := a.extractMicrodataTypes(htmlStr)
		for _, t := range microdataTypes {
			schemas = append(schemas, SchemaItem{
				Type:   t,
				Format: "Microdata",
				Valid:  true, // Basic validation only
			})
		}
	}

	result.Data["schema_count"] = len(schemas)
	result.Data["schemas"] = schemas

	if len(schemas) > 0 {
		// Set primary schema info for display
		result.Data["type"] = schemas[0].Type
		result.Data["format"] = schemas[0].Format
		result.Data["valid"] = schemas[0].Valid

		// Collect all errors/warnings
		var allErrors, allWarnings []string
		for _, s := range schemas {
			allErrors = append(allErrors, s.Errors...)
			allWarnings = append(allWarnings, s.Warnings...)
		}
		result.Data["errors"] = strings.Join(allErrors, "; ")
		result.Data["warnings"] = strings.Join(allWarnings, "; ")
		result.Data["error_count"] = len(allErrors)
		result.Data["warning_count"] = len(allWarnings)

		// Generate issues for errors
		for _, err := range allErrors {
			result.Issues = append(result.Issues, NewIssue(
				ctx.URL.ID,
				"structured_data_error",
				storage.IssueTypeError,
				storage.SeverityMedium,
				"structured_data",
				fmt.Sprintf("Structured data error: %s", err),
			))
		}
	} else {
		result.Data["type"] = ""
		result.Data["format"] = ""
		result.Data["valid"] = true
		result.Data["errors"] = ""
		result.Data["warnings"] = ""
	}

	return result
}

// extractJSONLD extracts and parses JSON-LD scripts.
func (a *StructuredDataAnalyzer) extractJSONLD(html string) []SchemaItem {
	schemas := make([]SchemaItem, 0)

	// Find all JSON-LD script tags
	re := regexp.MustCompile(`<script[^>]*type=["']application/ld\+json["'][^>]*>([\s\S]*?)</script>`)
	matches := re.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		jsonContent := strings.TrimSpace(match[1])
		schema := SchemaItem{
			Format: "JSON-LD",
			Valid:  true,
			Errors: make([]string, 0),
			Warnings: make([]string, 0),
		}

		// Try to parse JSON
		var data interface{}
		if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
			schema.Valid = false
			schema.Errors = append(schema.Errors, fmt.Sprintf("JSON parse error: %s", err.Error()))
			schema.Type = "Unknown (parse error)"
			schemas = append(schemas, schema)
			continue
		}

		// Extract type(s)
		types := a.extractSchemaTypes(data)
		if len(types) > 0 {
			schema.Type = strings.Join(types, ", ")
		} else {
			schema.Type = "Unknown"
		}

		// Store parsed data
		if dataMap, ok := data.(map[string]interface{}); ok {
			schema.Data = dataMap
			// Basic validation
			a.validateSchema(&schema, dataMap)
		}

		schemas = append(schemas, schema)
	}

	return schemas
}

// extractSchemaTypes recursively extracts @type values.
func (a *StructuredDataAnalyzer) extractSchemaTypes(data interface{}) []string {
	types := make([]string, 0)

	switch v := data.(type) {
	case map[string]interface{}:
		if t, ok := v["@type"]; ok {
			switch tt := t.(type) {
			case string:
				types = append(types, tt)
			case []interface{}:
				for _, item := range tt {
					if str, ok := item.(string); ok {
						types = append(types, str)
					}
				}
			}
		}
		// Check @graph
		if graph, ok := v["@graph"].([]interface{}); ok {
			for _, item := range graph {
				types = append(types, a.extractSchemaTypes(item)...)
			}
		}
	case []interface{}:
		for _, item := range v {
			types = append(types, a.extractSchemaTypes(item)...)
		}
	}

	return types
}

// extractMicrodataTypes extracts itemtype values from Microdata.
func (a *StructuredDataAnalyzer) extractMicrodataTypes(html string) []string {
	types := make([]string, 0)

	re := regexp.MustCompile(`itemtype=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(html, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 2 {
			typeURL := match[1]
			// Extract type name from URL
			typeName := typeURL
			if strings.Contains(typeURL, "schema.org/") {
				parts := strings.Split(typeURL, "/")
				typeName = parts[len(parts)-1]
			}
			if !seen[typeName] {
				types = append(types, typeName)
				seen[typeName] = true
			}
		}
	}

	return types
}

// validateSchema performs basic validation on schema data.
func (a *StructuredDataAnalyzer) validateSchema(schema *SchemaItem, data map[string]interface{}) {
	schemaType := ""
	if t, ok := data["@type"].(string); ok {
		schemaType = t
	}

	// Common required fields by type
	switch schemaType {
	case "Article", "NewsArticle", "BlogPosting":
		a.checkRequired(schema, data, []string{"headline", "author", "datePublished"})
	case "Product":
		a.checkRequired(schema, data, []string{"name"})
		// Check for offers
		if _, ok := data["offers"]; !ok {
			schema.Warnings = append(schema.Warnings, "Product missing 'offers'")
		}
	case "LocalBusiness", "Organization":
		a.checkRequired(schema, data, []string{"name"})
	case "BreadcrumbList":
		if _, ok := data["itemListElement"]; !ok {
			schema.Errors = append(schema.Errors, "BreadcrumbList missing 'itemListElement'")
			schema.Valid = false
		}
	case "FAQPage":
		if _, ok := data["mainEntity"]; !ok {
			schema.Errors = append(schema.Errors, "FAQPage missing 'mainEntity'")
			schema.Valid = false
		}
	case "WebPage":
		// Minimal requirements
	case "WebSite":
		a.checkRequired(schema, data, []string{"name", "url"})
	}
}

func (a *StructuredDataAnalyzer) checkRequired(schema *SchemaItem, data map[string]interface{}, fields []string) {
	for _, field := range fields {
		if _, ok := data[field]; !ok {
			schema.Warnings = append(schema.Warnings, fmt.Sprintf("Missing recommended field '%s'", field))
		}
	}
}

func (a *StructuredDataAnalyzer) ExportRow(result *AnalysisResult) []string {
	valid := "Yes"
	if v, ok := result.Data["valid"].(bool); ok && !v {
		valid = "No"
	}
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["type"]),
		fmt.Sprintf("%v", result.Data["format"]),
		valid,
		fmt.Sprintf("%v", result.Data["errors"]),
		fmt.Sprintf("%v", result.Data["warnings"]),
	}
}
