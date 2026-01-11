package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/spider-crawler/spider/internal/storage"
)

// ResponseCodesAnalyzer analyzes HTTP response codes.
type ResponseCodesAnalyzer struct{}

func NewResponseCodesAnalyzer() *ResponseCodesAnalyzer {
	return &ResponseCodesAnalyzer{}
}

func (a *ResponseCodesAnalyzer) Name() string {
	return "Response Codes"
}

func (a *ResponseCodesAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "status_code", Title: "Status Code", Width: 80, Sortable: true, DataKey: "status_code"},
		{ID: "status", Title: "Status", Width: 120, Sortable: true, DataKey: "status"},
		{ID: "redirect_url", Title: "Redirect URL", Width: 250, Sortable: true, DataKey: "redirect_url"},
		{ID: "redirect_type", Title: "Redirect Type", Width: 100, Sortable: true, DataKey: "redirect_type"},
		{ID: "chain_length", Title: "Chain Length", Width: 90, Sortable: true, DataKey: "chain_length"},
		{ID: "response_time", Title: "Response Time", Width: 100, Sortable: true, DataKey: "response_time"},
	}
}

func (a *ResponseCodesAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All URLs"},
		{ID: "success", Label: "Success (2xx)", Description: "Successful responses", FilterFunc: func(r *AnalysisResult) bool {
			if code, ok := r.Data["status_code"].(int); ok {
				return code >= 200 && code < 300
			}
			return false
		}},
		{ID: "redirect", Label: "Redirection (3xx)", Description: "Redirect responses", FilterFunc: func(r *AnalysisResult) bool {
			if code, ok := r.Data["status_code"].(int); ok {
				return code >= 300 && code < 400
			}
			return false
		}},
		{ID: "client_error", Label: "Client Error (4xx)", Description: "Client error responses", FilterFunc: func(r *AnalysisResult) bool {
			if code, ok := r.Data["status_code"].(int); ok {
				return code >= 400 && code < 500
			}
			return false
		}},
		{ID: "server_error", Label: "Server Error (5xx)", Description: "Server error responses", FilterFunc: func(r *AnalysisResult) bool {
			if code, ok := r.Data["status_code"].(int); ok {
				return code >= 500 && code < 600
			}
			return false
		}},
		{ID: "redirect_chain", Label: "Redirect Chains", Description: "URLs with redirect chains > 1", FilterFunc: func(r *AnalysisResult) bool {
			if length, ok := r.Data["chain_length"].(int); ok {
				return length > 1
			}
			return false
		}},
		{ID: "slow", Label: "Slow Response", Description: "Response time > 500ms", FilterFunc: func(r *AnalysisResult) bool {
			if time, ok := r.Data["response_time_ms"].(int64); ok {
				return time > Thresholds.SlowResponseTime
			}
			return false
		}},
	}
}

func (a *ResponseCodesAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	if ctx.Fetch == nil {
		return result
	}

	// Basic data
	result.Data["url"] = ctx.URL.URL
	result.Data["status_code"] = ctx.Fetch.StatusCode
	result.Data["status"] = ctx.Fetch.Status
	result.Data["response_time_ms"] = ctx.Fetch.ResponseTime.Milliseconds()
	result.Data["response_time"] = fmt.Sprintf("%dms", ctx.Fetch.ResponseTime.Milliseconds())

	// Redirect analysis
	if ctx.Fetch.RedirectChainID != nil {
		// Parse redirect chain from JSON if available
		result.Data["has_redirect"] = true
	}

	// Determine status category and generate issues
	statusCode := ctx.Fetch.StatusCode

	switch {
	case statusCode >= 200 && statusCode < 300:
		result.Data["status_category"] = "success"

	case statusCode >= 300 && statusCode < 400:
		result.Data["status_category"] = "redirect"
		result.Data["redirect_type"] = a.getRedirectType(statusCode)

		// Issue for redirect chains
		if chainLength, ok := result.Data["chain_length"].(int); ok && chainLength > Thresholds.MaxRedirectChain {
			result.Issues = append(result.Issues, NewIssue(
				ctx.URL.ID,
				storage.IssueRedirectChain,
				storage.IssueTypeWarning,
				storage.SeverityMedium,
				"response",
				fmt.Sprintf("Redirect chain length (%d) exceeds recommended maximum (%d)", chainLength, Thresholds.MaxRedirectChain),
			))
		}

	case statusCode >= 400 && statusCode < 500:
		result.Data["status_category"] = "client_error"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueClientError,
			storage.IssueTypeError,
			storage.SeverityHigh,
			"response",
			fmt.Sprintf("Client error: %d %s", statusCode, ctx.Fetch.Status),
		))

	case statusCode >= 500:
		result.Data["status_category"] = "server_error"
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueServerError,
			storage.IssueTypeError,
			storage.SeverityCritical,
			"response",
			fmt.Sprintf("Server error: %d %s", statusCode, ctx.Fetch.Status),
		))
	}

	// Slow response issue
	if ctx.Fetch.ResponseTime.Milliseconds() > Thresholds.SlowResponseTime {
		result.Issues = append(result.Issues, NewIssue(
			ctx.URL.ID,
			storage.IssueSlowResponse,
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"performance",
			fmt.Sprintf("Slow response time: %dms (threshold: %dms)", ctx.Fetch.ResponseTime.Milliseconds(), Thresholds.SlowResponseTime),
		))
	}

	return result
}

func (a *ResponseCodesAnalyzer) getRedirectType(statusCode int) string {
	switch statusCode {
	case 301:
		return "Permanent (301)"
	case 302:
		return "Temporary (302)"
	case 303:
		return "See Other (303)"
	case 307:
		return "Temporary (307)"
	case 308:
		return "Permanent (308)"
	default:
		return fmt.Sprintf("Redirect (%d)", statusCode)
	}
}

// AnalyzeRedirectChain analyzes a redirect chain.
func (a *ResponseCodesAnalyzer) AnalyzeRedirectChain(chain *storage.RedirectChain) *AnalysisResult {
	result := &AnalysisResult{
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["start_url"] = chain.StartURL
	result.Data["final_url"] = chain.FinalURL
	result.Data["chain_length"] = chain.Length
	result.Data["has_loop"] = chain.HasLoop

	// Parse chain JSON
	var hops []map[string]interface{}
	if err := json.Unmarshal([]byte(chain.ChainJSON), &hops); err == nil {
		result.Data["hops"] = hops
	}

	// Issue for redirect loop
	if chain.HasLoop {
		result.Issues = append(result.Issues, NewIssue(
			0, // URL ID would need to be resolved
			storage.IssueRedirectLoop,
			storage.IssueTypeError,
			storage.SeverityCritical,
			"response",
			fmt.Sprintf("Redirect loop detected: %s", chain.StartURL),
		))
	}

	// Issue for long chain
	if chain.Length > Thresholds.MaxRedirectChain {
		result.Issues = append(result.Issues, NewIssue(
			0,
			storage.IssueRedirectChain,
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"response",
			fmt.Sprintf("Long redirect chain (%d hops): %s -> %s", chain.Length, chain.StartURL, chain.FinalURL),
		))
	}

	return result
}

// ExportRow returns a row for CSV/Excel export.
func (a *ResponseCodesAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["status_code"]),
		fmt.Sprintf("%v", result.Data["status"]),
		fmt.Sprintf("%v", result.Data["redirect_url"]),
		fmt.Sprintf("%v", result.Data["redirect_type"]),
		fmt.Sprintf("%v", result.Data["chain_length"]),
		fmt.Sprintf("%v", result.Data["response_time"]),
	}
}
