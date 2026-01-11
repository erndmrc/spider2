// Package perf provides performance optimization utilities.
package perf

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

// StreamingParser provides memory-efficient HTML parsing for large documents.
type StreamingParser struct {
	mu sync.Mutex

	// Parsing state
	inHead       bool
	inBody       bool
	inScript     bool
	inStyle      bool
	currentDepth int

	// Extracted data
	title       string
	metaTags    []MetaTag
	links       []ExtractedLink
	headings    []Heading
	images      []Image
	scripts     []Script
	stylesheets []Stylesheet

	// Callbacks for streaming processing
	onTitle      func(string)
	onMeta       func(MetaTag)
	onLink       func(ExtractedLink)
	onHeading    func(Heading)
	onImage      func(Image)
	onScript     func(Script)
	onStylesheet func(Stylesheet)

	// Memory limits
	maxTitleLen   int
	maxMetaLen    int
	maxAnchorLen  int
	maxLinks      int
	maxImages     int
	maxHeadings   int
}

// MetaTag represents a meta tag.
type MetaTag struct {
	Name       string
	Property   string
	Content    string
	HTTPEquiv  string
}

// ExtractedLink represents an extracted link.
type ExtractedLink struct {
	Href       string
	Anchor     string
	Rel        string
	Type       string
	IsInternal bool
	NoFollow   bool
}

// Heading represents a heading element.
type Heading struct {
	Level   int    // 1-6
	Text    string
	ID      string
}

// Image represents an image element.
type Image struct {
	Src    string
	Alt    string
	Width  string
	Height string
	Lazy   bool
}

// Script represents a script element.
type Script struct {
	Src   string
	Async bool
	Defer bool
	Type  string
}

// Stylesheet represents a stylesheet link.
type Stylesheet struct {
	Href  string
	Media string
}

// NewStreamingParser creates a new streaming parser.
func NewStreamingParser() *StreamingParser {
	return &StreamingParser{
		metaTags:      make([]MetaTag, 0, 50),
		links:         make([]ExtractedLink, 0, 500),
		headings:      make([]Heading, 0, 50),
		images:        make([]Image, 0, 100),
		scripts:       make([]Script, 0, 50),
		stylesheets:   make([]Stylesheet, 0, 20),
		maxTitleLen:   500,
		maxMetaLen:    1000,
		maxAnchorLen:  500,
		maxLinks:      10000,
		maxImages:     5000,
		maxHeadings:   500,
	}
}

// SetCallbacks sets streaming callbacks.
func (p *StreamingParser) SetCallbacks(
	onTitle func(string),
	onMeta func(MetaTag),
	onLink func(ExtractedLink),
) {
	p.onTitle = onTitle
	p.onMeta = onMeta
	p.onLink = onLink
}

// SetLimits sets memory limits for extracted data.
func (p *StreamingParser) SetLimits(maxLinks, maxImages, maxHeadings int) {
	p.maxLinks = maxLinks
	p.maxImages = maxImages
	p.maxHeadings = maxHeadings
}

// Parse parses HTML from a reader in a streaming fashion.
func (p *StreamingParser) Parse(r io.Reader) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Use buffered reader for efficiency
	br := bufio.NewReaderSize(r, 64*1024) // 64KB buffer

	tokenizer := html.NewTokenizer(br)

	var textBuffer bytes.Buffer
	var currentTag string

	for {
		tt := tokenizer.Next()

		switch tt {
		case html.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				return nil
			}
			return err

		case html.StartTagToken, html.SelfClosingTagToken:
			tn, hasAttr := tokenizer.TagName()
			tagName := string(tn)

			switch tagName {
			case "head":
				p.inHead = true
			case "body":
				p.inBody = true
				p.inHead = false
			case "script":
				p.inScript = true
				p.processScript(tokenizer, hasAttr)
			case "style":
				p.inStyle = true
			case "title":
				currentTag = "title"
				textBuffer.Reset()
			case "meta":
				p.processMeta(tokenizer, hasAttr)
			case "a":
				p.processLink(tokenizer, hasAttr, &textBuffer)
				currentTag = "a"
			case "link":
				p.processLinkTag(tokenizer, hasAttr)
			case "h1", "h2", "h3", "h4", "h5", "h6":
				currentTag = tagName
				textBuffer.Reset()
			case "img":
				p.processImage(tokenizer, hasAttr)
			}

			p.currentDepth++

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			tagName := string(tn)

			switch tagName {
			case "head":
				p.inHead = false
			case "body":
				p.inBody = false
			case "script":
				p.inScript = false
			case "style":
				p.inStyle = false
			case "title":
				if currentTag == "title" {
					p.title = truncate(textBuffer.String(), p.maxTitleLen)
					if p.onTitle != nil {
						p.onTitle(p.title)
					}
				}
				currentTag = ""
			case "a":
				if currentTag == "a" && len(p.links) > 0 {
					// Update anchor text for last link
					p.links[len(p.links)-1].Anchor = truncate(textBuffer.String(), p.maxAnchorLen)
				}
				currentTag = ""
			case "h1", "h2", "h3", "h4", "h5", "h6":
				if currentTag == tagName && len(p.headings) < p.maxHeadings {
					level := int(tagName[1] - '0')
					p.headings = append(p.headings, Heading{
						Level: level,
						Text:  truncate(strings.TrimSpace(textBuffer.String()), 500),
					})
				}
				currentTag = ""
			}

			p.currentDepth--

		case html.TextToken:
			if p.inScript || p.inStyle {
				continue
			}

			if currentTag != "" {
				text := tokenizer.Text()
				textBuffer.Write(text)
			}
		}
	}
}

