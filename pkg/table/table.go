package table

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
)

// Table represents a table with headers, rows and configuration
type Table struct {
	headers   []string
	rows      [][]string
	writer    io.Writer
	config    Config
	tabwriter *tabwriter.Writer
}

// Config holds table configuration options
type Config struct {
	// Title shown above the table
	Title string
	// ShowHeaders whether to display column headers
	ShowHeaders bool
	// BoldHeaders whether to make headers bold
	BoldHeaders bool
	// SeparatorChar character used for separator lines (default: "=")
	SeparatorChar string
	// MaxColumnWidth maximum width for truncating columns (0 = no limit)
	MaxColumnWidth int
	// MinColumnWidth minimum width for each column
	MinColumnWidth int
	// UseTabwriter whether to use tabwriter for alignment (recommended)
	UseTabwriter bool
	// TabwriterConfig for fine-tuning tabwriter behavior
	TabwriterConfig TabwriterConfig
}

// TabwriterConfig holds tabwriter-specific configuration
type TabwriterConfig struct {
	MinWidth int  // minimum cell width
	TabWidth int  // tab width
	Padding  int  // padding added to a cell before computing its width
	PadChar  byte // padding character (usually ' ')
	Flags    uint // formatting flags
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() Config {
	return Config{
		ShowHeaders:    true,
		BoldHeaders:    false,
		SeparatorChar:  "─",
		MaxColumnWidth: 50,
		MinColumnWidth: 0,
		UseTabwriter:   true,
		TabwriterConfig: TabwriterConfig{
			MinWidth: 0,
			TabWidth: 8,
			Padding:  1,
			PadChar:  ' ',
			Flags:    0,
		},
	}
}

// New creates a new table with the given headers and default config
func New(headers ...string) *Table {
	return NewWithConfig(DefaultConfig(), headers...)
}

// NewWithConfig creates a new table with custom configuration
func NewWithConfig(config Config, headers ...string) *Table {
	t := &Table{
		headers: headers,
		rows:    make([][]string, 0),
		writer:  os.Stdout,
		config:  config,
	}

	if config.UseTabwriter {
		t.tabwriter = tabwriter.NewWriter(
			t.writer,
			config.TabwriterConfig.MinWidth,
			config.TabwriterConfig.TabWidth,
			config.TabwriterConfig.Padding,
			config.TabwriterConfig.PadChar,
			config.TabwriterConfig.Flags,
		)
	}

	return t
}

// AddRow adds a row to the table
func (t *Table) AddRow(columns ...string) *Table {
	// Ensure row has same number of columns as headers
	row := make([]string, len(t.headers))
	for i, col := range columns {
		if i < len(row) {
			row[i] = col
		}
	}
	t.rows = append(t.rows, row)
	return t
}

// truncateString truncates a string to maxLength characters, adding "..." if truncated
func truncateString(s string, maxLength int) string {
	if maxLength <= 0 || len(s) <= maxLength {
		return s
	}
	if maxLength <= 3 {
		return s[:maxLength]
	}
	return s[:maxLength-3] + "..."
}

// Render prints the table to the configured writer
func (t *Table) Render() error {
	// Show title if configured
	if t.config.Title != "" {
		fmt.Fprintf(t.writer, "%s\n", t.config.Title)
	}

	if t.config.UseTabwriter {
		return t.renderWithTabwriter()
	}
	return t.renderWithFixedWidth()
}

// renderWithTabwriter renders using text/tabwriter for alignment
func (t *Table) renderWithTabwriter() error {
	w := t.tabwriter

	// Calculate separator length for title section
	sepLength := 50
	if t.config.Title != "" {
		if len(t.config.Title) > sepLength {
			sepLength = len(t.config.Title)
		}
		fmt.Fprintf(t.writer, "%s\n", strings.Repeat(t.config.SeparatorChar, sepLength))
	}

	// Render headers
	if t.config.ShowHeaders && len(t.headers) > 0 {
		headerRow := make([]string, len(t.headers))
		for i, header := range t.headers {
			headerText := truncateString(header, t.config.MaxColumnWidth)
			if t.config.BoldHeaders {
				bold := color.New(color.Bold).SprintFunc()
				headerText = bold(headerText)
			}
			headerRow[i] = headerText
		}
		fmt.Fprintf(w, "%s\n", strings.Join(headerRow, "\t"))
	}

	// Render rows
	for _, row := range t.rows {
		renderRow := make([]string, len(row))
		for i, cell := range row {
			renderRow[i] = truncateString(cell, t.config.MaxColumnWidth)
		}
		fmt.Fprintf(w, "%s\n", strings.Join(renderRow, "\t"))
	}

	return w.Flush()
}

// renderWithFixedWidth renders using fixed-width printf formatting
func (t *Table) renderWithFixedWidth() error {
	if len(t.headers) == 0 {
		return fmt.Errorf("no headers defined")
	}

	// Calculate column widths
	colWidths := make([]int, len(t.headers))

	// Start with header widths (use visible text length, not formatted length)
	for i, header := range t.headers {
		headerText := truncateString(header, t.config.MaxColumnWidth)
		colWidths[i] = len(headerText) // Use visible length, not bold-formatted length
		if t.config.MinColumnWidth > colWidths[i] {
			colWidths[i] = t.config.MinColumnWidth
		}
	}

	// Update widths based on row data
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(colWidths) {
				cellLen := len(truncateString(cell, t.config.MaxColumnWidth))
				if cellLen > colWidths[i] {
					colWidths[i] = cellLen
				}
			}
		}
	}

	// Apply max column width constraint
	for i := range colWidths {
		if t.config.MaxColumnWidth > 0 && colWidths[i] > t.config.MaxColumnWidth {
			colWidths[i] = t.config.MaxColumnWidth
		}
	}

	// Calculate total width for separator
	totalWidth := 0
	for _, width := range colWidths {
		totalWidth += width + 1 // +1 for space
	}
	if totalWidth > 0 {
		totalWidth-- // Remove last space
	}

	// Title separator
	if t.config.Title != "" {
		fmt.Fprintf(t.writer, "%s\n", strings.Repeat(t.config.SeparatorChar, totalWidth))
	}

	// Render headers
	if t.config.ShowHeaders {
		formatStr := ""
		for i, width := range colWidths {
			if i > 0 {
				formatStr += " "
			}
			formatStr += fmt.Sprintf("%%-%ds", width)
		}

		headerVals := make([]interface{}, len(t.headers))
		for i, header := range t.headers {
			headerText := truncateString(header, colWidths[i])
			if t.config.BoldHeaders {
				bold := color.New(color.Bold).SprintFunc()
				// For fixed-width formatting, we need to pad the visible text, then apply bold
				paddedText := fmt.Sprintf("%-*s", colWidths[i], headerText)
				headerVals[i] = bold(paddedText)
			} else {
				headerVals[i] = headerText
			}
		}

		if t.config.BoldHeaders {
			// With bold headers, we need special formatting since padding is already included
			for i, val := range headerVals {
				if i > 0 {
					fmt.Fprintf(t.writer, " ")
				}
				fmt.Fprintf(t.writer, "%s", val)
			}
			fmt.Fprintf(t.writer, "\n")
		} else {
			fmt.Fprintf(t.writer, formatStr+"\n", headerVals...)
		}
	}

	// Render rows
	formatStr := ""
	for i, width := range colWidths {
		if i > 0 {
			formatStr += " "
		}
		formatStr += fmt.Sprintf("%%-%ds", width)
	}

	for _, row := range t.rows {
		rowVals := make([]interface{}, len(colWidths))
		for i := range colWidths {
			if i < len(row) {
				rowVals[i] = truncateString(row[i], colWidths[i])
			} else {
				rowVals[i] = ""
			}
		}
		fmt.Fprintf(t.writer, formatStr+"\n", rowVals...)
	}

	return nil
}
