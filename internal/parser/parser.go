// Package parser handles HTML parsing and data extraction.
package parser

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// PageData contains all extracted data from an HTML page.
type PageData struct {
	// Title tag content
	Title string

	// Meta description
	MetaDescription string

	// Meta keywords
	MetaKeywords string

	// Meta robots content
	MetaRobots string

	// Canonical URL
	Canonical string

	// Headings
	H1 []string
	H2 []string
	H3 []string

	// Links found on page
	Links []Link

	// Images found on page
	Images []Image

	// Scripts (external JS files)
	Scripts []Resource

	// Stylesheets (external CSS files)
	Stylesheets []Resource

	// Hreflang tags
	Hreflangs []Hreflang

	// Open Graph data
	OpenGraph map[string]string

	// Twitter Card data
	TwitterCard map[string]string

	// Other meta tags
	MetaTags map[string]string

	// Base URL if <base> tag present
	BaseURL string

	// Language from html lang attribute
	Language string

	// Word count (visible text)
	WordCount int

	// Text content (for content hash)
	TextContent string
}

// Link represents a link found on the page.
type Link struct {
	URL        string
	Text       string // Anchor text
	Rel        string // rel attribute (nofollow, sponsored, ugc, etc.)
	Type       string // link type: a, area, link
	IsInternal bool   // Will be set by crawler based on domain
	NoFollow   bool   // Has rel="nofollow"
}

// Image represents an image found on the page.
type Image struct {
	Src      string
	Alt      string
	Title    string
	Width    string
	Height   string
	Loading  string // lazy, eager
	IsDataSrc bool  // Has data-src (lazy loading)
}

// Resource represents an external resource (JS, CSS).
type Resource struct {
	URL   string
	Type  string
	Async bool
	Defer bool
}

// Hreflang represents an hreflang tag.
type Hreflang struct {
	Hreflang string // Language/region code
	URL      string
}

// Parser parses HTML content.
type Parser struct {
	baseURL *url.URL
}

// NewParser creates a new HTML parser.
func NewParser(baseURL string) (*Parser, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &Parser{baseURL: u}, nil
}

// Parse parses HTML content and extracts page data.
func (p *Parser) Parse(htmlContent []byte) (*PageData, error) {
	doc, err := html.Parse(bytes.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	data := &PageData{
		H1:          make([]string, 0),
		H2:          make([]string, 0),
		H3:          make([]string, 0),
		Links:       make([]Link, 0),
		Images:      make([]Image, 0),
		Scripts:     make([]Resource, 0),
		Stylesheets: make([]Resource, 0),
		Hreflangs:   make([]Hreflang, 0),
		OpenGraph:   make(map[string]string),
		TwitterCard: make(map[string]string),
		MetaTags:    make(map[string]string),
	}

	var textBuilder strings.Builder
	p.traverse(doc, data, &textBuilder)

	// Calculate word count
	data.TextContent = textBuilder.String()
	data.WordCount = countWords(data.TextContent)

	return data, nil
}

// traverse recursively traverses the HTML tree.
func (p *Parser) traverse(n *html.Node, data *PageData, textBuilder *strings.Builder) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "html":
			data.Language = getAttr(n, "lang")

		case "base":
			if href := getAttr(n, "href"); href != "" {
				data.BaseURL = href
				// Update parser base URL
				if u, err := url.Parse(href); err == nil {
					p.baseURL = p.baseURL.ResolveReference(u)
				}
			}

		case "title":
			data.Title = getTextContent(n)

		case "meta":
			p.parseMeta(n, data)

		case "link":
			p.parseLink(n, data)

		case "a":
			link := p.parseAnchor(n)
			if link.URL != "" {
				data.Links = append(data.Links, link)
			}

		case "img":
			img := p.parseImage(n)
			data.Images = append(data.Images, img)

		case "script":
			if src := getAttr(n, "src"); src != "" {
				data.Scripts = append(data.Scripts, Resource{
					URL:   p.resolveURL(src),
					Type:  getAttr(n, "type"),
					Async: hasAttr(n, "async"),
					Defer: hasAttr(n, "defer"),
				})
			}

		case "h1":
			text := strings.TrimSpace(getTextContent(n))
			if text != "" {
				data.H1 = append(data.H1, text)
			}

		case "h2":
			text := strings.TrimSpace(getTextContent(n))
			if text != "" {
				data.H2 = append(data.H2, text)
			}

		case "h3":
			text := strings.TrimSpace(getTextContent(n))
			if text != "" {
				data.H3 = append(data.H3, text)
			}
		}
	}

	// Collect text content (skip script/style)
	if n.Type == html.TextNode {
		parent := n.Parent
		if parent != nil && parent.Data != "script" && parent.Data != "style" {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				textBuilder.WriteString(text)
				textBuilder.WriteString(" ")
			}
		}
	}

	// Recurse into children
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.traverse(c, data, textBuilder)
	}
}