// processMeta extracts meta tag information.
func (p *StreamingParser) processMeta(tokenizer *html.Tokenizer, hasAttr bool) {
	if !hasAttr {
		return
	}

	meta := MetaTag{}

	for {
		key, val, more := tokenizer.TagAttr()
		k := string(key)
		v := string(val)

		switch k {
		case "name":
			meta.Name = v
		case "property":
			meta.Property = v
		case "content":
			meta.Content = truncate(v, p.maxMetaLen)
		case "http-equiv":
			meta.HTTPEquiv = v
		}

		if !more {
			break
		}
	}

	if meta.Name != "" || meta.Property != "" || meta.HTTPEquiv != "" {
		p.metaTags = append(p.metaTags, meta)
		if p.onMeta != nil {
			p.onMeta(meta)
		}
	}
}

// processLink extracts anchor link information.
func (p *StreamingParser) processLink(tokenizer *html.Tokenizer, hasAttr bool, textBuffer *bytes.Buffer) {
	if !hasAttr || len(p.links) >= p.maxLinks {
		return
	}

	link := ExtractedLink{}
	textBuffer.Reset()

	for {
		key, val, more := tokenizer.TagAttr()
		k := string(key)
		v := string(val)

		switch k {
		case "href":
			link.Href = v
		case "rel":
			link.Rel = v
			if strings.Contains(v, "nofollow") {
				link.NoFollow = true
			}
		case "type":
			link.Type = v
		}

		if !more {
			break
		}
	}

	if link.Href != "" {
		p.links = append(p.links, link)
		if p.onLink != nil {
			p.onLink(link)
		}
	}
}

// processLinkTag extracts <link> tag information (stylesheets, etc.).
func (p *StreamingParser) processLinkTag(tokenizer *html.Tokenizer, hasAttr bool) {
	if !hasAttr {
		return
	}

	var href, rel, media string

	for {
		key, val, more := tokenizer.TagAttr()
		k := string(key)
		v := string(val)

		switch k {
		case "href":
			href = v
		case "rel":
			rel = v
		case "media":
			media = v
		}

		if !more {
			break
		}
	}

	if rel == "stylesheet" && href != "" {
		p.stylesheets = append(p.stylesheets, Stylesheet{
			Href:  href,
			Media: media,
		})
	}
}

// processImage extracts image information.
func (p *StreamingParser) processImage(tokenizer *html.Tokenizer, hasAttr bool) {
	if !hasAttr || len(p.images) >= p.maxImages {
		return
	}

	img := Image{}

	for {
		key, val, more := tokenizer.TagAttr()
		k := string(key)
		v := string(val)

		switch k {
		case "src":
			img.Src = v
		case "alt":
			img.Alt = truncate(v, 500)
		case "width":
			img.Width = v
		case "height":
			img.Height = v
		case "loading":
			img.Lazy = v == "lazy"
		}

		if !more {
			break
		}
	}

	if img.Src != "" {
		p.images = append(p.images, img)
	}
}

// processScript extracts script information.
func (p *StreamingParser) processScript(tokenizer *html.Tokenizer, hasAttr bool) {
	if !hasAttr {
		return
	}

	script := Script{}

	for {
		key, val, more := tokenizer.TagAttr()
		k := string(key)
		v := string(val)

		switch k {
		case "src":
			script.Src = v
		case "async":
			script.Async = true
		case "defer":
			script.Defer = true
		case "type":
			script.Type = v
		}

		if !more {
			break
		}
	}

	if script.Src != "" {
		p.scripts = append(p.scripts, script)
	}
}

// Results getters

// GetTitle returns the parsed title.
func (p *StreamingParser) GetTitle() string {
	return p.title
}

// GetMetaTags returns parsed meta tags.
func (p *StreamingParser) GetMetaTags() []MetaTag {
	return p.metaTags
}

// GetLinks returns parsed links.
func (p *StreamingParser) GetLinks() []ExtractedLink {
	return p.links
}

// GetHeadings returns parsed headings.
func (p *StreamingParser) GetHeadings() []Heading {
	return p.headings
}

// GetImages returns parsed images.
func (p *StreamingParser) GetImages() []Image {
	return p.images
}

// GetScripts returns parsed scripts.
func (p *StreamingParser) GetScripts() []Script {
	return p.scripts
}

// GetStylesheets returns parsed stylesheets.
func (p *StreamingParser) GetStylesheets() []Stylesheet {
	return p.stylesheets
}

// Reset clears all parsed data.
func (p *StreamingParser) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.title = ""
	p.metaTags = p.metaTags[:0]
	p.links = p.links[:0]
	p.headings = p.headings[:0]
	p.images = p.images[:0]
	p.scripts = p.scripts[:0]
	p.stylesheets = p.stylesheets[:0]
	p.inHead = false
	p.inBody = false
	p.inScript = false
	p.inStyle = false
	p.currentDepth = 0
}

// truncate truncates string to max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// ParseStats returns parsing statistics.
type ParseStats struct {
	TitleLen     int
	MetaCount    int
	LinkCount    int
	HeadingCount int
	ImageCount   int
	ScriptCount  int
	StyleCount   int
}

// GetStats returns parsing statistics.
func (p *StreamingParser) GetStats() ParseStats {
	return ParseStats{
		TitleLen:     len(p.title),
		MetaCount:    len(p.metaTags),
		LinkCount:    len(p.links),
		HeadingCount: len(p.headings),
		ImageCount:   len(p.images),
		ScriptCount:  len(p.scripts),
		StyleCount:   len(p.stylesheets),
	}
}
