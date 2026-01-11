// Package tabs defines the main tab views for Spider UI.
package tabs

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/spider-crawler/spider/internal/ui/components"
)

// TabID identifies a tab.
type TabID string

const (
	TabInternal       TabID = "internal"
	TabExternal       TabID = "external"
	TabResponseCodes  TabID = "response_codes"
	TabURL            TabID = "url"
	TabPageTitles     TabID = "page_titles"
	TabMetaDesc       TabID = "meta_description"
	TabMetaKeywords   TabID = "meta_keywords"
	TabH1             TabID = "h1"
	TabH2             TabID = "h2"
	TabContent        TabID = "content"
	TabImages         TabID = "images"
	TabCanonicals     TabID = "canonicals"
	TabPagination     TabID = "pagination"
	TabDirectives     TabID = "directives"
	TabHreflang       TabID = "hreflang"
	TabJavaScript     TabID = "javascript"
	TabLinks          TabID = "links"
	TabAMP            TabID = "amp"
	TabStructuredData TabID = "structured_data"
	TabSitemaps       TabID = "sitemaps"
	TabPageSpeed      TabID = "pagespeed"
	TabMobile         TabID = "mobile"
	TabAccessibility  TabID = "accessibility"
	TabCustomSearch   TabID = "custom_search"
	TabCustomExtract  TabID = "custom_extraction"
)

// TabConfig defines a tab's configuration.
type TabConfig struct {
	ID      TabID
	Title   string
	Icon    fyne.Resource
	Columns []components.Column
	Filters []string
}

