// Package visualization provides crawl data visualization.
package visualization

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// SegmentType defines the type of segmentation.
type SegmentType string

const (
	SegmentByContentType  SegmentType = "content_type"
	SegmentByStatusCode   SegmentType = "status_code"
	SegmentByIndexability SegmentType = "indexability"
	SegmentByTemplate     SegmentType = "template"
	SegmentByDepth        SegmentType = "depth"
	SegmentByHost         SegmentType = "host"
	SegmentByFolder       SegmentType = "folder"
	SegmentByCustom       SegmentType = "custom"
)

// Segment represents a group of URLs sharing common characteristics.
type Segment struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Type        SegmentType         `json:"type"`
	Value       string              `json:"value"` // The value that defines this segment
	URLCount    int                 `json:"url_count"`
	URLs        []*storage.URL      `json:"urls,omitempty"`
	Metrics     *SegmentMetrics     `json:"metrics"`
	SubSegments []*Segment          `json:"sub_segments,omitempty"` // Nested segments
}

// SegmentMetrics contains metrics for a segment.
type SegmentMetrics struct {
	AvgResponseTime   float64 `json:"avg_response_time_ms"`
	AvgWordCount      float64 `json:"avg_word_count"`
	AvgDepth          float64 `json:"avg_depth"`
	IndexableCount    int     `json:"indexable_count"`
	NonIndexableCount int     `json:"non_indexable_count"`
	IssueCount        int     `json:"issue_count"`
	Status2xx         int     `json:"status_2xx"`
	Status3xx         int     `json:"status_3xx"`
	Status4xx         int     `json:"status_4xx"`
	Status5xx         int     `json:"status_5xx"`
}

// SegmentRule defines a custom segmentation rule.
type SegmentRule struct {
	Name          string      `json:"name"`
	Type          SegmentType `json:"type"`
	Pattern       string      `json:"pattern,omitempty"`       // Regex pattern for URL matching
	CompiledRegex *regexp.Regexp `json:"-"`
	Values        []string    `json:"values,omitempty"`        // Specific values to match
}

// Segmenter creates segments from crawl data.
type Segmenter struct {
	db       *storage.Database
	rules    []*SegmentRule
	segments map[string]*Segment
}

// NewSegmenter creates a new segmenter.
func NewSegmenter(db *storage.Database) *Segmenter {
	return &Segmenter{
		db:       db,
		rules:    make([]*SegmentRule, 0),
		segments: make(map[string]*Segment),
	}
}

// AddRule adds a custom segmentation rule.
func (s *Segmenter) AddRule(rule *SegmentRule) error {
	if rule.Pattern != "" {
		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return err
		}
		rule.CompiledRegex = compiled
	}
	s.rules = append(s.rules, rule)
	return nil
}

// SegmentByContentType segments URLs by content type.
func (s *Segmenter) SegmentByContentType() ([]*Segment, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	segments := make(map[string]*Segment)

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		fetch, _ := s.db.GetLatestFetch(url.ID)
		contentType := "unknown"
		if fetch != nil && fetch.ContentType != "" {
			contentType = normalizeContentType(fetch.ContentType)
		}

		if _, ok := segments[contentType]; !ok {
			segments[contentType] = &Segment{
				ID:       "content_type_" + sanitizeID(contentType),
				Name:     contentType,
				Type:     SegmentByContentType,
				Value:    contentType,
				URLs:     make([]*storage.URL, 0),
				Metrics:  &SegmentMetrics{},
			}
		}

		seg := segments[contentType]
		seg.URLs = append(seg.URLs, url)
		seg.URLCount++
		s.updateMetrics(seg.Metrics, url, fetch)
	}

	return s.finalize(segments)
}

