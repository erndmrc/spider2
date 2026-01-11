// Package fetcher handles HTTP fetching with redirect tracking and header capture.
package fetcher

import (
	"net/http"
	"time"
)

// Response represents the result of fetching a URL.
type Response struct {
	// Original requested URL
	RequestURL string

	// Final URL after redirects
	FinalURL string

	// HTTP status code
	StatusCode int

	// Status text (e.g., "200 OK")
	Status string

	// Response headers
	Headers http.Header

	// Content-Type header value
	ContentType string

	// Content-Length (from header or actual body size)
	ContentLength int64

	// Actual body size in bytes
	BodySize int64

	// Response body (HTML content)
	Body []byte

	// Redirect chain (list of URLs in redirect sequence)
	RedirectChain []RedirectHop

	// Time to first byte
	TTFB time.Duration

	// Total response time
	ResponseTime time.Duration

	// TLS/SSL information
	TLSInfo *TLSInfo

	// Error if request failed
	Error error

	// Whether this response should be retried
	Retryable bool
}

// RedirectHop represents a single redirect in the chain.
type RedirectHop struct {
	URL        string
	StatusCode int
	Location   string
}

// TLSInfo contains TLS/SSL certificate information.
type TLSInfo struct {
	Version     string
	CipherSuite string
	ServerName  string
	Issuer      string
	Subject     string
	NotBefore   time.Time
	NotAfter    time.Time
	IsValid     bool
	Error       string
}

// IsSuccess returns true if the response was successful (2xx).
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsRedirect returns true if the response was a redirect (3xx).
func (r *Response) IsRedirect() bool {
	return r.StatusCode >= 300 && r.StatusCode < 400
}

// IsClientError returns true if the response was a client error (4xx).
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// IsServerError returns true if the response was a server error (5xx).
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500 && r.StatusCode < 600
}

// HasRedirects returns true if there were any redirects.
func (r *Response) HasRedirects() bool {
	return len(r.RedirectChain) > 0
}

// RedirectCount returns the number of redirects.
func (r *Response) RedirectCount() int {
	return len(r.RedirectChain)
}

// GetHeader returns a header value (case-insensitive).
func (r *Response) GetHeader(name string) string {
	if r.Headers == nil {
		return ""
	}
	return r.Headers.Get(name)
}

// IsHTML returns true if the content type is HTML.
func (r *Response) IsHTML() bool {
	ct := r.ContentType
	return ct == "text/html" ||
		len(ct) > 9 && ct[:9] == "text/html"
}