// AllTabs returns all tab configurations.
func AllTabs() []TabConfig {
	return []TabConfig{
		{
			ID:    TabInternal,
			Title: "Internal",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "status_code", Title: "Status Code", Width: 80, Sortable: true, Visible: true},
				{ID: "status", Title: "Status", Width: 100, Sortable: true, Visible: true},
				{ID: "content_type", Title: "Content Type", Width: 120, Sortable: true, Visible: true},
				{ID: "title", Title: "Title", Width: 200, Sortable: true, Visible: true},
				{ID: "depth", Title: "Depth", Width: 60, Sortable: true, Visible: true},
				{ID: "inlinks", Title: "Inlinks", Width: 70, Sortable: true, Visible: true},
				{ID: "outlinks", Title: "Outlinks", Width: 70, Sortable: true, Visible: true},
				{ID: "response_time", Title: "Response Time", Width: 100, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "HTML", "JavaScript", "CSS", "Images", "PDF", "Other"},
		},
		{
			ID:    TabExternal,
			Title: "External",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "status_code", Title: "Status Code", Width: 80, Sortable: true, Visible: true},
				{ID: "anchor_text", Title: "Anchor Text", Width: 150, Sortable: true, Visible: true},
				{ID: "follow", Title: "Follow", Width: 60, Sortable: true, Visible: true},
				{ID: "found_on", Title: "Found On", Width: 200, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Follow", "Nofollow"},
		},
		{
			ID:    TabResponseCodes,
			Title: "Response Codes",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "status_code", Title: "Status Code", Width: 80, Sortable: true, Visible: true},
				{ID: "status", Title: "Status", Width: 120, Sortable: true, Visible: true},
				{ID: "redirect_url", Title: "Redirect URL", Width: 250, Sortable: true, Visible: true},
				{ID: "redirect_type", Title: "Redirect Type", Width: 100, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Success (2xx)", "Redirection (3xx)", "Client Error (4xx)", "Server Error (5xx)"},
		},
		{
			ID:    TabURL,
			Title: "URL",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "length", Title: "Length", Width: 70, Sortable: true, Visible: true},
				{ID: "path_depth", Title: "Path Depth", Width: 80, Sortable: true, Visible: true},
				{ID: "params", Title: "Parameters", Width: 100, Sortable: true, Visible: true},
				{ID: "hash", Title: "Hash", Width: 100, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Over 115 Characters", "Contains Uppercase", "Contains Underscores", "Duplicate"},
		},
		{
			ID:    TabPageTitles,
			Title: "Page Titles",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "title", Title: "Title", Width: 300, Sortable: true, Visible: true},
				{ID: "length", Title: "Length", Width: 70, Sortable: true, Visible: true},
				{ID: "pixel_width", Title: "Pixel Width", Width: 90, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Missing", "Duplicate", "Over 60 Characters", "Below 30 Characters", "Same as H1"},
		},
		{
			ID:    TabMetaDesc,
			Title: "Meta Description",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "meta_description", Title: "Meta Description", Width: 350, Sortable: true, Visible: true},
				{ID: "length", Title: "Length", Width: 70, Sortable: true, Visible: true},
				{ID: "pixel_width", Title: "Pixel Width", Width: 90, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Missing", "Duplicate", "Over 155 Characters", "Below 70 Characters"},
		},
		{
			ID:    TabMetaKeywords,
			Title: "Meta Keywords",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "meta_keywords", Title: "Meta Keywords", Width: 400, Sortable: true, Visible: true},
				{ID: "count", Title: "Keyword Count", Width: 100, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Missing", "Present"},
		},
		{
			ID:    TabH1,
			Title: "H1",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "h1", Title: "H1", Width: 300, Sortable: true, Visible: true},
				{ID: "length", Title: "Length", Width: 70, Sortable: true, Visible: true},
				{ID: "count", Title: "H1 Count", Width: 80, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Missing", "Duplicate", "Multiple", "Over 70 Characters"},
		},
		{
			ID:    TabH2,
			Title: "H2",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "h2", Title: "H2", Width: 300, Sortable: true, Visible: true},
				{ID: "count", Title: "H2 Count", Width: 80, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Missing", "Multiple"},
		},
		{
			ID:    TabContent,
			Title: "Content",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "word_count", Title: "Word Count", Width: 90, Sortable: true, Visible: true},
				{ID: "content_hash", Title: "Content Hash", Width: 150, Sortable: true, Visible: true},
				{ID: "text_ratio", Title: "Text Ratio", Width: 80, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Low Word Count", "Duplicate Content", "Near Duplicate"},
		},
		{
			ID:    TabImages,
			Title: "Images",
			Columns: []components.Column{
				{ID: "url", Title: "Image URL", Width: 300, Sortable: true, Visible: true},
				{ID: "status_code", Title: "Status", Width: 70, Sortable: true, Visible: true},
				{ID: "alt", Title: "Alt Text", Width: 200, Sortable: true, Visible: true},
				{ID: "size", Title: "Size", Width: 80, Sortable: true, Visible: true},
				{ID: "found_on", Title: "Found On", Width: 200, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Missing Alt", "Over 100KB", "Broken (4xx)", "Missing Alt Over 100KB"},
		},
		{
			ID:    TabCanonicals,
			Title: "Canonicals",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "canonical", Title: "Canonical", Width: 300, Sortable: true, Visible: true},
				{ID: "status", Title: "Status", Width: 120, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Missing", "Self-Referencing", "Canonicalised", "Non-Indexable Canonical"},
		},
		{
			ID:    TabDirectives,
			Title: "Directives",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "meta_robots", Title: "Meta Robots", Width: 150, Sortable: true, Visible: true},
				{ID: "x_robots", Title: "X-Robots-Tag", Width: 150, Sortable: true, Visible: true},
				{ID: "indexability", Title: "Indexability", Width: 100, Sortable: true, Visible: true},
				{ID: "reason", Title: "Reason", Width: 150, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Index", "Noindex", "Follow", "Nofollow"},
		},
		{
			ID:    TabHreflang,
			Title: "Hreflang",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 250, Sortable: true, Visible: true},
				{ID: "hreflang", Title: "Hreflang", Width: 80, Sortable: true, Visible: true},
				{ID: "href", Title: "Href", Width: 250, Sortable: true, Visible: true},
				{ID: "status", Title: "Status", Width: 100, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Contains Hreflang", "Missing Return Links", "Inconsistent"},
		},
		{
			ID:    TabJavaScript,
			Title: "JavaScript",
			Columns: []components.Column{
				{ID: "url", Title: "Script URL", Width: 300, Sortable: true, Visible: true},
				{ID: "status_code", Title: "Status", Width: 70, Sortable: true, Visible: true},
				{ID: "size", Title: "Size", Width: 80, Sortable: true, Visible: true},
				{ID: "async", Title: "Async", Width: 60, Sortable: true, Visible: true},
				{ID: "defer", Title: "Defer", Width: 60, Sortable: true, Visible: true},
				{ID: "found_on", Title: "Found On", Width: 200, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Async", "Defer", "Render Blocking", "Broken"},
		},
		{
			ID:    TabLinks,
			Title: "Links",
			Columns: []components.Column{
				{ID: "from_url", Title: "From", Width: 250, Sortable: true, Visible: true},
				{ID: "to_url", Title: "To", Width: 250, Sortable: true, Visible: true},
				{ID: "anchor_text", Title: "Anchor Text", Width: 150, Sortable: true, Visible: true},
				{ID: "rel", Title: "Rel", Width: 100, Sortable: true, Visible: true},
				{ID: "status", Title: "Status", Width: 70, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Internal", "External", "Follow", "Nofollow", "Broken"},
		},
		{
			ID:    TabSitemaps,
			Title: "Sitemaps",
			Columns: []components.Column{
				{ID: "url", Title: "Sitemap URL", Width: 300, Sortable: true, Visible: true},
				{ID: "type", Title: "Type", Width: 80, Sortable: true, Visible: true},
				{ID: "url_count", Title: "URLs", Width: 80, Sortable: true, Visible: true},
				{ID: "status_code", Title: "Status", Width: 70, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Sitemap Index", "Sitemap", "Orphan URLs", "Not In Sitemap"},
		},
		{
			ID:    TabStructuredData,
			Title: "Structured Data",
			Columns: []components.Column{
				{ID: "url", Title: "Address", Width: 300, Sortable: true, Visible: true},
				{ID: "type", Title: "Type", Width: 150, Sortable: true, Visible: true},
				{ID: "format", Title: "Format", Width: 100, Sortable: true, Visible: true},
				{ID: "valid", Title: "Valid", Width: 60, Sortable: true, Visible: true},
				{ID: "errors", Title: "Errors", Width: 200, Sortable: true, Visible: true},
			},
			Filters: []string{"All", "Contains Structured Data", "JSON-LD", "Microdata", "Errors", "Warnings"},
		},
	}
}

// TabView represents a single tab view.
type TabView struct {
	Config   TabConfig
	Table    *components.DataTable
	Filter   *components.FilterBar
	Content  fyne.CanvasObject
}

// NewTabView creates a new tab view.
func NewTabView(config TabConfig) *TabView {
	tv := &TabView{Config: config}

	// Create empty table data
	tableData := &components.TableData{
		Columns: config.Columns,
		Rows:    make([][]string, 0),
	}
	tv.Table = components.NewDataTable(tableData)

	// Create filter bar
	tv.Filter = components.NewFilterBar(config.Filters)
	tv.Filter.OnSearch = func(text string) {
		tv.Table.SetFilter(text)
	}

	// Layout
	tv.Content = container.NewBorder(
		tv.Filter, // top
		nil,       // bottom
		nil,       // left
		nil,       // right
		tv.Table,  // center
	)

	return tv
}

// SetData updates the tab's table data.
func (tv *TabView) SetData(rows [][]string) {
	tv.Table.SetData(&components.TableData{
		Columns: tv.Config.Columns,
		Rows:    rows,
	})
}
