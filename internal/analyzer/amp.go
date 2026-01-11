package analyzer

import (
	"fmt"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// AMPAnalyzer analyzes AMP (Accelerated Mobile Pages).
type AMPAnalyzer struct {
	ampPages map[int64]string // URL ID -> AMP URL
}

func NewAMPAnalyzer() *AMPAnalyzer {
	return &AMPAnalyzer{
		ampPages: make(map[int64]string),
	}
}

func (a *AMPAnalyzer) Name() string {
	return "AMP"
}

func (a *AMPAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "amp_url", Title: "AMP URL", Width: 300, Sortable: true, DataKey: "amp_url"},
		{ID: "has_amp", Title: "Has AMP", Width: 70, Sortable: true, DataKey: "has_amp"},
		{ID: "amp_status", Title: "AMP Status", Width: 80, Sortable: true, DataKey: "amp_status"},
		{ID: "canonical_match", Title: "Canonical Match", Width: 100, Sortable: true, DataKey: "canonical_match"},
	}
}

func (a *AMPAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "has_amp", Label: "Has AMP", Description: "Pages with AMP version", FilterFunc: func(r *AnalysisResult) bool {
			if hasAMP, ok := r.Data["has_amp"].(bool); ok {
				return hasAMP
			}
			return false
		}},
		{ID: "no_amp", Label: "No AMP", Description: "Pages without AMP version", FilterFunc: func(r *AnalysisResult) bool {
			if hasAMP, ok := r.Data["has_amp"].(bool); ok {
				return !hasAMP
			}
			return true
		}},
		{ID: "amp_broken", Label: "AMP Broken", Description: "AMP pages returning errors", FilterFunc: func(r *AnalysisResult) bool {
			if status, ok := r.Data["amp_status_code"].(int); ok {
				return status >= 400
			}
			return false
		}},
		{ID: "canonical_mismatch", Label: "Canonical Mismatch", Description: "AMP canonical doesn't match", FilterFunc: func(r *AnalysisResult) bool {
			if match, ok := r.Data["canonical_match"].(bool); ok {
				return !match
			}
			return false
		}},
	}
}

func (a *AMPAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL
	result.Data["has_amp"] = false
	result.Data["amp_url"] = ""

	if ctx.HTMLFeatures == nil {
		return result
	}

	// Check for AMP link in raw HTML (would need parser enhancement)
	// For now, check if page itself is AMP
	if ctx.RawHTML != nil {
		htmlStr := string(ctx.RawHTML)

		// Check if this page IS an AMP page
		isAMPPage := strings.Contains(htmlStr, "<html amp") ||
			strings.Contains(htmlStr, "<html ⚡") ||
			strings.Contains(htmlStr, `<html lang="` ) && strings.Contains(htmlStr, "⚡")

		result.Data["is_amp_page"] = isAMPPage

		// Look for amphtml link
		ampStart := strings.Index(htmlStr, `rel="amphtml"`)
		if ampStart == -1 {
			ampStart = strings.Index(htmlStr, `rel='amphtml'`)
		}

		if ampStart != -1 {
			// Find the href
			searchStart := ampStart - 200
			if searchStart < 0 {
				searchStart = 0
			}
			searchEnd := ampStart + 50
			if searchEnd > len(htmlStr) {
				searchEnd = len(htmlStr)
			}
			snippet := htmlStr[searchStart:searchEnd]

			hrefStart := strings.Index(snippet, `href="`)
			if hrefStart != -1 {
				hrefStart += 6
				hrefEnd := strings.Index(snippet[hrefStart:], `"`)
				if hrefEnd != -1 {
					ampURL := snippet[hrefStart : hrefStart+hrefEnd]
					result.Data["has_amp"] = true
					result.Data["amp_url"] = ampURL
					a.ampPages[ctx.URL.ID] = ampURL
				}
			}
		}
	}

	return result
}

// ValidateAMPCanonical validates that AMP page canonicals point back correctly.
func (a *AMPAnalyzer) ValidateAMPCanonical(ampURLID int64, ampCanonical string, originalURL string) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ampURLID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	canonicalMatch := ampCanonical == originalURL
	result.Data["canonical_match"] = canonicalMatch

	if !canonicalMatch {
		result.Issues = append(result.Issues, NewIssue(
			ampURLID,
			"amp_canonical_mismatch",
			storage.IssueTypeError,
			storage.SeverityHigh,
			"amp",
			fmt.Sprintf("AMP canonical (%s) doesn't match original page (%s)", ampCanonical, originalURL),
		))
	}

	return result
}

func (a *AMPAnalyzer) Reset() {
	a.ampPages = make(map[int64]string)
}

func (a *AMPAnalyzer) ExportRow(result *AnalysisResult) []string {
	hasAMP := "No"
	if h, ok := result.Data["has_amp"].(bool); ok && h {
		hasAMP = "Yes"
	}
	canonMatch := ""
	if m, ok := result.Data["canonical_match"].(bool); ok {
		if m {
			canonMatch = "Yes"
		} else {
			canonMatch = "No"
		}
	}
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["amp_url"]),
		hasAMP,
		fmt.Sprintf("%v", result.Data["amp_status"]),
		canonMatch,
	}
}
