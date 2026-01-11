// Package auth provides authentication handling for crawling.
package auth

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spider-crawler/spider/internal/config"
)

// Authenticator handles authentication for HTTP requests.
type Authenticator struct {
	mu sync.RWMutex

	config     *config.CrawlConfig
	cookieJar  http.CookieJar
	httpClient *http.Client

	// Session cookies after successful login
	sessionCookies []*http.Cookie

	// Authentication status
	isAuthenticated bool
	lastAuthTime    time.Time
	authError       error
}

// NewAuthenticator creates a new authenticator.
func NewAuthenticator(cfg *config.CrawlConfig) (*Authenticator, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	a := &Authenticator{
		config:    cfg,
		cookieJar: jar,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}

	// Add pre-configured cookies
	if len(cfg.Cookies) > 0 {
		a.addConfiguredCookies()
	}

	return a, nil
}

// addConfiguredCookies adds cookies from configuration.
func (a *Authenticator) addConfiguredCookies() {
	for _, cookieCfg := range a.config.Cookies {
		u, err := url.Parse(fmt.Sprintf("https://%s", cookieCfg.Domain))
		if err != nil {
			continue
		}

		cookie := &http.Cookie{
			Name:     cookieCfg.Name,
			Value:    cookieCfg.Value,
			Domain:   cookieCfg.Domain,
			Path:     cookieCfg.Path,
			Secure:   cookieCfg.Secure,
			HttpOnly: cookieCfg.HttpOnly,
		}

		a.cookieJar.SetCookies(u, []*http.Cookie{cookie})
	}
}

// Authenticate performs authentication based on config.
func (a *Authenticator) Authenticate() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch a.config.AuthType {
	case config.AuthNone:
		a.isAuthenticated = true
		return nil

	case config.AuthBasic:
		// Basic auth is applied per-request
		a.isAuthenticated = true
		return nil

	case config.AuthBearer:
		// Bearer auth is applied per-request
		a.isAuthenticated = true
		return nil

	case config.AuthCookie:
		// Cookies are already added
		a.isAuthenticated = true
		return nil

	case config.AuthForm:
		return a.performFormLogin()

	default:
		return fmt.Errorf("unknown auth type: %s", a.config.AuthType)
	}
}

// performFormLogin performs form-based login.
func (a *Authenticator) performFormLogin() error {
	if a.config.Auth == nil {
		return fmt.Errorf("auth configuration is missing")
	}

	if a.config.Auth.LoginURL == "" {
		return fmt.Errorf("login URL is required for form authentication")
	}

	// Build form data
	formData := url.Values{}
	for key, value := range a.config.Auth.FormFields {
		formData.Set(key, value)
	}

	// Perform login request
	resp, err := a.httpClient.PostForm(a.config.Auth.LoginURL, formData)
	if err != nil {
		a.authError = fmt.Errorf("login request failed: %w", err)
		return a.authError
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.authError = fmt.Errorf("failed to read login response: %w", err)
		return a.authError
	}

	// Check for success
	if resp.StatusCode >= 400 {
		a.authError = fmt.Errorf("login failed with status %d", resp.StatusCode)
		return a.authError
	}

	// Check success URL if specified
	if a.config.Auth.SuccessURL != "" && resp.Request.URL.String() != a.config.Auth.SuccessURL {
		// Check if redirected to expected URL
		if !strings.HasPrefix(resp.Request.URL.String(), a.config.Auth.SuccessURL) {
			a.authError = fmt.Errorf("login redirect to unexpected URL: %s", resp.Request.URL)
			return a.authError
		}
	}

	// Check success text if specified
	if a.config.Auth.SuccessText != "" {
		if !strings.Contains(string(body), a.config.Auth.SuccessText) {
			a.authError = fmt.Errorf("login response does not contain success text")
			return a.authError
		}
	}

	// Store session cookies
	loginURL, _ := url.Parse(a.config.Auth.LoginURL)
	a.sessionCookies = a.cookieJar.Cookies(loginURL)

	a.isAuthenticated = true
	a.lastAuthTime = time.Now()
	a.authError = nil

	return nil
}

