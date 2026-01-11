// Package components provides reusable UI components.
package components

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	spiderTheme "github.com/spider-crawler/spider/internal/ui/theme"
)

// Column defines a table column.
type Column struct {
	ID       string
	Title    string
	Width    float32
	Sortable bool
	Visible  bool
}

// TableData represents data for the table.
type TableData struct {
	Columns []Column
	Rows    [][]string
}

// DataTable is a custom table widget with sorting and selection.
type DataTable struct {
	widget.BaseWidget

	data          *TableData
	sortColumn    int
	sortAscending bool
	selectedRow   int
	filteredRows  []int // Indices of rows that match filter

	// Callbacks
	OnRowSelected func(rowIndex int, rowData []string)
	OnRowDouble   func(rowIndex int, rowData []string)

	// Internal
	header    *fyne.Container
	body      *widget.List
	filter    string
}

// NewDataTable creates a new data table.
func NewDataTable(data *TableData) *DataTable {
	dt := &DataTable{
		data:          data,
		sortColumn:    -1,
		sortAscending: true,
		selectedRow:   -1,
		filteredRows:  make([]int, 0),
	}

	// Initialize filtered rows to all rows
	for i := range data.Rows {
		dt.filteredRows = append(dt.filteredRows, i)
	}

	dt.ExtendBaseWidget(dt)
	return dt
}

// SetData updates the table data.
func (dt *DataTable) SetData(data *TableData) {
	dt.data = data
	dt.filteredRows = make([]int, 0)
	for i := range data.Rows {
		dt.filteredRows = append(dt.filteredRows, i)
	}
	dt.Refresh()
}

// SetFilter applies a text filter to the table.
func (dt *DataTable) SetFilter(filter string) {
	dt.filter = strings.ToLower(filter)
	dt.applyFilter()
	dt.Refresh()
}

// applyFilter filters rows based on current filter text.
func (dt *DataTable) applyFilter() {
	dt.filteredRows = make([]int, 0)

	if dt.filter == "" {
		for i := range dt.data.Rows {
			dt.filteredRows = append(dt.filteredRows, i)
		}
		return
	}

	for i, row := range dt.data.Rows {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), dt.filter) {
				dt.filteredRows = append(dt.filteredRows, i)
				break
			}
		}
	}
}

// Sort sorts the table by the specified column.
func (dt *DataTable) Sort(columnIndex int) {
	if columnIndex < 0 || columnIndex >= len(dt.data.Columns) {
		return
	}

	if dt.sortColumn == columnIndex {
		dt.sortAscending = !dt.sortAscending
	} else {
		dt.sortColumn = columnIndex
		dt.sortAscending = true
	}

	sort.SliceStable(dt.filteredRows, func(i, j int) bool {
		a := dt.data.Rows[dt.filteredRows[i]][columnIndex]
		b := dt.data.Rows[dt.filteredRows[j]][columnIndex]

		if dt.sortAscending {
			return a < b
		}
		return a > b
	})

	dt.Refresh()
}

// GetSelectedRow returns the currently selected row data.
func (dt *DataTable) GetSelectedRow() []string {
	if dt.selectedRow < 0 || dt.selectedRow >= len(dt.filteredRows) {
		return nil
	}
	return dt.data.Rows[dt.filteredRows[dt.selectedRow]]
}

// RowCount returns the number of visible rows.
func (dt *DataTable) RowCount() int {
	return len(dt.filteredRows)
}

