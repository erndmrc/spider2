// Package visualization provides crawl data visualization.
package visualization

import (
	"encoding/json"
	"math"

	"github.com/spider-crawler/spider/internal/storage"
)

// GraphNode represents a node in the crawl graph.
type GraphNode struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	URL         string  `json:"url"`
	URLID       int64   `json:"url_id"`
	Depth       int     `json:"depth"`
	StatusCode  int     `json:"status_code"`
	IsInternal  bool    `json:"is_internal"`
	IsIndexable bool    `json:"is_indexable"`
	ContentType string  `json:"content_type"`
	InLinks     int     `json:"in_links"`   // Incoming links count
	OutLinks    int     `json:"out_links"`  // Outgoing links count
	X           float64 `json:"x"`          // Position for visualization
	Y           float64 `json:"y"`          // Position for visualization
	Size        float64 `json:"size"`       // Node size based on importance
	Color       string  `json:"color"`      // Node color based on status
	Group       string  `json:"group"`      // Grouping for clustering
}

// GraphEdge represents an edge (link) in the crawl graph.
type GraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"` // Source node ID
	Target string `json:"target"` // Target node ID
	Label  string `json:"label"`  // Anchor text
	Type   string `json:"type"`   // Link type (a, link, etc.)
	Weight int    `json:"weight"` // Edge weight (link count)
}

// CrawlGraph represents the complete crawl graph.
type CrawlGraph struct {
	Nodes      []*GraphNode `json:"nodes"`
	Edges      []*GraphEdge `json:"edges"`
	NodeCount  int          `json:"node_count"`
	EdgeCount  int          `json:"edge_count"`
	Density    float64      `json:"density"`    // Graph density
	AvgDegree  float64      `json:"avg_degree"` // Average node degree
}

// GraphFilter defines filtering options for the graph.
type GraphFilter struct {
	InternalOnly   bool     `json:"internal_only"`
	MinStatusCode  int      `json:"min_status_code"`
	MaxStatusCode  int      `json:"max_status_code"`
	MaxDepth       int      `json:"max_depth"`
	ContentTypes   []string `json:"content_types"`
	IndexableOnly  bool     `json:"indexable_only"`
	MinInLinks     int      `json:"min_in_links"`
	MaxNodes       int      `json:"max_nodes"` // Limit nodes for performance
	IncludeOrphans bool     `json:"include_orphans"`
}

// DefaultGraphFilter returns default filter settings.
func DefaultGraphFilter() *GraphFilter {
	return &GraphFilter{
		InternalOnly:   true,
		MinStatusCode:  0,
		MaxStatusCode:  599,
		MaxDepth:       0, // Unlimited
		MaxNodes:       1000,
		IncludeOrphans: true,
	}
}

// GraphBuilder builds crawl graphs from database data.
type GraphBuilder struct {
	db     *storage.Database
	filter *GraphFilter
}

// NewGraphBuilder creates a new graph builder.
func NewGraphBuilder(db *storage.Database) *GraphBuilder {
	return &GraphBuilder{
		db:     db,
		filter: DefaultGraphFilter(),
	}
}

// SetFilter sets the graph filter.
func (g *GraphBuilder) SetFilter(filter *GraphFilter) {
	g.filter = filter
}