// SegmentByStatusCode segments URLs by HTTP status code category.
func (s *Segmenter) SegmentByStatusCode() ([]*Segment, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	segments := make(map[string]*Segment)
	categories := []struct {
		min, max int
		name     string
	}{
		{200, 299, "2xx Success"},
		{300, 399, "3xx Redirect"},
		{400, 499, "4xx Client Error"},
		{500, 599, "5xx Server Error"},
		{0, 0, "Not Fetched"},
	}

	// Initialize segments
	for _, cat := range categories {
		key := sanitizeID(cat.name)
		segments[key] = &Segment{
			ID:      "status_" + key,
			Name:    cat.name,
			Type:    SegmentByStatusCode,
			Value:   cat.name,
			URLs:    make([]*storage.URL, 0),
			Metrics: &SegmentMetrics{},
		}
	}

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		fetch, _ := s.db.GetLatestFetch(url.ID)
		statusCode := 0
		if fetch != nil {
			statusCode = fetch.StatusCode
		}

		var segKey string
		for _, cat := range categories {
			if statusCode >= cat.min && statusCode <= cat.max {
				segKey = sanitizeID(cat.name)
				break
			}
		}
		if segKey == "" {
			segKey = "not_fetched"
		}

		if seg, ok := segments[segKey]; ok {
			seg.URLs = append(seg.URLs, url)
			seg.URLCount++
			s.updateMetrics(seg.Metrics, url, fetch)
		}
	}

	return s.finalize(segments)
}

// SegmentByIndexability segments URLs by indexability.
func (s *Segmenter) SegmentByIndexability() ([]*Segment, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	segments := map[string]*Segment{
		"indexable": {
			ID:      "indexability_indexable",
			Name:    "Indexable",
			Type:    SegmentByIndexability,
			Value:   "indexable",
			URLs:    make([]*storage.URL, 0),
			Metrics: &SegmentMetrics{},
		},
		"non_indexable": {
			ID:      "indexability_non_indexable",
			Name:    "Non-Indexable",
			Type:    SegmentByIndexability,
			Value:   "non_indexable",
			URLs:    make([]*storage.URL, 0),
			Metrics: &SegmentMetrics{},
		},
	}

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		features, _ := s.db.GetHTMLFeatures(url.ID)
		fetch, _ := s.db.GetLatestFetch(url.ID)

		key := "non_indexable"
		if features != nil && features.IsIndexable {
			key = "indexable"
		}

		seg := segments[key]
		seg.URLs = append(seg.URLs, url)
		seg.URLCount++
		s.updateMetrics(seg.Metrics, url, fetch)
	}

	return s.finalize(segments)
}

// SegmentByTemplate segments URLs by URL template pattern.
func (s *Segmenter) SegmentByTemplate() ([]*Segment, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	segments := make(map[string]*Segment)

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		template := extractTemplate(url.Path)
		if template == "" {
			template = "/"
		}

		if _, ok := segments[template]; !ok {
			segments[template] = &Segment{
				ID:      "template_" + sanitizeID(template),
				Name:    template,
				Type:    SegmentByTemplate,
				Value:   template,
				URLs:    make([]*storage.URL, 0),
				Metrics: &SegmentMetrics{},
			}
		}

		fetch, _ := s.db.GetLatestFetch(url.ID)
		seg := segments[template]
		seg.URLs = append(seg.URLs, url)
		seg.URLCount++
		s.updateMetrics(seg.Metrics, url, fetch)
	}

	return s.finalize(segments)
}

// SegmentByDepth segments URLs by crawl depth.
func (s *Segmenter) SegmentByDepth() ([]*Segment, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	segments := make(map[int]*Segment)

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		if _, ok := segments[url.Depth]; !ok {
			depthStr := string(rune('0' + url.Depth))
			if url.Depth > 9 {
				depthStr = strings.Repeat("0", url.Depth/10) + string(rune('0'+url.Depth%10))
			}
			segments[url.Depth] = &Segment{
				ID:      "depth_" + depthStr,
				Name:    "Depth " + depthStr,
				Type:    SegmentByDepth,
				Value:   depthStr,
				URLs:    make([]*storage.URL, 0),
				Metrics: &SegmentMetrics{},
			}
		}

		fetch, _ := s.db.GetLatestFetch(url.ID)
		seg := segments[url.Depth]
		seg.URLs = append(seg.URLs, url)
		seg.URLCount++
		s.updateMetrics(seg.Metrics, url, fetch)
	}

	// Convert to slice
	result := make([]*Segment, 0, len(segments))
	for _, seg := range segments {
		s.calculateAverages(seg)
		result = append(result, seg)
	}

	return result, nil
}

// SegmentByFolder segments URLs by top-level folder.
func (s *Segmenter) SegmentByFolder() ([]*Segment, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	segments := make(map[string]*Segment)

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		folder := extractTopFolder(url.Path)

		if _, ok := segments[folder]; !ok {
			segments[folder] = &Segment{
				ID:      "folder_" + sanitizeID(folder),
				Name:    folder,
				Type:    SegmentByFolder,
				Value:   folder,
				URLs:    make([]*storage.URL, 0),
				Metrics: &SegmentMetrics{},
			}
		}

		fetch, _ := s.db.GetLatestFetch(url.ID)
		seg := segments[folder]
		seg.URLs = append(seg.URLs, url)
		seg.URLCount++
		s.updateMetrics(seg.Metrics, url, fetch)
	}

	return s.finalize(segments)
}