// ApplyAuth applies authentication to an HTTP request.
func (a *Authenticator) ApplyAuth(req *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	switch a.config.AuthType {
	case config.AuthBasic:
		if a.config.Auth != nil {
			credentials := base64.StdEncoding.EncodeToString(
				[]byte(a.config.Auth.Username + ":" + a.config.Auth.Password),
			)
			req.Header.Set("Authorization", "Basic "+credentials)
		}

	case config.AuthBearer:
		if a.config.Auth != nil && a.config.Auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+a.config.Auth.Token)
		}

	case config.AuthCookie, config.AuthForm:
		// Cookies are automatically added by the cookie jar
		// But we can also manually add session cookies
		for _, cookie := range a.sessionCookies {
			req.AddCookie(cookie)
		}
	}

	// Apply custom headers
	for key, value := range a.config.CustomHeaders {
		req.Header.Set(key, value)
	}
}

// IsAuthenticated returns whether authentication was successful.
func (a *Authenticator) IsAuthenticated() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isAuthenticated
}

// GetAuthError returns the last authentication error.
func (a *Authenticator) GetAuthError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.authError
}

// GetCookieJar returns the cookie jar for use with HTTP client.
func (a *Authenticator) GetCookieJar() http.CookieJar {
	return a.cookieJar
}

// GetSessionCookies returns the session cookies.
func (a *Authenticator) GetSessionCookies() []*http.Cookie {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sessionCookies
}

// SetCookie manually sets a cookie.
func (a *Authenticator) SetCookie(domain, name, value string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	u, err := url.Parse(fmt.Sprintf("https://%s", domain))
	if err != nil {
		return
	}

	cookie := &http.Cookie{
		Name:   name,
		Value:  value,
		Domain: domain,
		Path:   "/",
	}

	a.cookieJar.SetCookies(u, []*http.Cookie{cookie})
}

// ClearCookies removes all cookies.
func (a *Authenticator) ClearCookies() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Create new cookie jar
	jar, _ := cookiejar.New(nil)
	a.cookieJar = jar
	a.httpClient.Jar = jar
	a.sessionCookies = nil
	a.isAuthenticated = false
}

// GetHTTPClient returns an HTTP client with authentication configured.
func (a *Authenticator) GetHTTPClient() *http.Client {
	return a.httpClient
}

// RefreshAuth re-authenticates if needed.
func (a *Authenticator) RefreshAuth() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if re-authentication is needed
	if a.config.AuthType == config.AuthForm {
		// Re-authenticate after 30 minutes
		if time.Since(a.lastAuthTime) > 30*time.Minute {
			a.isAuthenticated = false
			return a.performFormLogin()
		}
	}

	return nil
}

// ExportCookies exports cookies in Netscape format.
func (a *Authenticator) ExportCookies(domain string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	u, err := url.Parse(fmt.Sprintf("https://%s", domain))
	if err != nil {
		return ""
	}

	cookies := a.cookieJar.Cookies(u)
	var builder strings.Builder

	builder.WriteString("# Netscape HTTP Cookie File\n")
	builder.WriteString("# https://curl.se/docs/http-cookies.html\n\n")

	for _, cookie := range cookies {
		// Format: domain, flag, path, secure, expiration, name, value
		httpOnly := "FALSE"
		if cookie.HttpOnly {
			httpOnly = "TRUE"
		}
		secure := "FALSE"
		if cookie.Secure {
			secure = "TRUE"
		}
		expires := "0"
		if !cookie.Expires.IsZero() {
			expires = fmt.Sprintf("%d", cookie.Expires.Unix())
		}

		builder.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			cookie.Domain,
			httpOnly,
			cookie.Path,
			secure,
			expires,
			cookie.Name,
			cookie.Value,
		))
	}

	return builder.String()
}

// ImportCookies imports cookies from Netscape format.
func (a *Authenticator) ImportCookies(data string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 7 {
			continue
		}

		domain := parts[0]
		// httpOnly := parts[1] == "TRUE"
		path := parts[2]
		secure := parts[3] == "TRUE"
		// expires := parts[4]
		name := parts[5]
		value := parts[6]

		u, err := url.Parse(fmt.Sprintf("https://%s", domain))
		if err != nil {
			continue
		}

		cookie := &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: domain,
			Path:   path,
			Secure: secure,
		}

		a.cookieJar.SetCookies(u, []*http.Cookie{cookie})
	}

	return nil
}
