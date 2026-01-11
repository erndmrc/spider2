// Package visualization provides crawl data visualization.
package visualization

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/spider-crawler/spider/internal/storage"
)

// TreeNode represents a node in the site structure tree.
type TreeNode struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`        // Folder or page name
	FullPath    string      `json:"full_path"`   // Full URL path
	Type        NodeType    `json:"type"`        // folder or page
	Depth       int         `json:"depth"`       // Depth in tree
	URLCount    int         `json:"url_count"`   // URLs under this node
	Children    []*TreeNode `json:"children"`    // Child nodes
	URL         *storage.URL `json:"url,omitempty"` // Associated URL if page
	StatusCode  int         `json:"status_code,omitempty"`
	ContentType string      `json:"content_type,omitempty"`
	Issues      int         `json:"issues,omitempty"` // Issue count
}

// NodeType defines the type of tree node.
type NodeType string

const (
	NodeTypeFolder NodeType = "folder"
	NodeTypePage   NodeType = "page"
)

// SiteStructure analyzes and visualizes site structure.
type SiteStructure struct {
	db   *storage.Database
	root *TreeNode
}

// NewSiteStructure creates a new site structure analyzer.
func NewSiteStructure(db *storage.Database) *SiteStructure {
	return &SiteStructure{
		db: db,
		root: &TreeNode{
			ID:       "root",
			Name:     "/",
			FullPath: "/",
			Type:     NodeTypeFolder,
			Children: make([]*TreeNode, 0),
		},
	}
}

// BuildTree builds the URL path tree from crawled URLs.
func (s *SiteStructure) BuildTree() (*TreeNode, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	// Reset root
	s.root = &TreeNode{
		ID:       "root",
		Name:     "/",
		FullPath: "/",
		Type:     NodeTypeFolder,
		Depth:    0,
		Children: make([]*TreeNode, 0),
	}

	// Build tree from URLs
	for _, url := range urls {
		if !url.IsInternal {
			continue // Only internal URLs in structure
		}
		s.addURLToTree(url)
	}

	// Calculate URL counts
	s.calculateCounts(s.root)

	return s.root, nil
}

// addURLToTree adds a URL to the appropriate place in the tree.
func (s *SiteStructure) addURLToTree(url *storage.URL) {
	// Parse path segments
	path := strings.TrimPrefix(url.Path, "/")
	if path == "" {
		// Root page
		s.root.URL = url
		s.root.StatusCode = s.getStatusCode(url.ID)
		return
	}

	segments := strings.Split(path, "/")
	current := s.root

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		isLast := i == len(segments)-1
		child := s.findChild(current, segment)

		if child == nil {
			// Create new node
			fullPath := "/" + strings.Join(segments[:i+1], "/")
			nodeType := NodeTypeFolder
			if isLast {
				nodeType = NodeTypePage
			}

			child = &TreeNode{
				ID:       fullPath,
				Name:     segment,
				FullPath: fullPath,
				Type:     nodeType,
				Depth:    i + 1,
				Children: make([]*TreeNode, 0),
			}
			current.Children = append(current.Children, child)
		}

		if isLast {
			child.URL = url
			child.Type = NodeTypePage
			child.StatusCode = s.getStatusCode(url.ID)
			child.ContentType = s.getContentType(url.ID)
		}

		current = child
	}
}

// findChild finds a child node by name.
func (s *SiteStructure) findChild(parent *TreeNode, name string) *TreeNode {
	for _, child := range parent.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

// getStatusCode gets the status code for a URL.
func (s *SiteStructure) getStatusCode(urlID int64) int {
	fetch, err := s.db.GetLatestFetch(urlID)
	if err != nil || fetch == nil {
		return 0
	}
	return fetch.StatusCode
}

// getContentType gets the content type for a URL.
func (s *SiteStructure) getContentType(urlID int64) string {
	fetch, err := s.db.GetLatestFetch(urlID)
	if err != nil || fetch == nil {
		return ""
	}
	return fetch.ContentType
}

// calculateCounts recursively calculates URL counts.
func (s *SiteStructure) calculateCounts(node *TreeNode) int {
	count := 0
	if node.URL != nil {
		count = 1
	}

	for _, child := range node.Children {
		count += s.calculateCounts(child)
	}

	node.URLCount = count
	return count
}

// GetRoot returns the root node.
func (s *SiteStructure) GetRoot() *TreeNode {
	return s.root
}

// DepthDistribution represents URL distribution by depth.
type DepthDistribution struct {
	Depth    int `json:"depth"`
	URLCount int `json:"url_count"`
	Percent  float64 `json:"percent"`
}

// GetDepthDistribution returns URL distribution by crawl depth.
func (s *SiteStructure) GetDepthDistribution() ([]DepthDistribution, error) {
	urls, err := s.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	// Count by depth
	depthCounts := make(map[int]int)
	total := 0
	maxDepth := 0

	for _, url := range urls {
		if url.IsInternal {
			depthCounts[url.Depth]++
			total++
			if url.Depth > maxDepth {
				maxDepth = url.Depth
			}
		}
	}

	// Build distribution
	distribution := make([]DepthDistribution, 0, maxDepth+1)
	for d := 0; d <= maxDepth; d++ {
		count := depthCounts[d]
		percent := 0.0
		if total > 0 {
			percent = float64(count) / float64(total) * 100
		}
		distribution = append(distribution, DepthDistribution{
			Depth:    d,
			URLCount: count,
			Percent:  percent,
		})
	}

	return distribution, nil
}

// StatusDistribution represents URL distribution by status code.
type StatusDistribution struct {
	StatusCode int     `json:"status_code"`
	Status     string  `json:"status"`
	URLCount   int     `json:"url_count"`
	Percent    float64 `json:"percent"`
}

// GetStatusDistribution returns URL distribution by HTTP status code.
func (s *SiteStructure) GetStatusDistribution() ([]StatusDistribution, error) {
	stats, err := s.db.GetCrawlStats()
	if err != nil {
		return nil, err
	}

	// Get status codes from stats
	total := 0
	for _, count := range stats.StatusCodes {
		total += count
	}

	// Build distribution
	var distribution []StatusDistribution
	for code, count := range stats.StatusCodes {
		percent := 0.0
		if total > 0 {
			percent = float64(count) / float64(total) * 100
		}
		distribution = append(distribution, StatusDistribution{
			StatusCode: code,
			Status:     getStatusText(code),
			URLCount:   count,
			Percent:    percent,
		})
	}

	// Sort by status code
	sort.Slice(distribution, func(i, j int) bool {
		return distribution[i].StatusCode < distribution[j].StatusCode
	})

	return distribution, nil
}

// getStatusText returns human-readable status text.
func getStatusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 307:
		return "Temporary Redirect"
	case 308:
		return "Permanent Redirect"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 410:
		return "Gone"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	default:
		if code >= 200 && code < 300 {
			return "Success"
		} else if code >= 300 && code < 400 {
			return "Redirect"
		} else if code >= 400 && code < 500 {
			return "Client Error"
		} else if code >= 500 {
			return "Server Error"
		}
		return "Unknown"
	}
}

// FolderStats represents statistics for a folder.
type FolderStats struct {
	Path          string  `json:"path"`
	URLCount      int     `json:"url_count"`
	AvgDepth      float64 `json:"avg_depth"`
	StatusOK      int     `json:"status_ok"`
	StatusError   int     `json:"status_error"`
	StatusRedirect int    `json:"status_redirect"`
	IssueCount    int     `json:"issue_count"`
}

// GetFolderStats returns statistics for each folder.
func (s *SiteStructure) GetFolderStats() ([]FolderStats, error) {
	if s.root == nil {
		if _, err := s.BuildTree(); err != nil {
			return nil, err
		}
	}

	var stats []FolderStats
	s.collectFolderStats(s.root, &stats)
	return stats, nil
}

// collectFolderStats recursively collects folder statistics.
func (s *SiteStructure) collectFolderStats(node *TreeNode, stats *[]FolderStats) {
	if node.Type == NodeTypeFolder && len(node.Children) > 0 {
		folderStats := s.calculateFolderStats(node)
		*stats = append(*stats, folderStats)

		for _, child := range node.Children {
			s.collectFolderStats(child, stats)
		}
	}
}

// calculateFolderStats calculates statistics for a single folder.
func (s *SiteStructure) calculateFolderStats(node *TreeNode) FolderStats {
	stats := FolderStats{
		Path: node.FullPath,
	}

	var depths []int
	s.countInFolder(node, &stats.URLCount, &depths, &stats.StatusOK, &stats.StatusError, &stats.StatusRedirect)

	if len(depths) > 0 {
		sum := 0
		for _, d := range depths {
			sum += d
		}
		stats.AvgDepth = float64(sum) / float64(len(depths))
	}

	return stats
}

// countInFolder counts URLs within a folder.
func (s *SiteStructure) countInFolder(node *TreeNode, count *int, depths *[]int, ok, errors, redirects *int) {
	if node.URL != nil {
		*count++
		*depths = append(*depths, node.Depth)

		if node.StatusCode >= 200 && node.StatusCode < 300 {
			*ok++
		} else if node.StatusCode >= 300 && node.StatusCode < 400 {
			*redirects++
		} else if node.StatusCode >= 400 {
			*errors++
		}
	}

	for _, child := range node.Children {
		s.countInFolder(child, count, depths, ok, errors, redirects)
	}
}

// ToJSON exports the tree structure as JSON.
func (s *SiteStructure) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s.root, "", "  ")
}

// GetFlatList returns a flat list of all nodes.
func (s *SiteStructure) GetFlatList() []*TreeNode {
	var nodes []*TreeNode
	s.flattenTree(s.root, &nodes)
	return nodes
}

// flattenTree flattens the tree into a list.
func (s *SiteStructure) flattenTree(node *TreeNode, nodes *[]*TreeNode) {
	*nodes = append(*nodes, node)
	for _, child := range node.Children {
		s.flattenTree(child, nodes)
	}
}

// FilterByStatus filters tree nodes by status code range.
func (s *SiteStructure) FilterByStatus(minCode, maxCode int) []*TreeNode {
	var filtered []*TreeNode
	s.filterByStatusRecursive(s.root, minCode, maxCode, &filtered)
	return filtered
}

// filterByStatusRecursive recursively filters by status.
func (s *SiteStructure) filterByStatusRecursive(node *TreeNode, min, max int, results *[]*TreeNode) {
	if node.StatusCode >= min && node.StatusCode <= max {
		*results = append(*results, node)
	}
	for _, child := range node.Children {
		s.filterByStatusRecursive(child, min, max, results)
	}
}

// FilterByDepth filters tree nodes by depth.
func (s *SiteStructure) FilterByDepth(depth int) []*TreeNode {
	var filtered []*TreeNode
	s.filterByDepthRecursive(s.root, depth, &filtered)
	return filtered
}

// filterByDepthRecursive recursively filters by depth.
func (s *SiteStructure) filterByDepthRecursive(node *TreeNode, depth int, results *[]*TreeNode) {
	if node.Depth == depth {
		*results = append(*results, node)
	}
	for _, child := range node.Children {
		s.filterByDepthRecursive(child, depth, results)
	}
}
