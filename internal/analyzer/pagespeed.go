package analyzer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/spider-crawler/spider/internal/storage"
)

// PageSpeedAnalyzer integrates with Google PageSpeed Insights API.
type PageSpeedAnalyzer struct {
	apiKey      string
	cache       map[string]*PageSpeedResult
	cacheMu     sync.RWMutex
	rateLimiter *time.Ticker
	client      *http.Client
}

// PageSpeedResult holds PSI API results.
type PageSpeedResult struct {
	URL           string
	FetchedAt     time.Time
	Strategy      string // "mobile" or "desktop"
	Score         int    // 0-100
	LCP           float64 // Largest Contentful Paint (ms)
	FID           float64 // First Input Delay (ms) - deprecated, use INP
	INP           float64 // Interaction to Next Paint (ms)
	CLS           float64 // Cumulative Layout Shift
	FCP           float64 // First Contentful Paint (ms)
	TTFB          float64 // Time to First Byte (ms)
	TBT           float64 // Total Blocking Time (ms)
	SpeedIndex    float64
	Error         string
}

func NewPageSpeedAnalyzer(apiKey string) *PageSpeedAnalyzer {
	return &PageSpeedAnalyzer{
		apiKey:      apiKey,
		cache:       make(map[string]*PageSpeedResult),
		rateLimiter: time.NewTicker(time.Second), // 1 request per second
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (a *PageSpeedAnalyzer) Name() string {
	return "PageSpeed"
}

func (a *PageSpeedAnalyzer) Columns() []ColumnDef {
	return []ColumnDef{
		{ID: "url", Title: "Address", Width: 300, Sortable: true, DataKey: "url"},
		{ID: "score", Title: "Score", Width: 60, Sortable: true, DataKey: "score"},
		{ID: "lcp", Title: "LCP", Width: 70, Sortable: true, DataKey: "lcp"},
		{ID: "inp", Title: "INP", Width: 70, Sortable: true, DataKey: "inp"},
		{ID: "cls", Title: "CLS", Width: 70, Sortable: true, DataKey: "cls"},
		{ID: "fcp", Title: "FCP", Width: 70, Sortable: true, DataKey: "fcp"},
		{ID: "ttfb", Title: "TTFB", Width: 70, Sortable: true, DataKey: "ttfb"},
		{ID: "status", Title: "Status", Width: 80, Sortable: true, DataKey: "status"},
	}
}

func (a *PageSpeedAnalyzer) Filters() []FilterDef {
	return []FilterDef{
		{ID: "all", Label: "All", Description: "All tested URLs"},
		{ID: "good", Label: "Good (90+)", Description: "Score 90 or above", FilterFunc: func(r *AnalysisResult) bool {
			if score, ok := r.Data["score"].(int); ok {
				return score >= 90
			}
			return false
		}},
		{ID: "needs_improvement", Label: "Needs Improvement (50-89)", Description: "Score between 50-89", FilterFunc: func(r *AnalysisResult) bool {
			if score, ok := r.Data["score"].(int); ok {
				return score >= 50 && score < 90
			}
			return false
		}},
		{ID: "poor", Label: "Poor (<50)", Description: "Score below 50", FilterFunc: func(r *AnalysisResult) bool {
			if score, ok := r.Data["score"].(int); ok {
				return score < 50
			}
			return false
		}},
		{ID: "lcp_poor", Label: "Poor LCP (>4s)", Description: "LCP over 4 seconds", FilterFunc: func(r *AnalysisResult) bool {
			if lcp, ok := r.Data["lcp_ms"].(float64); ok {
				return lcp > 4000
			}
			return false
		}},
		{ID: "cls_poor", Label: "Poor CLS (>0.25)", Description: "CLS over 0.25", FilterFunc: func(r *AnalysisResult) bool {
			if cls, ok := r.Data["cls"].(float64); ok {
				return cls > 0.25
			}
			return false
		}},
	}
}

func (a *PageSpeedAnalyzer) Analyze(ctx *AnalysisContext) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  ctx.URL.ID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = ctx.URL.URL
	result.Data["status"] = "Not Tested"

	return result
}

// FetchPageSpeed fetches PageSpeed data for a URL.
func (a *PageSpeedAnalyzer) FetchPageSpeed(targetURL string, strategy string) *PageSpeedResult {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", strategy, targetURL)
	a.cacheMu.RLock()
	if cached, ok := a.cache[cacheKey]; ok {
		a.cacheMu.RUnlock()
		return cached
	}
	a.cacheMu.RUnlock()

	// Rate limiting
	<-a.rateLimiter.C

	result := &PageSpeedResult{
		URL:       targetURL,
		Strategy:  strategy,
		FetchedAt: time.Now(),
	}

	// Build API URL
	apiURL := fmt.Sprintf(
		"https://www.googleapis.com/pagespeedonline/v5/runPagespeed?url=%s&strategy=%s&category=performance",
		url.QueryEscape(targetURL),
		strategy,
	)

	if a.apiKey != "" {
		apiURL += "&key=" + a.apiKey
	}

	// Make request
	resp, err := a.client.Get(apiURL)
	if err != nil {
		result.Error = fmt.Sprintf("API request failed: %s", err.Error())
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		result.Error = fmt.Sprintf("API returned status %d", resp.StatusCode)
		return result
	}

	// Parse response
	var psiResponse PSIResponse
	if err := json.NewDecoder(resp.Body).Decode(&psiResponse); err != nil {
		result.Error = fmt.Sprintf("Failed to parse response: %s", err.Error())
		return result
	}

	// Extract metrics
	if psiResponse.LighthouseResult != nil {
		lr := psiResponse.LighthouseResult

		// Overall score
		if perf, ok := lr.Categories["performance"]; ok {
			result.Score = int(perf.Score * 100)
		}

		// Core Web Vitals
		if audits := lr.Audits; audits != nil {
			if lcp, ok := audits["largest-contentful-paint"]; ok {
				result.LCP = lcp.NumericValue
			}
			if fcp, ok := audits["first-contentful-paint"]; ok {
				result.FCP = fcp.NumericValue
			}
			if cls, ok := audits["cumulative-layout-shift"]; ok {
				result.CLS = cls.NumericValue
			}
			if tbt, ok := audits["total-blocking-time"]; ok {
				result.TBT = tbt.NumericValue
			}
			if si, ok := audits["speed-index"]; ok {
				result.SpeedIndex = si.NumericValue
			}
			if ttfb, ok := audits["server-response-time"]; ok {
				result.TTFB = ttfb.NumericValue
			}
			// INP might be in experimental audits
			if inp, ok := audits["interaction-to-next-paint"]; ok {
				result.INP = inp.NumericValue
			}
		}
	}

	// Cache result
	a.cacheMu.Lock()
	a.cache[cacheKey] = result
	a.cacheMu.Unlock()

	return result
}

// AnalyzePageSpeedResult converts PSI result to analysis result.
func (a *PageSpeedAnalyzer) AnalyzePageSpeedResult(psi *PageSpeedResult, urlID int64) *AnalysisResult {
	result := &AnalysisResult{
		URLID:  urlID,
		Issues: make([]*storage.Issue, 0),
		Data:   make(map[string]interface{}),
	}

	result.Data["url"] = psi.URL
	result.Data["strategy"] = psi.Strategy

	if psi.Error != "" {
		result.Data["status"] = "Error"
		result.Data["error"] = psi.Error
		return result
	}

	result.Data["status"] = "Tested"
	result.Data["score"] = psi.Score
	result.Data["lcp_ms"] = psi.LCP
	result.Data["lcp"] = fmt.Sprintf("%.1fs", psi.LCP/1000)
	result.Data["fcp_ms"] = psi.FCP
	result.Data["fcp"] = fmt.Sprintf("%.1fs", psi.FCP/1000)
	result.Data["cls"] = psi.CLS
	result.Data["inp_ms"] = psi.INP
	result.Data["inp"] = fmt.Sprintf("%.0fms", psi.INP)
	result.Data["ttfb_ms"] = psi.TTFB
	result.Data["ttfb"] = fmt.Sprintf("%.0fms", psi.TTFB)
	result.Data["tbt_ms"] = psi.TBT

	// Generate issues based on thresholds
	// LCP: Good < 2.5s, Needs Improvement < 4s, Poor >= 4s
	if psi.LCP >= 4000 {
		result.Issues = append(result.Issues, NewIssue(
			urlID,
			"pagespeed_poor_lcp",
			storage.IssueTypeError,
			storage.SeverityHigh,
			"pagespeed",
			fmt.Sprintf("Poor LCP: %.1fs (should be < 2.5s)", psi.LCP/1000),
		))
	} else if psi.LCP >= 2500 {
		result.Issues = append(result.Issues, NewIssue(
			urlID,
			"pagespeed_needs_improvement_lcp",
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"pagespeed",
			fmt.Sprintf("LCP needs improvement: %.1fs (should be < 2.5s)", psi.LCP/1000),
		))
	}

	// CLS: Good < 0.1, Needs Improvement < 0.25, Poor >= 0.25
	if psi.CLS >= 0.25 {
		result.Issues = append(result.Issues, NewIssue(
			urlID,
			"pagespeed_poor_cls",
			storage.IssueTypeError,
			storage.SeverityHigh,
			"pagespeed",
			fmt.Sprintf("Poor CLS: %.3f (should be < 0.1)", psi.CLS),
		))
	} else if psi.CLS >= 0.1 {
		result.Issues = append(result.Issues, NewIssue(
			urlID,
			"pagespeed_needs_improvement_cls",
			storage.IssueTypeWarning,
			storage.SeverityMedium,
			"pagespeed",
			fmt.Sprintf("CLS needs improvement: %.3f (should be < 0.1)", psi.CLS),
		))
	}

	// INP: Good < 200ms, Needs Improvement < 500ms, Poor >= 500ms
	if psi.INP >= 500 {
		result.Issues = append(result.Issues, NewIssue(
			urlID,
			"pagespeed_poor_inp",
			storage.IssueTypeError,
			storage.SeverityHigh,
			"pagespeed",
			fmt.Sprintf("Poor INP: %.0fms (should be < 200ms)", psi.INP),
		))
	}

	return result
}

func (a *PageSpeedAnalyzer) ExportRow(result *AnalysisResult) []string {
	return []string{
		fmt.Sprintf("%v", result.Data["url"]),
		fmt.Sprintf("%v", result.Data["score"]),
		fmt.Sprintf("%v", result.Data["lcp"]),
		fmt.Sprintf("%v", result.Data["inp"]),
		fmt.Sprintf("%v", result.Data["cls"]),
		fmt.Sprintf("%v", result.Data["fcp"]),
		fmt.Sprintf("%v", result.Data["ttfb"]),
		fmt.Sprintf("%v", result.Data["status"]),
	}
}

// PSI API Response structures
type PSIResponse struct {
	LighthouseResult *LighthouseResult `json:"lighthouseResult"`
}

type LighthouseResult struct {
	Categories map[string]Category     `json:"categories"`
	Audits     map[string]Audit        `json:"audits"`
}

type Category struct {
	Score float64 `json:"score"`
}

type Audit struct {
	NumericValue float64 `json:"numericValue"`
	DisplayValue string  `json:"displayValue"`
}
