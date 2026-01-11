// Package ui provides the main user interface for Spider.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/spider-crawler/spider/internal/ui/components"
	"github.com/spider-crawler/spider/internal/ui/tabs"
	spiderTheme "github.com/spider-crawler/spider/internal/ui/theme"
)

// App represents the main Spider application.
type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window

	// UI Components
	urlEntry      *widget.Entry
	startButton   *widget.Button
	pauseButton   *widget.Button
	stopButton    *widget.Button
	progressBar   *widget.ProgressBar
	statsBar      *components.StatsBar
	mainTabs      *container.AppTabs
	detailsPanel  *components.URLDetailsPanel
	tabViews      map[tabs.TabID]*tabs.TabView

	// State
	isCrawling bool
	isPaused   bool

	// Callbacks
	OnStartCrawl func(url string)
	OnPauseCrawl func()
	OnStopCrawl  func()
}

// NewApp creates a new Spider application.
func NewApp() *App {
	a := &App{
		tabViews: make(map[tabs.TabID]*tabs.TabView),
	}

	// Create Fyne app
	a.fyneApp = app.New()
	a.fyneApp.Settings().SetTheme(&spiderTheme.SpiderTheme{})

	// Create main window
	a.mainWindow = a.fyneApp.NewWindow("Spider - SEO Crawler")
	a.mainWindow.Resize(fyne.NewSize(1400, 900))
	a.mainWindow.CenterOnScreen()

	// Build UI
	a.buildUI()

	return a
}

// buildUI constructs the user interface.
func (a *App) buildUI() {
	// Top toolbar
	toolbar := a.buildToolbar()

	// Main tabs
	a.mainTabs = a.buildMainTabs()

	// Details panel (bottom)
	a.detailsPanel = components.NewURLDetailsPanel()
	detailsContainer := container.NewStack(
		canvas.NewRectangle(spiderTheme.ColorSurface),
		a.detailsPanel,
	)

	// Status bar
	a.statsBar = components.NewStatsBar()
	statusContainer := container.NewStack(
		canvas.NewRectangle(spiderTheme.ColorSidebar),
		container.NewPadded(a.statsBar),
	)

	// Split: main content (tabs) and details panel
	split := container.NewVSplit(a.mainTabs, detailsContainer)
	split.SetOffset(0.65) // 65% top, 35% bottom

	// Main layout
	content := container.NewBorder(
		toolbar,         // top
		statusContainer, // bottom
		nil,             // left
		nil,             // right
		split,           // center
	)

	a.mainWindow.SetContent(content)
}

// buildToolbar creates the top toolbar.
func (a *App) buildToolbar() fyne.CanvasObject {
	// URL Entry
	a.urlEntry = widget.NewEntry()
	a.urlEntry.SetPlaceHolder("Enter URL to crawl (e.g., https://example.com)")

	// Buttons
	a.startButton = widget.NewButton("Start", func() {
		if a.OnStartCrawl != nil && a.urlEntry.Text != "" {
			a.OnStartCrawl(a.urlEntry.Text)
			a.setIsCrawling(true)
		}
	})
	a.startButton.Importance = widget.HighImportance

	a.pauseButton = widget.NewButton("Pause", func() {
		if a.OnPauseCrawl != nil {
			a.OnPauseCrawl()
			a.isPaused = !a.isPaused
			if a.isPaused {
				a.pauseButton.SetText("Resume")
			} else {
				a.pauseButton.SetText("Pause")
			}
		}
	})
	a.pauseButton.Disable()

	a.stopButton = widget.NewButton("Stop", func() {
		if a.OnStopCrawl != nil {
			a.OnStopCrawl()
			a.setIsCrawling(false)
		}
	})
	a.stopButton.Disable()

	// Progress bar
	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	// Layout
	buttons := container.NewHBox(
		a.startButton,
		a.pauseButton,
		a.stopButton,
	)

	urlRow := container.NewBorder(
		nil, nil, nil, buttons,
		a.urlEntry,
	)

	// Toolbar background
	toolbarBg := canvas.NewRectangle(spiderTheme.ColorSidebar)

	return container.NewStack(
		toolbarBg,
		container.NewVBox(
			container.NewPadded(urlRow),
			a.progressBar,
		),
	)
}

// buildMainTabs creates all the main tabs.
func (a *App) buildMainTabs() *container.AppTabs {
	allTabs := tabs.AllTabs()
	tabItems := make([]*container.TabItem, 0, len(allTabs))

	for _, tabConfig := range allTabs {
		tv := tabs.NewTabView(tabConfig)
		a.tabViews[tabConfig.ID] = tv

		// Set row selection callback
		tv.Table.OnRowSelected = func(rowIndex int, rowData []string) {
			// Update details panel with selected row
			a.onRowSelected(tabConfig.ID, rowIndex, rowData)
		}

		tabItems = append(tabItems, container.NewTabItem(tabConfig.Title, tv.Content))
	}

	appTabs := container.NewAppTabs(tabItems...)
	appTabs.SetTabLocation(container.TabLocationTop)

	return appTabs
}