// BuildGraph constructs the crawl graph.
func (g *GraphBuilder) BuildGraph() (*CrawlGraph, error) {
	urls, err := g.db.GetAllURLs()
	if err != nil {
		return nil, err
	}

	links, err := g.db.GetAllLinks()
	if err != nil {
		return nil, err
	}

	// Build URL ID to URL map
	urlMap := make(map[int64]*storage.URL)
	for _, url := range urls {
		urlMap[url.ID] = url
	}

	// Count in-links and out-links
	inLinks := make(map[int64]int)
	outLinks := make(map[int64]int)
	for _, link := range links {
		outLinks[link.FromURLID]++
		if link.ToURLID != nil {
			inLinks[*link.ToURLID]++
		}
	}

	// Build nodes
	nodes := make([]*GraphNode, 0)
	nodeMap := make(map[int64]*GraphNode)
	nodeIndex := 0

	for _, url := range urls {
		if !g.passesFilter(url, inLinks[url.ID]) {
			continue
		}

		if g.filter.MaxNodes > 0 && nodeIndex >= g.filter.MaxNodes {
			break
		}

		// Get fetch data
		fetch, _ := g.db.GetLatestFetch(url.ID)
		features, _ := g.db.GetHTMLFeatures(url.ID)

		node := &GraphNode{
			ID:         url.URL,
			Label:      extractLabel(url.Path),
			URL:        url.URL,
			URLID:      url.ID,
			Depth:      url.Depth,
			IsInternal: url.IsInternal,
			InLinks:    inLinks[url.ID],
			OutLinks:   outLinks[url.ID],
		}

		if fetch != nil {
			node.StatusCode = fetch.StatusCode
			node.ContentType = fetch.ContentType
		}

		if features != nil {
			node.IsIndexable = features.IsIndexable
		}

		// Calculate size based on in-links
		node.Size = calculateNodeSize(node.InLinks)

		// Set color based on status
		node.Color = getStatusColor(node.StatusCode)

		// Set group (for clustering)
		node.Group = determineGroup(url)

		nodes = append(nodes, node)
		nodeMap[url.ID] = node
		nodeIndex++
	}

	// Build edges
	edges := make([]*GraphEdge, 0)
	edgeMap := make(map[string]*GraphEdge) // Track unique edges

	for _, link := range links {
		// Get source and target nodes
		_, sourceExists := nodeMap[link.FromURLID]
		var targetExists bool
		if link.ToURLID != nil {
			_, targetExists = nodeMap[*link.ToURLID]
		}

		// Skip if nodes not in graph
		if !sourceExists || (link.ToURLID != nil && !targetExists) {
			continue
		}

		// Get source URL
		sourceURL := urlMap[link.FromURLID]
		if sourceURL == nil {
			continue
		}

		targetURL := link.ToURL
		if link.ToURLID != nil {
			if t := urlMap[*link.ToURLID]; t != nil {
				targetURL = t.URL
			}
		}

		// Create edge key for deduplication
		edgeKey := sourceURL.URL + "->" + targetURL
		if existing, ok := edgeMap[edgeKey]; ok {
			existing.Weight++
			continue
		}

		edge := &GraphEdge{
			ID:     edgeKey,
			Source: sourceURL.URL,
			Target: targetURL,
			Label:  truncate(link.AnchorText, 30),
			Type:   link.LinkType,
			Weight: 1,
		}

		edges = append(edges, edge)
		edgeMap[edgeKey] = edge
	}

	// Calculate positions using force-directed layout
	calculatePositions(nodes, edges)

	// Build graph
	graph := &CrawlGraph{
		Nodes:     nodes,
		Edges:     edges,
		NodeCount: len(nodes),
		EdgeCount: len(edges),
	}

	// Calculate metrics
	if len(nodes) > 0 {
		maxEdges := float64(len(nodes) * (len(nodes) - 1))
		if maxEdges > 0 {
			graph.Density = float64(len(edges)) / maxEdges
		}

		totalDegree := 0
		for _, node := range nodes {
			totalDegree += node.InLinks + node.OutLinks
		}
		graph.AvgDegree = float64(totalDegree) / float64(len(nodes))
	}

	return graph, nil
}