// CreateRenderer creates the table renderer.
func (dt *DataTable) CreateRenderer() fyne.WidgetRenderer {
	// Create header row
	headerCells := make([]fyne.CanvasObject, len(dt.data.Columns))
	for i, col := range dt.data.Columns {
		idx := i // Capture for closure
		btn := widget.NewButton(col.Title, func() {
			if dt.data.Columns[idx].Sortable {
				dt.Sort(idx)
			}
		})
		btn.Importance = widget.LowImportance
		headerCells[i] = btn
	}
	header := container.NewGridWithColumns(len(dt.data.Columns), headerCells...)

	// Create list for body
	list := widget.NewList(
		func() int {
			return len(dt.filteredRows)
		},
		func() fyne.CanvasObject {
			cells := make([]fyne.CanvasObject, len(dt.data.Columns))
			for i := range cells {
				cells[i] = widget.NewLabel("")
			}
			return container.NewGridWithColumns(len(dt.data.Columns), cells...)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(dt.filteredRows) {
				return
			}
			rowIdx := dt.filteredRows[id]
			row := dt.data.Rows[rowIdx]
			cont := obj.(*fyne.Container)
			for i, cell := range cont.Objects {
				if i < len(row) {
					cell.(*widget.Label).SetText(row[i])
				}
			}
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		dt.selectedRow = int(id)
		if dt.OnRowSelected != nil && id < len(dt.filteredRows) {
			dt.OnRowSelected(dt.filteredRows[id], dt.data.Rows[dt.filteredRows[id]])
		}
	}

	dt.body = list

	// Header background
	headerBg := canvas.NewRectangle(spiderTheme.ColorTableHeader)

	content := container.NewBorder(
		container.NewStack(headerBg, header), // top
		nil, // bottom
		nil, // left
		nil, // right
		list, // center
	)

	return widget.NewSimpleRenderer(content)
}

// StatusBadge creates a colored status badge.
func StatusBadge(statusCode int) fyne.CanvasObject {
	text := fmt.Sprintf("%d", statusCode)
	label := widget.NewLabel(text)

	var bgColor = spiderTheme.ColorSurface
	switch {
	case statusCode >= 200 && statusCode < 300:
		bgColor = spiderTheme.ColorSuccess
	case statusCode >= 300 && statusCode < 400:
		bgColor = spiderTheme.ColorWarning
	case statusCode >= 400:
		bgColor = spiderTheme.ColorError
	}

	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 4

	return container.NewStack(bg, container.NewCenter(label))
}

// FilterBar creates a filter/search bar.
type FilterBar struct {
	widget.BaseWidget

	searchEntry  *widget.Entry
	filterSelect *widget.Select
	exportBtn    *widget.Button

	OnSearch func(text string)
	OnFilter func(filter string)
	OnExport func()
}

// NewFilterBar creates a new filter bar.
func NewFilterBar(filters []string) *FilterBar {
	fb := &FilterBar{}

	fb.searchEntry = widget.NewEntry()
	fb.searchEntry.SetPlaceHolder("Search URLs, titles, meta...")
	fb.searchEntry.OnChanged = func(s string) {
		if fb.OnSearch != nil {
			fb.OnSearch(s)
		}
	}

	fb.filterSelect = widget.NewSelect(filters, func(s string) {
		if fb.OnFilter != nil {
			fb.OnFilter(s)
		}
	})
	fb.filterSelect.PlaceHolder = "Filter"

	fb.exportBtn = widget.NewButton("Export", func() {
		if fb.OnExport != nil {
			fb.OnExport()
		}
	})

	fb.ExtendBaseWidget(fb)
	return fb
}

// CreateRenderer creates the filter bar renderer.
func (fb *FilterBar) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewBorder(
		nil, nil, nil,
		container.NewHBox(fb.filterSelect, fb.exportBtn),
		fb.searchEntry,
	)
	return widget.NewSimpleRenderer(content)
}

// StatsBar shows crawl statistics.
type StatsBar struct {
	widget.BaseWidget

	urlsLabel     *widget.Label
	crawledLabel  *widget.Label
	errorsLabel   *widget.Label
	timeLabel     *widget.Label
}

// NewStatsBar creates a new stats bar.
func NewStatsBar() *StatsBar {
	sb := &StatsBar{
		urlsLabel:    widget.NewLabel("URLs: 0"),
		crawledLabel: widget.NewLabel("Crawled: 0"),
		errorsLabel:  widget.NewLabel("Errors: 0"),
		timeLabel:    widget.NewLabel("Time: 0s"),
	}
	sb.ExtendBaseWidget(sb)
	return sb
}

// Update updates the stats display.
func (sb *StatsBar) Update(urls, crawled, errors int, elapsed string) {
	sb.urlsLabel.SetText(fmt.Sprintf("URLs: %d", urls))
	sb.crawledLabel.SetText(fmt.Sprintf("Crawled: %d", crawled))
	sb.errorsLabel.SetText(fmt.Sprintf("Errors: %d", errors))
	sb.timeLabel.SetText(fmt.Sprintf("Time: %s", elapsed))
}

// CreateRenderer creates the stats bar renderer.
func (sb *StatsBar) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewHBox(
		sb.urlsLabel,
		widget.NewSeparator(),
		sb.crawledLabel,
		widget.NewSeparator(),
		sb.errorsLabel,
		widget.NewSeparator(),
		sb.timeLabel,
	)
	return widget.NewSimpleRenderer(content)
}
