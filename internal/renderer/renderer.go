// Package renderer provides JavaScript rendering capabilities using Chromium.
package renderer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/spider-crawler/spider/internal/config"
)

// RenderResult holds the result of rendering a page.
type RenderResult struct {
	// Final HTML after JavaScript execution
	HTML string

	// Final URL after any client-side redirects
	FinalURL string

	// Page title
	Title string

	// Response status code
	StatusCode int

	// Response headers
	Headers map[string]string

	// Performance metrics
	Metrics *PerformanceMetrics

	// Resources loaded
	Resources []*ResourceInfo

	// Console messages
	ConsoleMessages []string

	// JavaScript errors
	JSErrors []string

	// Render duration
	RenderTime time.Duration

	// Error if any
	Error error
}

// PerformanceMetrics holds page load performance data.
type PerformanceMetrics struct {
	// Navigation timing
	NavigationStart      float64
	DOMContentLoaded     float64
	LoadEventEnd         float64
	FirstPaint           float64
	FirstContentfulPaint float64

	// Layout metrics
	LayoutCount   int64
	RecalcStyleCount int64
	ScriptDuration float64
	TaskDuration   float64
}

// ResourceInfo holds information about a loaded resource.
type ResourceInfo struct {
	URL          string
	Type         string
	Status       int
	Size         int64
	MimeType     string
	FromCache    bool
	LoadTime     time.Duration
}

// Renderer handles JavaScript rendering using Chromium.
type Renderer struct {
	mu sync.Mutex

	config    *config.CrawlConfig
	allocator context.Context
	cancel    context.CancelFunc

	// Browser pool for concurrent rendering
	browserPool chan context.Context
	poolSize    int
}

// NewRenderer creates a new renderer instance.
func NewRenderer(cfg *config.CrawlConfig) (*Renderer, error) {
	r := &Renderer{
		config:   cfg,
		poolSize: cfg.Concurrency,
	}

	// Create allocator options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-features", "TranslateUI"),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.UserAgent(cfg.UserAgent),
	)

	// Use custom Chromium path if specified
	if cfg.ChromiumPath != "" {
		opts = append(opts, chromedp.ExecPath(cfg.ChromiumPath))
	}

	// Create allocator context
	r.allocator, r.cancel = chromedp.NewExecAllocator(context.Background(), opts...)

	// Initialize browser pool
	r.browserPool = make(chan context.Context, r.poolSize)
	for i := 0; i < r.poolSize; i++ {
		ctx, _ := chromedp.NewContext(r.allocator)
		r.browserPool <- ctx
	}

	return r, nil
}

// Render renders a page and returns the result.
func (r *Renderer) Render(urlStr string) *RenderResult {
	result := &RenderResult{
		Headers:   make(map[string]string),
		Resources: make([]*ResourceInfo, 0),
	}

	startTime := time.Now()

	// Get browser context from pool
	ctx := <-r.browserPool
	defer func() {
		r.browserPool <- ctx
	}()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, r.config.RenderTimeout)
	defer cancel()

	// Track resources
	resources := make(map[string]*ResourceInfo)
	var resourcesMu sync.Mutex

	// Listen for network events
	chromedp.ListenTarget(timeoutCtx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			resourcesMu.Lock()
			resources[e.RequestID.String()] = &ResourceInfo{
				URL:      e.Response.URL,
				Type:     string(e.Type),
				Status:   int(e.Response.Status),
				MimeType: e.Response.MimeType,
			}
			resourcesMu.Unlock()

			// Capture main document headers
			if e.Type == network.ResourceTypeDocument {
				for k, v := range e.Response.Headers {
					if str, ok := v.(string); ok {
						result.Headers[k] = str
					}
				}
				result.StatusCode = int(e.Response.Status)
			}

		case *network.EventLoadingFinished:
			resourcesMu.Lock()
			if res, ok := resources[e.RequestID.String()]; ok {
				res.Size = int64(e.EncodedDataLength)
			}
			resourcesMu.Unlock()

		case *page.EventJavascriptDialogOpening:
			// Dismiss any dialogs
			go chromedp.Run(timeoutCtx, page.HandleJavaScriptDialog(true))
		}
	})

	// Enable network tracking
	if err := chromedp.Run(timeoutCtx, network.Enable()); err != nil {
		result.Error = fmt.Errorf("failed to enable network: %w", err)
		return result
	}

	// Build navigation actions based on wait condition
	var waitAction chromedp.Action
	switch r.config.WaitCondition {
	case config.WaitDOMContentLoaded:
		waitAction = chromedp.WaitReady("body", chromedp.ByQuery)
	case config.WaitLoad:
		waitAction = chromedp.WaitReady("body", chromedp.ByQuery)
	case config.WaitNetworkIdle:
		waitAction = chromedp.Sleep(2 * time.Second) // Simplified network idle
	case config.WaitSelector:
		if r.config.WaitSelector != "" {
			waitAction = chromedp.WaitVisible(r.config.WaitSelector, chromedp.ByQuery)
		} else {
			waitAction = chromedp.WaitReady("body", chromedp.ByQuery)
		}
	default:
		waitAction = chromedp.WaitReady("body", chromedp.ByQuery)
	}

	// Navigate and wait
	var html string
	var title string
	var finalURL string

	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(urlStr),
		waitAction,
		chromedp.Location(&finalURL),
		chromedp.Title(&title),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			html, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		}),
	)

	if err != nil {
		result.Error = fmt.Errorf("render failed: %w", err)
		return result
	}

	result.HTML = html
	result.Title = title
	result.FinalURL = finalURL
	result.RenderTime = time.Since(startTime)

	// Collect resources
	resourcesMu.Lock()
	for _, res := range resources {
		result.Resources = append(result.Resources, res)
	}
	resourcesMu.Unlock()

	// Get performance metrics
	result.Metrics = r.getPerformanceMetrics(timeoutCtx)

	return result
}

