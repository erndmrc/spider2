// Package urlutil provides URL normalization and utility functions.
package urlutil

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Normalizer handles URL normalization.
type Normalizer struct {
	// Query parameters to remove (utm_*, gclid, etc.)
	IgnoreParams map[string]struct{}

	// Remove trailing slashes
	RemoveTrailingSlash bool

	// Remove default ports (80 for http, 443 for https)
	RemoveDefaultPort bool

	// Remove fragment (#...)
	RemoveFragment bool

	// Lowercase scheme and host
	LowercaseSchemeHost bool

	// Sort query parameters
	SortQueryParams bool

	// Remove www prefix
	RemoveWWW bool
}

// DefaultNormalizer returns a normalizer with default settings.
func DefaultNormalizer(ignoreParams []string) *Normalizer {
	params := make(map[string]struct{})
	for _, p := range ignoreParams {
		params[strings.ToLower(p)] = struct{}{}
	}

	return &Normalizer{
		IgnoreParams:        params,
		RemoveTrailingSlash: true,
		RemoveDefaultPort:   true,
		RemoveFragment:      true,
		LowercaseSchemeHost: true,
		SortQueryParams:     true,
		RemoveWWW:           false, // Keep www by default
	}
}

// Normalize normalizes a URL string.
func (n *Normalizer) Normalize(rawURL string) (string, error) {
	// Parse the URL
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}

	// Lowercase scheme
	if n.LowercaseSchemeHost {
		u.Scheme = strings.ToLower(u.Scheme)
		u.Host = strings.ToLower(u.Host)
	}

	// Remove default ports
	if n.RemoveDefaultPort {
		host := u.Host
		if u.Scheme == "http" && strings.HasSuffix(host, ":80") {
			u.Host = strings.TrimSuffix(host, ":80")
		} else if u.Scheme == "https" && strings.HasSuffix(host, ":443") {
			u.Host = strings.TrimSuffix(host, ":443")
		}
	}

	// Remove www prefix if configured
	if n.RemoveWWW {
		u.Host = strings.TrimPrefix(u.Host, "www.")
	}

	// Remove fragment
	if n.RemoveFragment {
		u.Fragment = ""
	}

	// Handle path
	path := u.Path
	if path == "" {
		path = "/"
	}

	// Remove trailing slash (except for root)
	if n.RemoveTrailingSlash && len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	// Normalize path (remove double slashes, resolve . and ..)
	path = normalizePath(path)
	u.Path = path

	// Handle query parameters
	if u.RawQuery != "" {
		query := u.Query()
		newQuery := url.Values{}

		for key, values := range query {
			// Skip ignored parameters
			if _, ignore := n.IgnoreParams[strings.ToLower(key)]; ignore {
				continue
			}
			// Skip empty values
			for _, v := range values {
				if v != "" || len(values) == 1 {
					newQuery.Add(key, v)
				}
			}
		}

		if n.SortQueryParams {
			u.RawQuery = sortedQueryString(newQuery)
		} else {
			u.RawQuery = newQuery.Encode()
		}
	}

	return u.String(), nil
}

// normalizePath removes double slashes and resolves . and ..
func normalizePath(path string) string {
	// Replace multiple slashes with single slash
	re := regexp.MustCompile(`/+`)
	path = re.ReplaceAllString(path, "/")

	// Split and resolve . and ..
	parts := strings.Split(path, "/")
	var result []string

	for _, part := range parts {
		switch part {
		case ".":
			// Skip current directory
		case "..":
			// Go up one directory
			if len(result) > 0 && result[len(result)-1] != "" {
				result = result[:len(result)-1]
			}
		default:
			result = append(result, part)
		}
	}

	normalized := strings.Join(result, "/")
	if normalized == "" {
		return "/"
	}
	return normalized
}

// sortedQueryString returns a sorted query string.
func sortedQueryString(query url.Values) string {
	if len(query) == 0 {
		return ""
	}

	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			if v == "" {
				parts = append(parts, url.QueryEscape(k))
			} else {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
	}

	return strings.Join(parts, "&")
}

// ExtractHost extracts the host from a URL.
func ExtractHost(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return strings.ToLower(u.Host), nil
}

// ExtractDomain extracts the registrable domain from a host.
func ExtractDomain(host string) string {
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Check if it's IPv6
		if !strings.Contains(host, "]") || idx > strings.LastIndex(host, "]") {
			host = host[:idx]
		}
	}

	// Simple domain extraction (for more accurate results, use publicsuffix)
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return host
}

// IsAbsoluteURL checks if a URL is absolute.
func IsAbsoluteURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.IsAbs()
}

// ResolveURL resolves a possibly relative URL against a base URL.
func ResolveURL(base, ref string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}

	resolved := baseURL.ResolveReference(refURL)
	return resolved.String(), nil
}

// IsSameHost checks if two URLs have the same host.
func IsSameHost(url1, url2 string) bool {
	host1, err1 := ExtractHost(url1)
	host2, err2 := ExtractHost(url2)
	if err1 != nil || err2 != nil {
		return false
	}
	return host1 == host2
}

// IsSameDomain checks if two URLs have the same registrable domain.
func IsSameDomain(url1, url2 string) bool {
	host1, err1 := ExtractHost(url1)
	host2, err2 := ExtractHost(url2)
	if err1 != nil || err2 != nil {
		return false
	}
	return ExtractDomain(host1) == ExtractDomain(host2)
}
