// Package components provides reusable UI components.
package components

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// URLDetails holds details about a URL.
type URLDetails struct {
	URL             string
	StatusCode      int
	ContentType     string
	Title           string
	MetaDescription string
	H1              string
	Canonical       string
	WordCount       int
	ResponseTime    string
	Inlinks         []LinkInfo
	Outlinks        []LinkInfo
	Images          []ImageInfo
	Headers         map[string]string
}

// LinkInfo holds link information.
type LinkInfo struct {
	URL        string
	AnchorText string
	Rel        string
	NoFollow   bool
}

// ImageInfo holds image information.
type ImageInfo struct {
	URL    string
	Alt    string
	Size   string
	Status int
}

// URLDetailsPanel shows detailed information about a selected URL.
type URLDetailsPanel struct {
	widget.BaseWidget

	tabs *container.AppTabs

	// Tab contents
	summaryContent  fyne.CanvasObject
	inlinksContent  fyne.CanvasObject
	outlinksContent fyne.CanvasObject
	imagesContent   fyne.CanvasObject
	headersContent  fyne.CanvasObject
	sourceContent   fyne.CanvasObject

	// Current data
	currentURL *URLDetails
}

// NewURLDetailsPanel creates a new URL details panel.
func NewURLDetailsPanel() *URLDetailsPanel {
	p := &URLDetailsPanel{}

	// Initialize empty contents
	p.summaryContent = widget.NewLabel("Select a URL to view details")
	p.inlinksContent = widget.NewLabel("No inlinks")
	p.outlinksContent = widget.NewLabel("No outlinks")
	p.imagesContent = widget.NewLabel("No images")
	p.headersContent = widget.NewLabel("No headers")
	p.sourceContent = widget.NewLabel("No source")

	p.tabs = container.NewAppTabs(
		container.NewTabItem("URL Details", p.summaryContent),
		container.NewTabItem("Inlinks", p.inlinksContent),
		container.NewTabItem("Outlinks", p.outlinksContent),
		container.NewTabItem("Images", p.imagesContent),
		container.NewTabItem("HTTP Headers", p.headersContent),
		container.NewTabItem("View Source", p.sourceContent),
	)

	p.ExtendBaseWidget(p)
	return p
}

// SetDetails updates the panel with new URL details.
func (p *URLDetailsPanel) SetDetails(details *URLDetails) {
	p.currentURL = details

	if details == nil {
		p.tabs.Items[0].Content = widget.NewLabel("Select a URL to view details")
		p.Refresh()
		return
	}

	// Update Summary tab
	p.tabs.Items[0].Content = p.createSummaryContent(details)

	// Update Inlinks tab
	p.tabs.Items[1].Content = p.createLinksContent(details.Inlinks, "Inlinks")

	// Update Outlinks tab
	p.tabs.Items[2].Content = p.createLinksContent(details.Outlinks, "Outlinks")

	// Update Images tab
	p.tabs.Items[3].Content = p.createImagesContent(details.Images)

	// Update Headers tab
	p.tabs.Items[4].Content = p.createHeadersContent(details.Headers)

	p.Refresh()
}

// createSummaryContent creates the summary tab content.
func (p *URLDetailsPanel) createSummaryContent(details *URLDetails) fyne.CanvasObject {
	items := []fyne.CanvasObject{
		p.createDetailRow("URL", details.URL),
		p.createDetailRow("Status", fmt.Sprintf("%d", details.StatusCode)),
		p.createDetailRow("Content Type", details.ContentType),
		p.createDetailRow("Response Time", details.ResponseTime),
		widget.NewSeparator(),
		p.createDetailRow("Title", details.Title),
		p.createDetailRow("Meta Description", truncate(details.MetaDescription, 200)),
		p.createDetailRow("H1", details.H1),
		p.createDetailRow("Canonical", details.Canonical),
		p.createDetailRow("Word Count", fmt.Sprintf("%d", details.WordCount)),
		widget.NewSeparator(),
		p.createDetailRow("Inlinks Count", fmt.Sprintf("%d", len(details.Inlinks))),
		p.createDetailRow("Outlinks Count", fmt.Sprintf("%d", len(details.Outlinks))),
		p.createDetailRow("Images Count", fmt.Sprintf("%d", len(details.Images))),
	}

	return container.NewVScroll(container.NewVBox(items...))
}

// createDetailRow creates a labeled detail row.
func (p *URLDetailsPanel) createDetailRow(label, value string) fyne.CanvasObject {
	labelWidget := widget.NewLabelWithStyle(label+":", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	valueWidget := widget.NewLabel(value)
	valueWidget.Wrapping = fyne.TextWrapWord

	return container.NewBorder(nil, nil, labelWidget, nil, valueWidget)
}

// createLinksContent creates the links list content.
func (p *URLDetailsPanel) createLinksContent(links []LinkInfo, title string) fyne.CanvasObject {
	if len(links) == 0 {
		return widget.NewLabel(fmt.Sprintf("No %s found", title))
	}

	list := widget.NewList(
		func() int { return len(links) },
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("URL"),
				widget.NewLabel("Anchor"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			link := links[id]
			cont := obj.(*fyne.Container)
			cont.Objects[0].(*widget.Label).SetText(link.URL)

			anchor := link.AnchorText
			if link.NoFollow {
				anchor += " [nofollow]"
			}
			cont.Objects[1].(*widget.Label).SetText(anchor)
		},
	)

	header := widget.NewLabel(fmt.Sprintf("%s (%d)", title, len(links)))
	return container.NewBorder(header, nil, nil, nil, list)
}

// createImagesContent creates the images list content.
func (p *URLDetailsPanel) createImagesContent(images []ImageInfo) fyne.CanvasObject {
	if len(images) == 0 {
		return widget.NewLabel("No images found")
	}

	list := widget.NewList(
		func() int { return len(images) },
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabel("URL"),
				widget.NewLabel("Alt"),
				widget.NewLabel("Size / Status"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			img := images[id]
			cont := obj.(*fyne.Container)
			cont.Objects[0].(*widget.Label).SetText(img.URL)
			cont.Objects[1].(*widget.Label).SetText("Alt: " + img.Alt)
			cont.Objects[2].(*widget.Label).SetText(fmt.Sprintf("Size: %s | Status: %d", img.Size, img.Status))
		},
	)

	header := widget.NewLabel(fmt.Sprintf("Images (%d)", len(images)))
	return container.NewBorder(header, nil, nil, nil, list)
}

// createHeadersContent creates the headers content.
func (p *URLDetailsPanel) createHeadersContent(headers map[string]string) fyne.CanvasObject {
	if len(headers) == 0 {
		return widget.NewLabel("No headers")
	}

	items := make([]fyne.CanvasObject, 0, len(headers))
	for k, v := range headers {
		items = append(items, p.createDetailRow(k, v))
	}

	return container.NewVScroll(container.NewVBox(items...))
}

// SetSource sets the HTML source content.
func (p *URLDetailsPanel) SetSource(source string) {
	entry := widget.NewMultiLineEntry()
	entry.SetText(source)
	entry.Disable() // Read-only
	p.tabs.Items[5].Content = entry
	p.Refresh()
}

// CreateRenderer creates the panel renderer.
func (p *URLDetailsPanel) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.tabs)
}

// Helper functions

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