// SegmentByHost segments URLs by hostname.
func (s *Segmenter) SegmentByHost() ([]*Segment, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	segments := make(map[string]*Segment)

	for _, url := range urls {
		host := url.Host

		if _, ok := segments[host]; !ok {
			segments[host] = &Segment{
				ID:      "host_" + sanitizeID(host),
				Name:    host,
				Type:    SegmentByHost,
				Value:   host,
				URLs:    make([]*storage.URL, 0),
				Metrics: &SegmentMetrics{},
			}
		}

		fetch, _ := s.db.GetLatestFetch(url.ID)
		seg := segments[host]
		seg.URLs = append(seg.URLs, url)
		seg.URLCount++
		s.updateMetrics(seg.Metrics, url, fetch)
	}

	return s.finalize(segments)
}

// SegmentByCustomRules segments URLs by custom rules.
func (s *Segmenter) SegmentByCustomRules() ([]*Segment, error) {
	if len(s.rules) == 0 {
		return []*Segment{}, nil
	}

	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	segments := make(map[string]*Segment)

	// Initialize segments from rules
	for _, rule := range s.rules {
		key := sanitizeID(rule.Name)
		segments[key] = &Segment{
			ID:      "custom_" + key,
			Name:    rule.Name,
			Type:    SegmentByCustom,
			Value:   rule.Pattern,
			URLs:    make([]*storage.URL, 0),
			Metrics: &SegmentMetrics{},
		}
	}

	// Add "Other" segment for non-matching URLs
	segments["other"] = &Segment{
		ID:      "custom_other",
		Name:    "Other",
		Type:    SegmentByCustom,
		Value:   "",
		URLs:    make([]*storage.URL, 0),
		Metrics: &SegmentMetrics{},
	}

	for _, url := range urls {
		if !url.IsInternal {
			continue
		}

		fetch, _ := s.db.GetLatestFetch(url.ID)
		matched := false

		for _, rule := range s.rules {
			if rule.CompiledRegex != nil && rule.CompiledRegex.MatchString(url.URL) {
				key := sanitizeID(rule.Name)
				seg := segments[key]
				seg.URLs = append(seg.URLs, url)
				seg.URLCount++
				s.updateMetrics(seg.Metrics, url, fetch)
				matched = true
				break // URL goes to first matching segment
			}
		}

		if !matched {
			seg := segments["other"]
			seg.URLs = append(seg.URLs, url)
			seg.URLCount++
			s.updateMetrics(seg.Metrics, url, fetch)
		}
	}

	return s.finalize(segments)
}

// updateMetrics updates segment metrics with URL data.
func (s *Segmenter) updateMetrics(metrics *SegmentMetrics, url *storage.URL, fetch *storage.Fetch) {
	metrics.AvgDepth += float64(url.Depth)

	if fetch != nil {
		metrics.AvgResponseTime += float64(fetch.ResponseTime.Milliseconds())

		switch {
		case fetch.StatusCode >= 200 && fetch.StatusCode < 300:
			metrics.Status2xx++
		case fetch.StatusCode >= 300 && fetch.StatusCode < 400:
			metrics.Status3xx++
		case fetch.StatusCode >= 400 && fetch.StatusCode < 500:
			metrics.Status4xx++
		case fetch.StatusCode >= 500:
			metrics.Status5xx++
		}
	}

	features, _ := s.db.GetHTMLFeatures(url.ID)
	if features != nil {
		metrics.AvgWordCount += float64(features.WordCount)
		if features.IsIndexable {
			metrics.IndexableCount++
		} else {
			metrics.NonIndexableCount++
		}
	}
}

// calculateAverages calculates average values for metrics.
func (s *Segmenter) calculateAverages(seg *Segment) {
	if seg.URLCount > 0 {
		seg.Metrics.AvgResponseTime /= float64(seg.URLCount)
		seg.Metrics.AvgWordCount /= float64(seg.URLCount)
		seg.Metrics.AvgDepth /= float64(seg.URLCount)
	}
}