// passesFilter checks if a URL passes the current filter.
func (g *GraphBuilder) passesFilter(url *storage.URL, inLinkCount int) bool {
	// Internal only filter
	if g.filter.InternalOnly && !url.IsInternal {
		return false
	}

	// Depth filter
	if g.filter.MaxDepth > 0 && url.Depth > g.filter.MaxDepth {
		return false
	}

	// In-links filter
	if g.filter.MinInLinks > 0 && inLinkCount < g.filter.MinInLinks {
		return false
	}

	// Get fetch for status code filter
	fetch, _ := g.db.GetLatestFetch(url.ID)
	if fetch != nil {
		if fetch.StatusCode < g.filter.MinStatusCode || fetch.StatusCode > g.filter.MaxStatusCode {
			return false
		}

		// Content type filter
		if len(g.filter.ContentTypes) > 0 {
			found := false
			for _, ct := range g.filter.ContentTypes {
				if fetch.ContentType == ct {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Indexable filter
	if g.filter.IndexableOnly {
		features, _ := g.db.GetHTMLFeatures(url.ID)
		if features == nil || !features.IsIndexable {
			return false
		}
	}

	return true
}

// calculateNodeSize calculates node size based on importance.
func calculateNodeSize(inLinks int) float64 {
	// Base size + log scale for in-links
	base := 5.0
	if inLinks > 0 {
		return base + math.Log10(float64(inLinks+1))*5
	}
	return base
}

// getStatusColor returns color based on HTTP status.
func getStatusColor(status int) string {
	switch {
	case status == 0:
		return "#9E9E9E" // Gray - not fetched
	case status >= 200 && status < 300:
		return "#00C853" // Green - success
	case status >= 300 && status < 400:
		return "#FFD600" // Yellow - redirect
	case status >= 400 && status < 500:
		return "#FF5722" // Orange - client error
	case status >= 500:
		return "#F44336" // Red - server error
	default:
		return "#9E9E9E" // Gray
	}
}

// determineGroup determines the group for a URL (for clustering).
func determineGroup(url *storage.URL) string {
	// Group by path prefix
	if url.Path == "" || url.Path == "/" {
		return "root"
	}

	parts := splitPath(url.Path)
	if len(parts) > 0 {
		return parts[0]
	}
	return "other"
}

// splitPath splits a URL path into segments.
func splitPath(path string) []string {
	parts := make([]string, 0)
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// extractLabel extracts a readable label from URL path.
func extractLabel(path string) string {
	if path == "" || path == "/" {
		return "/"
	}

	parts := splitPath(path)
	if len(parts) > 0 {
		label := parts[len(parts)-1]
		return truncate(label, 25)
	}
	return path
}

// truncate truncates a string to max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// calculatePositions calculates node positions using simple force-directed layout.
func calculatePositions(nodes []*GraphNode, edges []*GraphEdge) {
	if len(nodes) == 0 {
		return
	}

	// Simple circular layout for initial positions
	n := len(nodes)
	radius := float64(n) * 10
	for i, node := range nodes {
		angle := float64(i) * 2 * math.Pi / float64(n)
		node.X = radius * math.Cos(angle)
		node.Y = radius * math.Sin(angle)
	}

	// Build adjacency for force calculation
	nodeIndex := make(map[string]int)
	for i, node := range nodes {
		nodeIndex[node.ID] = i
	}

	// Simple force-directed iterations
	iterations := 50
	k := 100.0 // Optimal distance

	for iter := 0; iter < iterations; iter++ {
		// Calculate repulsion between all nodes
		for i, node1 := range nodes {
			for j, node2 := range nodes {
				if i >= j {
					continue
				}

				dx := node1.X - node2.X
				dy := node1.Y - node2.Y
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist < 1 {
					dist = 1
				}

				// Repulsive force
				force := (k * k) / dist
				fx := dx / dist * force
				fy := dy / dist * force

				node1.X += fx * 0.1
				node1.Y += fy * 0.1
				node2.X -= fx * 0.1
				node2.Y -= fy * 0.1
			}
		}

		// Calculate attraction along edges
		for _, edge := range edges {
			srcIdx, srcOK := nodeIndex[edge.Source]
			tgtIdx, tgtOK := nodeIndex[edge.Target]
			if !srcOK || !tgtOK {
				continue
			}

			src := nodes[srcIdx]
			tgt := nodes[tgtIdx]

			dx := src.X - tgt.X
			dy := src.Y - tgt.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < 1 {
				dist = 1
			}

			// Attractive force
			force := (dist * dist) / k
			fx := dx / dist * force
			fy := dy / dist * force

			src.X -= fx * 0.01
			src.Y -= fy * 0.01
			tgt.X += fx * 0.01
			tgt.Y += fy * 0.01
		}
	}
}

// ToJSON exports the graph as JSON.
func (cg *CrawlGraph) ToJSON() ([]byte, error) {
	return json.MarshalIndent(cg, "", "  ")
}

// GetSubgraph returns a subgraph starting from a specific node.
func (g *GraphBuilder) GetSubgraph(startURL string, depth int) (*CrawlGraph, error) {
	// Find start URL
	url, err := g.db.GetURLByAddress(startURL)
	if err != nil || url == nil {
		return nil, err
	}

	// BFS to collect nodes within depth
	visited := make(map[int64]bool)
	queue := []struct {
		urlID int64
		depth int
	}{{url.ID, 0}}

	visitedURLs := make([]*storage.URL, 0)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current.urlID] || current.depth > depth {
			continue
		}
		visited[current.urlID] = true

		u, _ := g.db.GetURLByID(current.urlID)
		if u != nil {
			visitedURLs = append(visitedURLs, u)
		}

		// Get outgoing links
		links, _ := g.db.GetAllLinks()
		for _, link := range links {
			if link.FromURLID == current.urlID && link.ToURLID != nil {
				queue = append(queue, struct {
					urlID int64
					depth int
				}{*link.ToURLID, current.depth + 1})
			}
		}
	}

	// Build subgraph with limited URLs
	oldFilter := g.filter
	g.filter = &GraphFilter{
		InternalOnly: false,
		MaxNodes:     len(visitedURLs),
	}

	// Temporarily use visited URLs
	graph, err := g.BuildGraph()
	g.filter = oldFilter

	return graph, err
}

// GetNodeStats returns statistics for graph nodes.
type NodeStats struct {
	TotalNodes    int     `json:"total_nodes"`
	InternalNodes int     `json:"internal_nodes"`
	ExternalNodes int     `json:"external_nodes"`
	OrphanNodes   int     `json:"orphan_nodes"`  // No incoming links
	DeadEndNodes  int     `json:"dead_end_nodes"` // No outgoing links
	AvgInLinks    float64 `json:"avg_in_links"`
	AvgOutLinks   float64 `json:"avg_out_links"`
	MaxInLinks    int     `json:"max_in_links"`
	MaxOutLinks   int     `json:"max_out_links"`
}

// GetNodeStats calculates node statistics.
func (cg *CrawlGraph) GetNodeStats() *NodeStats {
	stats := &NodeStats{
		TotalNodes: len(cg.Nodes),
	}

	totalIn := 0
	totalOut := 0

	for _, node := range cg.Nodes {
		if node.IsInternal {
			stats.InternalNodes++
		} else {
			stats.ExternalNodes++
		}

		if node.InLinks == 0 {
			stats.OrphanNodes++
		}
		if node.OutLinks == 0 {
			stats.DeadEndNodes++
		}

		totalIn += node.InLinks
		totalOut += node.OutLinks

		if node.InLinks > stats.MaxInLinks {
			stats.MaxInLinks = node.InLinks
		}
		if node.OutLinks > stats.MaxOutLinks {
			stats.MaxOutLinks = node.OutLinks
		}
	}

	if stats.TotalNodes > 0 {
		stats.AvgInLinks = float64(totalIn) / float64(stats.TotalNodes)
		stats.AvgOutLinks = float64(totalOut) / float64(stats.TotalNodes)
	}

	return stats
}

// GetTopNodes returns top nodes by in-link count.
func (cg *CrawlGraph) GetTopNodes(limit int) []*GraphNode {
	// Copy nodes
	sorted := make([]*GraphNode, len(cg.Nodes))
	copy(sorted, cg.Nodes)

	// Sort by in-links descending
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].InLinks > sorted[i].InLinks {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if limit > len(sorted) {
		limit = len(sorted)
	}
	return sorted[:limit]
}