// parseMeta parses a meta tag.
func (p *Parser) parseMeta(n *html.Node, data *PageData) {
	name := strings.ToLower(getAttr(n, "name"))
	property := strings.ToLower(getAttr(n, "property"))
	content := getAttr(n, "content")
	httpEquiv := strings.ToLower(getAttr(n, "http-equiv"))

	switch {
	case name == "description":
		data.MetaDescription = content
	case name == "keywords":
		data.MetaKeywords = content
	case name == "robots":
		data.MetaRobots = content
	case strings.HasPrefix(property, "og:"):
		data.OpenGraph[property] = content
	case strings.HasPrefix(name, "twitter:") || strings.HasPrefix(property, "twitter:"):
		key := name
		if key == "" {
			key = property
		}
		data.TwitterCard[key] = content
	case httpEquiv == "content-type":
		// Content-Type meta
		data.MetaTags["content-type"] = content
	case httpEquiv == "refresh":
		data.MetaTags["refresh"] = content
	default:
		if name != "" {
			data.MetaTags[name] = content
		}
	}
}

// parseLink parses a link tag.
func (p *Parser) parseLink(n *html.Node, data *PageData) {
	rel := strings.ToLower(getAttr(n, "rel"))
	href := getAttr(n, "href")

	switch rel {
	case "canonical":
		data.Canonical = p.resolveURL(href)
	case "stylesheet":
		data.Stylesheets = append(data.Stylesheets, Resource{
			URL:  p.resolveURL(href),
			Type: "text/css",
		})
	case "alternate":
		// Check for hreflang
		if hreflang := getAttr(n, "hreflang"); hreflang != "" {
			data.Hreflangs = append(data.Hreflangs, Hreflang{
				Hreflang: hreflang,
				URL:      p.resolveURL(href),
			})
		}
	}
}

// parseAnchor parses an anchor tag.
func (p *Parser) parseAnchor(n *html.Node) Link {
	href := getAttr(n, "href")
	rel := strings.ToLower(getAttr(n, "rel"))

	// Skip empty, javascript:, mailto:, tel: links
	if href == "" || strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") ||
		strings.HasPrefix(href, "#") {
		return Link{}
	}

	return Link{
		URL:      p.resolveURL(href),
		Text:     strings.TrimSpace(getTextContent(n)),
		Rel:      rel,
		Type:     "a",
		NoFollow: strings.Contains(rel, "nofollow"),
	}
}

// parseImage parses an img tag.
func (p *Parser) parseImage(n *html.Node) Image {
	src := getAttr(n, "src")
	dataSrc := getAttr(n, "data-src")

	img := Image{
		Alt:     getAttr(n, "alt"),
		Title:   getAttr(n, "title"),
		Width:   getAttr(n, "width"),
		Height:  getAttr(n, "height"),
		Loading: getAttr(n, "loading"),
	}

	// Prefer data-src for lazy-loaded images
	if dataSrc != "" {
		img.Src = p.resolveURL(dataSrc)
		img.IsDataSrc = true
	} else if src != "" {
		img.Src = p.resolveURL(src)
	}

	return img
}

// resolveURL resolves a relative URL against the base URL.
func (p *Parser) resolveURL(href string) string {
	if href == "" {
		return ""
	}

	ref, err := url.Parse(href)
	if err != nil {
		return href
	}

	return p.baseURL.ResolveReference(ref).String()
}

// Helper functions

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func getTextContent(n *html.Node) string {
	var buf bytes.Buffer
	collectText(n, &buf)
	return buf.String()
}

func collectText(n *html.Node, buf *bytes.Buffer) {
	if n.Type == html.TextNode {
		buf.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectText(c, buf)
	}
}

func countWords(text string) int {
	words := strings.Fields(text)
	return len(words)
}

// ParseHTML is a convenience function to parse HTML from bytes.
func ParseHTML(baseURL string, content []byte) (*PageData, error) {
	parser, err := NewParser(baseURL)
	if err != nil {
		return nil, err
	}
	return parser.Parse(content)
}

// ParseHTMLReader parses HTML from an io.Reader.
func ParseHTMLReader(baseURL string, r io.Reader) (*PageData, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseHTML(baseURL, content)
}

// ExtractLinks extracts only links from HTML content.
func ExtractLinks(baseURL string, content []byte) ([]Link, error) {
	data, err := ParseHTML(baseURL, content)
	if err != nil {
		return nil, err
	}
	return data.Links, nil
}
