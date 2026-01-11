package fetcher

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spider-crawler/spider/internal/config"
)

// Fetcher handles HTTP requests with redirect tracking.
type Fetcher struct {
	client       *http.Client
	config       *config.CrawlConfig
	maxBodySize  int64
	transport    *http.Transport
}

// NewFetcher creates a new HTTP fetcher.
func NewFetcher(cfg *config.CrawlConfig) *Fetcher {
	// Create custom transport for connection pooling and timeouts
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false, // Enable compression
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false, // Verify certificates by default
		},
	}

	f := &Fetcher{
		config:      cfg,
		maxBodySize: 10 * 1024 * 1024, // 10MB default max body size
		transport:   transport,
	}

	// Create HTTP client with custom redirect handling
	f.client = &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Return error to stop redirect and handle manually
			return http.ErrUseLastResponse
		},
	}

	return f
}

// Fetch fetches a URL and returns the response.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) *Response {
	startTime := time.Now()
	response := &Response{
		RequestURL:    rawURL,
		RedirectChain: make([]RedirectHop, 0),
	}

	currentURL := rawURL
	var ttfbRecorded bool

	// Follow redirects manually to track the chain
	for i := 0; i <= f.config.MaxRedirects; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", currentURL, nil)
		if err != nil {
			response.Error = fmt.Errorf("failed to create request: %w", err)
			response.Retryable = false
			return response
		}

		// Set headers
		f.setRequestHeaders(req)

		// Make request
		reqStart := time.Now()
		resp, err := f.client.Do(req)
		if err != nil {
			response.Error = f.categorizeError(err)
			response.Retryable = f.isRetryableError(err)
			response.FinalURL = currentURL
			return response
		}

		// Record TTFB on first response
		if !ttfbRecorded {
			response.TTFB = time.Since(reqStart)
			ttfbRecorded = true
		}

		// Check if redirect
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := resp.Header.Get("Location")
			resp.Body.Close()

			// Record redirect hop
			response.RedirectChain = append(response.RedirectChain, RedirectHop{
				URL:        currentURL,
				StatusCode: resp.StatusCode,
				Location:   location,
			})

			// Resolve relative redirect URL
			if location != "" {
				redirectURL, err := resolveRedirectURL(currentURL, location)
				if err != nil {
					response.Error = fmt.Errorf("invalid redirect location: %w", err)
					response.FinalURL = currentURL
					response.StatusCode = resp.StatusCode
					return response
				}

				// Check redirect policy
				if !f.shouldFollowRedirect(rawURL, redirectURL) {
					response.FinalURL = currentURL
					response.StatusCode = resp.StatusCode
					response.Headers = resp.Header
					return response
				}

				currentURL = redirectURL
				continue
			}
		}

		// Not a redirect or final response - read body
		response.FinalURL = currentURL
		response.StatusCode = resp.StatusCode
		response.Status = resp.Status
		response.Headers = resp.Header
		response.ContentType = extractContentType(resp.Header.Get("Content-Type"))
		response.ContentLength = resp.ContentLength

		// Extract TLS info if available
		if resp.TLS != nil {
			response.TLSInfo = extractTLSInfo(resp.TLS)
		}

		// Read body
		body, bodySize, err := f.readBody(resp)
		resp.Body.Close()

		if err != nil {
			response.Error = fmt.Errorf("failed to read body: %w", err)
			response.Retryable = true
		} else {
			response.Body = body
			response.BodySize = bodySize
		}

		response.ResponseTime = time.Since(startTime)
		return response
	}

	// Max redirects exceeded
	response.Error = fmt.Errorf("max redirects (%d) exceeded", f.config.MaxRedirects)
	response.FinalURL = currentURL
	response.Retryable = false
	return response
}

// setRequestHeaders sets common request headers.
func (f *Fetcher) setRequestHeaders(req *http.Request) {
	req.Header.Set("User-Agent", f.config.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Connection", "keep-alive")
}

// readBody reads the response body with size limit.
func (f *Fetcher) readBody(resp *http.Response) ([]byte, int64, error) {
	var reader io.Reader = resp.Body

	// Handle gzip compression
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, 0, fmt.Errorf("gzip decode error: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Read with size limit
	limitedReader := io.LimitReader(reader, f.maxBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, 0, err
	}

	return body, int64(len(body)), nil
}

// shouldFollowRedirect checks if a redirect should be followed based on policy.
func (f *Fetcher) shouldFollowRedirect(originalURL, redirectURL string) bool {
	switch f.config.RedirectPolicy {
	case config.RedirectNoFollow:
		return false
	case config.RedirectFollowSame:
		// Only follow if same domain
		origHost, _ := extractHost(originalURL)
		redirHost, _ := extractHost(redirectURL)
		return origHost == redirHost
	default: // RedirectFollow
		return true
	}
}

// categorizeError categorizes network errors.
func (f *Fetcher) categorizeError(err error) error {
	if err == nil {
		return nil
	}

	// Check for timeout
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return fmt.Errorf("timeout: %w", err)
	}

	// Check for DNS errors
	if _, ok := err.(*net.DNSError); ok {
		return fmt.Errorf("DNS error: %w", err)
	}

	// Check for connection refused
	if opErr, ok := err.(*net.OpError); ok {
		if opErr.Op == "dial" {
			return fmt.Errorf("connection failed: %w", err)
		}
	}

	// Check for TLS errors
	if strings.Contains(err.Error(), "tls:") || strings.Contains(err.Error(), "certificate") {
		return fmt.Errorf("TLS error: %w", err)
	}

	return err
}

// isRetryableError checks if an error is retryable.
func (f *Fetcher) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Timeouts are retryable
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Connection errors are often temporary
	if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
		return true
	}

	// Some specific errors
	errStr := err.Error()
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"no such host",
		"EOF",
		"broken pipe",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// SetMaxBodySize sets the maximum body size to read.
func (f *Fetcher) SetMaxBodySize(size int64) {
	f.maxBodySize = size
}

// SetInsecureSkipVerify enables/disables TLS certificate verification.
func (f *Fetcher) SetInsecureSkipVerify(skip bool) {
	f.transport.TLSClientConfig.InsecureSkipVerify = skip
}

// Close closes the fetcher and releases resources.
func (f *Fetcher) Close() {
	f.transport.CloseIdleConnections()
}

// Helper functions

func resolveRedirectURL(baseURL, location string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	loc, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	resolved := base.ResolveReference(loc)
	return resolved.String(), nil
}

func extractHost(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return strings.ToLower(u.Host), nil
}

func extractContentType(contentType string) string {
	// Remove charset and other parameters
	if idx := strings.Index(contentType, ";"); idx != -1 {
		return strings.TrimSpace(contentType[:idx])
	}
	return strings.TrimSpace(contentType)
}

func extractTLSInfo(state *tls.ConnectionState) *TLSInfo {
	info := &TLSInfo{
		Version:     tlsVersionString(state.Version),
		CipherSuite: tls.CipherSuiteName(state.CipherSuite),
		ServerName:  state.ServerName,
		IsValid:     true,
	}

	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		info.Subject = cert.Subject.CommonName
		info.Issuer = cert.Issuer.CommonName
		info.NotBefore = cert.NotBefore
		info.NotAfter = cert.NotAfter

		// Check if certificate is currently valid
		now := time.Now()
		if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
			info.IsValid = false
			info.Error = "certificate expired or not yet valid"
		}
	}

	return info
}

func tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}