// getPerformanceMetrics extracts performance metrics from the page.
func (r *Renderer) getPerformanceMetrics(ctx context.Context) *PerformanceMetrics {
	metrics := &PerformanceMetrics{}

	var perfJSON string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`JSON.stringify(performance.timing)`, &perfJSON),
	)
	if err != nil {
		return metrics
	}

	// Parse timing data (simplified)
	var timing map[string]float64
	// Note: Would need to parse JSON properly
	_ = timing

	// Get paint timing
	var paintEntries string
	chromedp.Run(ctx,
		chromedp.Evaluate(`JSON.stringify(performance.getEntriesByType('paint'))`, &paintEntries),
	)

	return metrics
}

// RenderBatch renders multiple URLs concurrently.
func (r *Renderer) RenderBatch(urls []string) []*RenderResult {
	results := make([]*RenderResult, len(urls))
	var wg sync.WaitGroup

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, u string) {
			defer wg.Done()
			results[idx] = r.Render(u)
		}(i, url)
	}

	wg.Wait()
	return results
}

// Close shuts down the renderer and releases resources.
func (r *Renderer) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close browser pool
	close(r.browserPool)
	for ctx := range r.browserPool {
		chromedp.Cancel(ctx)
	}

	// Cancel allocator
	if r.cancel != nil {
		r.cancel()
	}

	return nil
}

// Screenshot captures a screenshot of the page.
func (r *Renderer) Screenshot(urlStr string, quality int) ([]byte, error) {
	ctx := <-r.browserPool
	defer func() {
		r.browserPool <- ctx
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, r.config.RenderTimeout)
	defer cancel()

	var buf []byte
	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(urlStr),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.FullScreenshot(&buf, quality),
	)

	if err != nil {
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	return buf, nil
}

// PDF generates a PDF of the page.
func (r *Renderer) PDF(urlStr string) ([]byte, error) {
	ctx := <-r.browserPool
	defer func() {
		r.browserPool <- ctx
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, r.config.RenderTimeout)
	defer cancel()

	var buf []byte
	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(urlStr),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("PDF generation failed: %w", err)
	}

	return buf, nil
}

// ExecuteScript executes JavaScript on a page and returns the result.
func (r *Renderer) ExecuteScript(urlStr string, script string) (interface{}, error) {
	ctx := <-r.browserPool
	defer func() {
		r.browserPool <- ctx
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, r.config.RenderTimeout)
	defer cancel()

	var result interface{}
	err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(urlStr),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(script, &result),
	)

	if err != nil {
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	return result, nil
}

// CheckMobileFriendly performs a mobile-friendly check.
func (r *Renderer) CheckMobileFriendly(urlStr string) (*MobileFriendlyResult, error) {
	result := &MobileFriendlyResult{}

	// Create mobile device emulation context
	ctx := <-r.browserPool
	defer func() {
		r.browserPool <- ctx
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, r.config.RenderTimeout)
	defer cancel()

	// Set mobile viewport
	err := chromedp.Run(timeoutCtx,
		chromedp.EmulateViewport(375, 667, chromedp.EmulateScale(2)),
		chromedp.Navigate(urlStr),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)

	if err != nil {
		return nil, fmt.Errorf("mobile check failed: %w", err)
	}

	// Check viewport meta
	var viewportContent string
	chromedp.Run(timeoutCtx,
		chromedp.Evaluate(`document.querySelector('meta[name="viewport"]')?.content || ''`, &viewportContent),
	)
	result.HasViewport = viewportContent != ""
	result.ViewportContent = viewportContent

	// Check for horizontal scroll
	var hasHorizontalScroll bool
	chromedp.Run(timeoutCtx,
		chromedp.Evaluate(`document.documentElement.scrollWidth > document.documentElement.clientWidth`, &hasHorizontalScroll),
	)
	result.HasHorizontalScroll = hasHorizontalScroll

	// Check font sizes
	var smallFonts int
	chromedp.Run(timeoutCtx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('p, span, a, li, td')).filter(el => {
				const style = window.getComputedStyle(el);
				return parseFloat(style.fontSize) < 12;
			}).length
		`, &smallFonts),
	)
	result.SmallFontCount = smallFonts

	// Check tap targets
	var smallTapTargets int
	chromedp.Run(timeoutCtx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('a, button, input, select')).filter(el => {
				const rect = el.getBoundingClientRect();
				return rect.width < 48 || rect.height < 48;
			}).length
		`, &smallTapTargets),
	)
	result.SmallTapTargetCount = smallTapTargets

	result.IsMobileFriendly = result.HasViewport && !result.HasHorizontalScroll && smallFonts == 0 && smallTapTargets == 0

	return result, nil
}

// MobileFriendlyResult holds mobile-friendliness check results.
type MobileFriendlyResult struct {
	IsMobileFriendly    bool
	HasViewport         bool
	ViewportContent     string
	HasHorizontalScroll bool
	SmallFontCount      int
	SmallTapTargetCount int
}
