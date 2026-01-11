package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExportFormat defines the export file format.
type ExportFormat string

const (
	FormatCSV  ExportFormat = "csv"
	FormatXLSX ExportFormat = "xlsx"
	FormatJSON ExportFormat = "json"
)

// ExportOptions defines export configuration.
type ExportOptions struct {
	Format       ExportFormat
	FilePath     string
	IncludeEmpty bool // Include rows with empty values
	MaxRows      int  // 0 = unlimited
	Delimiter    rune // For CSV, default is comma
}

// DefaultExportOptions returns default export options.
func DefaultExportOptions() *ExportOptions {
	return &ExportOptions{
		Format:       FormatCSV,
		IncludeEmpty: true,
		MaxRows:      0,
		Delimiter:    ',',
	}
}

// Exporter handles exporting reports to various formats.
type Exporter struct {
	options *ExportOptions
}

// NewExporter creates a new exporter.
func NewExporter(options *ExportOptions) *Exporter {
	if options == nil {
		options = DefaultExportOptions()
	}
	return &Exporter{options: options}
}

// Export exports a report to the specified format.
func (e *Exporter) Export(report *Report) error {
	switch e.options.Format {
	case FormatCSV:
		return e.exportCSV(report)
	case FormatXLSX:
		return e.exportXLSX(report)
	case FormatJSON:
		return e.exportJSON(report)
	default:
		return fmt.Errorf("unsupported export format: %s", e.options.Format)
	}
}

// exportCSV exports report to CSV format.
func (e *Exporter) exportCSV(report *Report) error {
	file, err := os.Create(e.options.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write UTF-8 BOM for Excel compatibility
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(file)
	if e.options.Delimiter != 0 {
		writer.Comma = e.options.Delimiter
	}
	defer writer.Flush()

	// Write header
	if err := writer.Write(report.Definition.Columns); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write rows
	rowCount := 0
	for _, row := range report.Rows {
		if e.options.MaxRows > 0 && rowCount >= e.options.MaxRows {
			break
		}

		values := make([]string, len(report.Definition.Columns))
		isEmpty := true

		for i, col := range report.Definition.Columns {
			if val, ok := row.Values[col]; ok {
				values[i] = formatValue(val)
				if values[i] != "" {
					isEmpty = false
				}
			}
		}

		if !e.options.IncludeEmpty && isEmpty {
			continue
		}

		if err := writer.Write(values); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
		rowCount++
	}

	return nil
}

// exportXLSX exports report to Excel format.
func (e *Exporter) exportXLSX(report *Report) error {
	f := excelize.NewFile()
	defer f.Close()

	// Create sheet with report name
	sheetName := sanitizeSheetName(report.Definition.Name)
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(index)

	// Delete default sheet
	f.DeleteSheet("Sheet1")

	// Style for header
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"00C853"}},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})

	// Style for alternating rows
	evenRowStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F5F5F5"}},
	})

	// Write header
	for i, col := range report.Definition.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, col)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Set column widths
	for i, col := range report.Definition.Columns {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		width := float64(len(col) + 5)
		if width < 15 {
			width = 15
		}
		if width > 50 {
			width = 50
		}
		f.SetColWidth(sheetName, colName, colName, width)
	}

	// Write rows
	rowCount := 0
	for rowIdx, row := range report.Rows {
		if e.options.MaxRows > 0 && rowCount >= e.options.MaxRows {
			break
		}

		isEmpty := true
		for i, col := range report.Definition.Columns {
			cell, _ := excelize.CoordinatesToCellName(i+1, rowIdx+2)

			if val, ok := row.Values[col]; ok {
				f.SetCellValue(sheetName, cell, val)
				if formatValue(val) != "" {
					isEmpty = false
				}
			}

			// Apply alternating row style
			if rowIdx%2 == 1 {
				f.SetCellStyle(sheetName, cell, cell, evenRowStyle)
			}
		}

		if !e.options.IncludeEmpty && isEmpty {
			continue
		}
		rowCount++
	}

	// Add filters
	lastCol, _ := excelize.ColumnNumberToName(len(report.Definition.Columns))
	lastRow := len(report.Rows) + 1
	filterRange := fmt.Sprintf("%s!A1:%s%d", sheetName, lastCol, lastRow)
	f.AutoFilter(sheetName, filterRange, nil)

	// Freeze header row
	f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// Add metadata sheet
	e.addMetadataSheet(f, report)

	return f.SaveAs(e.options.FilePath)
}

// addMetadataSheet adds a metadata sheet to the Excel file.
func (e *Exporter) addMetadataSheet(f *excelize.File, report *Report) {
	sheetName := "Metadata"
	f.NewSheet(sheetName)

	metadata := [][]string{
		{"Report Name", report.Definition.Name},
		{"Description", report.Definition.Description},
		{"Category", report.Definition.Category},
		{"Total Rows", fmt.Sprintf("%d", report.TotalCount)},
		{"Generated", time.Now().Format(time.RFC3339)},
		{"Tool", "Spider SEO Crawler"},
	}

	for i, row := range metadata {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", i+1), row[0])
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", i+1), row[1])
	}

	f.SetColWidth(sheetName, "A", "A", 20)
	f.SetColWidth(sheetName, "B", "B", 50)
}

