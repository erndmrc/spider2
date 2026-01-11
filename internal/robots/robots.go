// Package robots handles robots.txt parsing and meta robots directives.
package robots

import (
	"bufio"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RobotsTxt represents a parsed robots.txt file.
type RobotsTxt struct {
	// Rules per user-agent
	rules map[string]*AgentRules

	// Sitemaps found in robots.txt
	Sitemaps []string

	// Host directive (if present)
	Host string

	// Raw content
	Raw string

	// Parse errors
	Errors []string
}

// AgentRules contains rules for a specific user-agent.
type AgentRules struct {
	UserAgent  string
	Allow      []string
	Disallow   []string
	CrawlDelay time.Duration

	// Compiled patterns for faster matching
	allowPatterns    []*regexp.Regexp
	disallowPatterns []*regexp.Regexp
}

// NewRobotsTxt creates an empty RobotsTxt.
func NewRobotsTxt() *RobotsTxt {
	return &RobotsTxt{
		rules:    make(map[string]*AgentRules),
		Sitemaps: make([]string, 0),
		Errors:   make([]string, 0),
	}
}

// Parse parses robots.txt content.
func Parse(content string) *RobotsTxt {
	robots := NewRobotsTxt()
	robots.Raw = content

	scanner := bufio.NewScanner(strings.NewReader(content))
	var currentAgents []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove inline comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		// Parse directive
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		directive := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch directive {
		case "user-agent":
			agent := strings.ToLower(value)
			if len(currentAgents) == 0 || !isAgentDirective(directive) {
				currentAgents = []string{agent}
			} else {
				currentAgents = append(currentAgents, agent)
			}
			// Ensure rules exist for this agent
			if _, exists := robots.rules[agent]; !exists {
				robots.rules[agent] = &AgentRules{
					UserAgent: agent,
					Allow:     make([]string, 0),
					Disallow:  make([]string, 0),
				}
			}

		case "disallow":
			for _, agent := range currentAgents {
				if rules, exists := robots.rules[agent]; exists {
					rules.Disallow = append(rules.Disallow, value)
					if pattern := compilePattern(value); pattern != nil {
						rules.disallowPatterns = append(rules.disallowPatterns, pattern)
					}
				}
			}

		case "allow":
			for _, agent := range currentAgents {
				if rules, exists := robots.rules[agent]; exists {
					rules.Allow = append(rules.Allow, value)
					if pattern := compilePattern(value); pattern != nil {
						rules.allowPatterns = append(rules.allowPatterns, pattern)
					}
				}
			}

		case "crawl-delay":
			delay, err := strconv.ParseFloat(value, 64)
			if err == nil {
				for _, agent := range currentAgents {
					if rules, exists := robots.rules[agent]; exists {
						rules.CrawlDelay = time.Duration(delay * float64(time.Second))
					}
				}
			}

		case "sitemap":
			robots.Sitemaps = append(robots.Sitemaps, value)

		case "host":
			robots.Host = value
		}
	}

	return robots
}

// IsAllowed checks if a URL is allowed for a given user-agent.
func (r *RobotsTxt) IsAllowed(userAgent, urlPath string) bool {
	rules := r.getRulesForAgent(userAgent)
	if rules == nil {
		return true // No rules = allowed
	}

	// Normalize path
	if urlPath == "" {
		urlPath = "/"
	}

	// Check allow rules first (more specific wins)
	allowMatch := r.findBestMatch(rules.Allow, rules.allowPatterns, urlPath)
	disallowMatch := r.findBestMatch(rules.Disallow, rules.disallowPatterns, urlPath)

	// If no disallow match, allowed
	if disallowMatch == "" {
		return true
	}

	// If no allow match but disallow match, disallowed
	if allowMatch == "" {
		return false
	}

	// Both match - longer (more specific) wins
	return len(allowMatch) >= len(disallowMatch)
}

// GetCrawlDelay returns the crawl delay for a user-agent.
func (r *RobotsTxt) GetCrawlDelay(userAgent string) time.Duration {
	rules := r.getRulesForAgent(userAgent)
	if rules == nil {
		return 0
	}
	return rules.CrawlDelay
}

// getRulesForAgent finds rules for a specific user-agent.
func (r *RobotsTxt) getRulesForAgent(userAgent string) *AgentRules {
	userAgent = strings.ToLower(userAgent)

	// Try exact match first
	if rules, exists := r.rules[userAgent]; exists {
		return rules
	}

	// Try partial match (user-agent contains)
	for agent, rules := range r.rules {
		if strings.Contains(userAgent, agent) || strings.Contains(agent, userAgent) {
			return rules
		}
	}

	// Fall back to wildcard
	if rules, exists := r.rules["*"]; exists {
		return rules
	}

	return nil
}

// findBestMatch finds the longest matching pattern.
func (r *RobotsTxt) findBestMatch(patterns []string, compiled []*regexp.Regexp, path string) string {
	var bestMatch string

	for i, pattern := range patterns {
		if pattern == "" {
			continue
		}

		var matched bool
		if i < len(compiled) && compiled[i] != nil {
			matched = compiled[i].MatchString(path)
		} else {
			matched = matchSimple(pattern, path)
		}

		if matched && len(pattern) > len(bestMatch) {
			bestMatch = pattern
		}
	}

	return bestMatch
}