// onRowSelected handles row selection in any tab.
func (a *App) onRowSelected(tabID tabs.TabID, rowIndex int, rowData []string) {
	// Convert row data to URLDetails
	// This is a simplified version - real implementation would fetch full details from database
	if len(rowData) > 0 {
		details := &components.URLDetails{
			URL: rowData[0],
		}

		if len(rowData) > 1 {
			// Parse other fields based on tab type
			// This is tab-specific logic
		}

		a.detailsPanel.SetDetails(details)
	}
}

// setIsCrawling updates UI state for crawling.
func (a *App) setIsCrawling(crawling bool) {
	a.isCrawling = crawling

	if crawling {
		a.startButton.Disable()
		a.pauseButton.Enable()
		a.stopButton.Enable()
		a.urlEntry.Disable()
		a.progressBar.Show()
	} else {
		a.startButton.Enable()
		a.pauseButton.Disable()
		a.stopButton.Disable()
		a.urlEntry.Enable()
		a.progressBar.Hide()
		a.pauseButton.SetText("Pause")
		a.isPaused = false
	}
}

// UpdateProgress updates the progress bar.
func (a *App) UpdateProgress(current, total int) {
	if total > 0 {
		a.progressBar.SetValue(float64(current) / float64(total))
	}
}

// UpdateStats updates the stats bar.
func (a *App) UpdateStats(urls, crawled, errors int, elapsed string) {
	a.statsBar.Update(urls, crawled, errors, elapsed)
}

// UpdateTabData updates data for a specific tab.
func (a *App) UpdateTabData(tabID tabs.TabID, rows [][]string) {
	if tv, exists := a.tabViews[tabID]; exists {
		tv.SetData(rows)
	}
}

// GetTabView returns a specific tab view.
func (a *App) GetTabView(tabID tabs.TabID) *tabs.TabView {
	return a.tabViews[tabID]
}

// ShowError shows an error dialog.
func (a *App) ShowError(title, message string) {
	dialog := widget.NewLabel(message)
	popup := widget.NewModalPopUp(
		container.NewVBox(
			widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			dialog,
			widget.NewButton("OK", func() {}),
		),
		a.mainWindow.Canvas(),
	)
	popup.Show()
}

// Run starts the application.
func (a *App) Run() {
	a.mainWindow.ShowAndRun()
}

// Window returns the main window.
func (a *App) Window() fyne.Window {
	return a.mainWindow
}

// Quit closes the application.
func (a *App) Quit() {
	a.fyneApp.Quit()
}

// buildMenuBar creates the application menu (optional).
func (a *App) buildMenuBar() *fyne.MainMenu {
	// File menu
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("New Crawl", func() {
			a.urlEntry.SetText("")
			a.urlEntry.FocusGained()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Export All...", func() {
			// TODO: Export functionality
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Exit", func() {
			a.Quit()
		}),
	)

	// Edit menu
	editMenu := fyne.NewMenu("Edit",
		fyne.NewMenuItem("Copy URL", func() {
			// TODO: Copy selected URL
		}),
	)

	// View menu
	viewMenu := fyne.NewMenu("View",
		fyne.NewMenuItem("Refresh", func() {
			// TODO: Refresh current view
		}),
	)

	// Help menu
	helpMenu := fyne.NewMenu("Help",
		fyne.NewMenuItem("About", func() {
			// TODO: About dialog
		}),
	)

	return fyne.NewMainMenu(fileMenu, editMenu, viewMenu, helpMenu)
}

// NewSplashScreen creates a splash/loading screen (optional).
func NewSplashScreen(fyneApp fyne.App) fyne.Window {
	splash := fyneApp.NewWindow("Spider")
	splash.SetFixedSize(true)
	splash.Resize(fyne.NewSize(400, 250))
	splash.CenterOnScreen()

	// Logo/Title
	title := widget.NewLabelWithStyle("🕷️ SPIDER", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("SEO Crawler")
	subtitle.Alignment = fyne.TextAlignCenter

	version := widget.NewLabel("v0.1.0")
	version.Alignment = fyne.TextAlignCenter

	loading := widget.NewProgressBarInfinite()

	content := container.NewVBox(
		layout.NewSpacer(),
		title,
		subtitle,
		layout.NewSpacer(),
		loading,
		version,
		layout.NewSpacer(),
	)

	bg := canvas.NewRectangle(spiderTheme.ColorBackground)
	splash.SetContent(container.NewStack(bg, container.NewPadded(content)))

	return splash
}