// exportJSON exports report to JSON format.
func (e *Exporter) exportJSON(report *Report) error {
	data := &JSONReport{
		Metadata: JSONMetadata{
			ReportType:  string(report.Definition.Type),
			Name:        report.Definition.Name,
			Description: report.Definition.Description,
			Category:    report.Definition.Category,
			TotalCount:  report.TotalCount,
			Generated:   time.Now().Format(time.RFC3339),
			Columns:     report.Definition.Columns,
		},
		Rows: make([]map[string]interface{}, 0, len(report.Rows)),
	}

	rowCount := 0
	for _, row := range report.Rows {
		if e.options.MaxRows > 0 && rowCount >= e.options.MaxRows {
			break
		}

		isEmpty := true
		for _, v := range row.Values {
			if formatValue(v) != "" {
				isEmpty = false
				break
			}
		}

		if !e.options.IncludeEmpty && isEmpty {
			continue
		}

		data.Rows = append(data.Rows, row.Values)
		rowCount++
	}

	file, err := os.Create(e.options.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	return encoder.Encode(data)
}

// JSONReport represents the JSON export structure.
type JSONReport struct {
	Metadata JSONMetadata             `json:"metadata"`
	Rows     []map[string]interface{} `json:"rows"`
}

// JSONMetadata represents report metadata.
type JSONMetadata struct {
	ReportType  string   `json:"report_type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	TotalCount  int      `json:"total_count"`
	Generated   string   `json:"generated"`
	Columns     []string `json:"columns"`
}

// formatValue converts a value to string for export.
func formatValue(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%.2f", val)
	case bool:
		if val {
			return "Yes"
		}
		return "No"
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// sanitizeSheetName ensures sheet name is valid for Excel.
func sanitizeSheetName(name string) string {
	// Excel sheet names have restrictions
	invalid := []string{"\\", "/", "?", "*", "[", "]", ":"}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}

	// Max 31 characters
	if len(result) > 31 {
		result = result[:31]
	}

	return result
}

// BulkExporter handles exporting multiple reports.
type BulkExporter struct {
	generator *Generator
	outputDir string
}

// NewBulkExporter creates a new bulk exporter.
func NewBulkExporter(generator *Generator, outputDir string) *BulkExporter {
	return &BulkExporter{
		generator: generator,
		outputDir: outputDir,
	}
}

// ExportAll exports all reports in the specified format.
func (b *BulkExporter) ExportAll(format ExportFormat) error {
	// Create output directory
	if err := os.MkdirAll(b.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")

	for _, def := range AllReports() {
		report, err := b.generator.Generate(def.Type)
		if err != nil {
			continue // Skip failed reports
		}

		if report.TotalCount == 0 {
			continue // Skip empty reports
		}

		ext := string(format)
		filename := fmt.Sprintf("%s_%s.%s", sanitizeFilename(def.Name), timestamp, ext)
		filePath := filepath.Join(b.outputDir, filename)

		options := &ExportOptions{
			Format:   format,
			FilePath: filePath,
		}

		exporter := NewExporter(options)
		if err := exporter.Export(report); err != nil {
			continue // Skip failed exports
		}
	}

	return nil
}

// ExportAllToXLSX exports all reports to a single Excel file with multiple sheets.
func (b *BulkExporter) ExportAllToXLSX(filePath string) error {
	f := excelize.NewFile()
	defer f.Close()

	// Delete default sheet
	f.DeleteSheet("Sheet1")

	// Create summary sheet first
	b.addSummarySheet(f)

	for _, def := range AllReports() {
		report, err := b.generator.Generate(def.Type)
		if err != nil || report.TotalCount == 0 {
			continue
		}

		sheetName := sanitizeSheetName(def.Name)
		f.NewSheet(sheetName)

		// Write header
		for i, col := range report.Definition.Columns {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(sheetName, cell, col)
		}

		// Write rows
		for rowIdx, row := range report.Rows {
			for i, col := range report.Definition.Columns {
				cell, _ := excelize.CoordinatesToCellName(i+1, rowIdx+2)
				if val, ok := row.Values[col]; ok {
					f.SetCellValue(sheetName, cell, val)
				}
			}
		}
	}

	return f.SaveAs(filePath)
}

// addSummarySheet adds a summary sheet with links to all reports.
func (b *BulkExporter) addSummarySheet(f *excelize.File) {
	sheetName := "Summary"
	f.NewSheet(sheetName)
	f.SetActiveSheet(0)

	headers := []string{"Report", "Category", "Description", "Rows"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}

	row := 2
	for _, def := range AllReports() {
		report, _ := b.generator.Generate(def.Type)
		count := 0
		if report != nil {
			count = report.TotalCount
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), def.Name)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), def.Category)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), def.Description)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), count)

		// Add hyperlink to sheet
		sheetLinkName := sanitizeSheetName(def.Name)
		f.SetCellHyperLink(sheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("'%s'!A1", sheetLinkName), "Location")

		row++
	}

	f.SetColWidth(sheetName, "A", "A", 30)
	f.SetColWidth(sheetName, "B", "B", 15)
	f.SetColWidth(sheetName, "C", "C", 50)
	f.SetColWidth(sheetName, "D", "D", 10)
}

// sanitizeFilename ensures filename is valid.
func sanitizeFilename(name string) string {
	invalid := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	return strings.ToLower(result)
}

// ExportCurrentView exports only the current filtered view.
type ViewExporter struct {
	*Exporter
}

// NewViewExporter creates an exporter for current filtered view.
func NewViewExporter(options *ExportOptions) *ViewExporter {
	return &ViewExporter{
		Exporter: NewExporter(options),
	}
}

// ExportFiltered exports only rows matching the filter.
func (v *ViewExporter) ExportFiltered(report *Report, filterColumn string, filterValue interface{}) error {
	filteredReport := report.FilterReport(filterColumn, filterValue)
	return v.Export(filteredReport)
}
