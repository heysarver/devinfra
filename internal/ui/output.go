package ui

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
)

var (
	cyan   = color.New(color.FgCyan)
	green  = color.New(color.FgGreen)
	yellow = color.New(color.FgYellow)
	red    = color.New(color.FgRed)
)

// Info prints an informational message to stderr.
func Info(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", cyan.Sprint("[INFO]"), fmt.Sprintf(msg, args...))
}

// Ok prints a success message to stderr.
func Ok(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", green.Sprint("[OK]"), fmt.Sprintf(msg, args...))
}

// Warn prints a warning message to stderr.
func Warn(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", yellow.Sprint("[WARN]"), fmt.Sprintf(msg, args...))
}

// Fail prints an error message to stderr.
func Fail(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", red.Sprint("[FAIL]"), fmt.Sprintf(msg, args...))
}

// PrintJSON writes a JSON-encoded value to stdout.
func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintTable prints a formatted table to stdout.
func PrintTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, h := range headers {
		fmt.Fprintf(os.Stdout, "%-*s  ", widths[i], h)
	}
	fmt.Fprintln(os.Stdout)

	// Print separator
	for i := range headers {
		for j := 0; j < widths[i]; j++ {
			fmt.Fprint(os.Stdout, "-")
		}
		fmt.Fprint(os.Stdout, "  ")
	}
	fmt.Fprintln(os.Stdout)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Fprintf(os.Stdout, "%-*s  ", widths[i], cell)
			}
		}
		fmt.Fprintln(os.Stdout)
	}
}