// finalize finalizes segments and calculates averages.
func (s *Segmenter) finalize(segments map[string]*Segment) ([]*Segment, error) {
	result := make([]*Segment, 0, len(segments))
	for _, seg := range segments {
		if seg.URLCount > 0 {
			s.calculateAverages(seg)
			result = append(result, seg)
		}
	}
	return result, nil
}

// GetAllSegments returns all segment types at once.
func (s *Segmenter) GetAllSegments() (map[SegmentType][]*Segment, error) {
	result := make(map[SegmentType][]*Segment)

	contentType, _ := s.SegmentByContentType()
	result[SegmentByContentType] = contentType

	statusCode, _ := s.SegmentByStatusCode()
	result[SegmentByStatusCode] = statusCode

	indexability, _ := s.SegmentByIndexability()
	result[SegmentByIndexability] = indexability

	template, _ := s.SegmentByTemplate()
	result[SegmentByTemplate] = template

	depth, _ := s.SegmentByDepth()
	result[SegmentByDepth] = depth

	folder, _ := s.SegmentByFolder()
	result[SegmentByFolder] = folder

	if len(s.rules) > 0 {
		custom, _ := s.SegmentByCustomRules()
		result[SegmentByCustom] = custom
	}

	return result, nil
}

// CompareSegments compares two segments.
type SegmentComparison struct {
	Segment1    *Segment `json:"segment1"`
	Segment2    *Segment `json:"segment2"`
	CommonURLs  int      `json:"common_urls"`
	Unique1     int      `json:"unique_in_segment1"`
	Unique2     int      `json:"unique_in_segment2"`
	Overlap     float64  `json:"overlap_percent"`
}

// CompareSegments compares two segments.
func CompareSegments(seg1, seg2 *Segment) *SegmentComparison {
	urls1 := make(map[int64]bool)
	for _, url := range seg1.URLs {
		urls1[url.ID] = true
	}

	common := 0
	for _, url := range seg2.URLs {
		if urls1[url.ID] {
			common++
		}
	}

	totalUnique := seg1.URLCount + seg2.URLCount - common
	overlap := 0.0
	if totalUnique > 0 {
		overlap = float64(common) / float64(totalUnique) * 100
	}

	return &SegmentComparison{
		Segment1:   seg1,
		Segment2:   seg2,
		CommonURLs: common,
		Unique1:    seg1.URLCount - common,
		Unique2:    seg2.URLCount - common,
		Overlap:    overlap,
	}
}

// ToJSON exports segment as JSON.
func (seg *Segment) ToJSON() ([]byte, error) {
	return json.MarshalIndent(seg, "", "  ")
}

// Helper functions

// normalizeContentType extracts main content type.
func normalizeContentType(ct string) string {
	// Remove charset and other params
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = ct[:idx]
	}
	return strings.TrimSpace(ct)
}

// sanitizeID creates a safe ID from a string.
func sanitizeID(s string) string {
	result := strings.ToLower(s)
	result = strings.ReplaceAll(result, " ", "_")
	result = strings.ReplaceAll(result, "/", "_")
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ReplaceAll(result, "-", "_")
	return result
}

// extractTemplate extracts a URL template pattern.
func extractTemplate(path string) string {
	if path == "" || path == "/" {
		return "/"
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return "/"
	}

	// Replace numeric and ID-like segments with placeholders
	for i, part := range parts {
		// Check if it looks like an ID (numeric, UUID-like, etc.)
		if isIDSegment(part) {
			parts[i] = "{id}"
		}
	}

	return "/" + strings.Join(parts, "/")
}

// isIDSegment checks if a path segment looks like an ID.
func isIDSegment(s string) bool {
	// All numeric
	allNumeric := true
	for _, c := range s {
		if c < '0' || c > '9' {
			allNumeric = false
			break
		}
	}
	if allNumeric && len(s) > 0 {
		return true
	}

	// UUID-like (contains mostly hex and dashes)
	if len(s) >= 32 {
		hexCount := 0
		for _, c := range s {
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-' {
				hexCount++
			}
		}
		if float64(hexCount)/float64(len(s)) > 0.9 {
			return true
		}
	}

	return false
}

// extractTopFolder extracts the top-level folder from path.
func extractTopFolder(path string) string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "/"
	}

	if idx := strings.Index(path, "/"); idx != -1 {
		return "/" + path[:idx]
	}

	// Check if it's a file (has extension)
	if strings.Contains(path, ".") {
		return "/"
	}

	return "/" + path
}