// compilePattern converts a robots.txt pattern to regex.
func compilePattern(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}

	// Escape regex special characters except * and $
	escaped := regexp.QuoteMeta(pattern)

	// Convert * to regex .*
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)

	// Convert $ at end to regex end anchor
	if strings.HasSuffix(escaped, `\$`) {
		escaped = escaped[:len(escaped)-2] + "$"
	}

	// Must match from start
	escaped = "^" + escaped

	re, err := regexp.Compile(escaped)
	if err != nil {
		return nil
	}
	return re
}

// matchSimple does simple prefix matching without regex.
func matchSimple(pattern, path string) bool {
	// Handle wildcard
	if strings.Contains(pattern, "*") {
		return false // Let regex handle it
	}

	// Simple prefix match
	return strings.HasPrefix(path, pattern)
}

func isAgentDirective(directive string) bool {
	return directive == "user-agent"
}

// MetaRobots represents parsed meta robots directives.
type MetaRobots struct {
	NoIndex        bool
	NoFollow       bool
	NoArchive      bool
	NoSnippet      bool
	NoImageIndex   bool
	NoTranslate    bool
	MaxSnippet     int  // -1 = not set
	MaxImagePreview string // "none", "standard", "large"
	MaxVideoPreview int  // -1 = not set
	Unavailable    *time.Time
	Raw            string
}

// ParseMetaRobots parses a meta robots content string.
func ParseMetaRobots(content string) *MetaRobots {
	meta := &MetaRobots{
		MaxSnippet:      -1,
		MaxVideoPreview: -1,
		Raw:             content,
	}

	content = strings.ToLower(strings.TrimSpace(content))
	directives := strings.Split(content, ",")

	for _, d := range directives {
		d = strings.TrimSpace(d)

		switch {
		case d == "noindex":
			meta.NoIndex = true
		case d == "nofollow":
			meta.NoFollow = true
		case d == "noarchive":
			meta.NoArchive = true
		case d == "nosnippet":
			meta.NoSnippet = true
		case d == "noimageindex":
			meta.NoImageIndex = true
		case d == "notranslate":
			meta.NoTranslate = true
		case d == "none":
			meta.NoIndex = true
			meta.NoFollow = true
		case d == "all":
			// Default behavior, everything allowed
		case strings.HasPrefix(d, "max-snippet:"):
			if val, err := strconv.Atoi(strings.TrimPrefix(d, "max-snippet:")); err == nil {
				meta.MaxSnippet = val
			}
		case strings.HasPrefix(d, "max-image-preview:"):
			meta.MaxImagePreview = strings.TrimPrefix(d, "max-image-preview:")
		case strings.HasPrefix(d, "max-video-preview:"):
			if val, err := strconv.Atoi(strings.TrimPrefix(d, "max-video-preview:")); err == nil {
				meta.MaxVideoPreview = val
			}
		}
	}

	return meta
}

// IsIndexable returns true if the page can be indexed.
func (m *MetaRobots) IsIndexable() bool {
	return !m.NoIndex
}

// IsFollowable returns true if links on the page can be followed.
func (m *MetaRobots) IsFollowable() bool {
	return !m.NoFollow
}

// XRobotsTag represents parsed X-Robots-Tag header.
type XRobotsTag struct {
	// Per user-agent directives
	Directives map[string]*MetaRobots

	// Default directives (no user-agent specified)
	Default *MetaRobots

	Raw string
}

// ParseXRobotsTag parses X-Robots-Tag header value(s).
func ParseXRobotsTag(values []string) *XRobotsTag {
	tag := &XRobotsTag{
		Directives: make(map[string]*MetaRobots),
		Raw:        strings.Join(values, ", "),
	}

	for _, value := range values {
		value = strings.TrimSpace(value)

		// Check if user-agent is specified: "googlebot: noindex"
		if idx := strings.Index(value, ":"); idx != -1 {
			possibleAgent := strings.TrimSpace(value[:idx])
			// Check if it looks like a user-agent (no spaces, common pattern)
			if !strings.Contains(possibleAgent, " ") && !strings.HasPrefix(possibleAgent, "max-") {
				agent := strings.ToLower(possibleAgent)
				directives := strings.TrimSpace(value[idx+1:])
				tag.Directives[agent] = ParseMetaRobots(directives)
				continue
			}
		}

		// No user-agent specified, apply to default
		if tag.Default == nil {
			tag.Default = ParseMetaRobots(value)
		} else {
			// Merge with existing default
			parsed := ParseMetaRobots(value)
			if parsed.NoIndex {
				tag.Default.NoIndex = true
			}
			if parsed.NoFollow {
				tag.Default.NoFollow = true
			}
			// ... merge other fields as needed
		}
	}

	return tag
}

// GetDirectives returns directives for a specific user-agent.
func (x *XRobotsTag) GetDirectives(userAgent string) *MetaRobots {
	userAgent = strings.ToLower(userAgent)

	// Try exact match
	if directives, exists := x.Directives[userAgent]; exists {
		return directives
	}

	// Try partial match
	for agent, directives := range x.Directives {
		if strings.Contains(userAgent, agent) {
			return directives
		}
	}

	// Return default
	return x.Default
}

// ExtractPathFromURL extracts the path from a URL for robots.txt matching.
func ExtractPathFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "/"
	}

	path := u.Path
	if path == "" {
		path = "/"
	}

	// Include query string for matching
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}

	return path
}
