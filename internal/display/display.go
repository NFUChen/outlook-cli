// Package display renders CLI output: styled messages, tables, and panels.
package display

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

// Printer writes styled output to Out and Err.
type Printer struct {
	Out   io.Writer
	Err   io.Writer
	Color bool
}

// NewPrinter returns a Printer bound to stdout/stderr with color auto-detection.
func NewPrinter() *Printer {
	return &Printer{
		Out:   os.Stdout,
		Err:   os.Stderr,
		Color: colorEnabled(os.Stdout),
	}
}

func colorEnabled(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func (p *Printer) style(codes, s string) string {
	if !p.Color {
		return s
	}
	return codes + s + ansiReset
}

// Error prints "Error: msg" to Err.
func (p *Printer) Error(msg string) {
	fmt.Fprintf(p.Err, "%s %s\n", p.style(ansiBold+ansiRed, "Error:"), msg)
}

// Success prints "OK: msg" to Out.
func (p *Printer) Success(msg string) {
	fmt.Fprintf(p.Out, "%s %s\n", p.style(ansiBold+ansiGreen, "OK:"), msg)
}

// Warn prints "Warning: msg" to Out.
func (p *Printer) Warn(msg string) {
	fmt.Fprintf(p.Out, "%s %s\n", p.style(ansiBold+ansiYellow, "Warning:"), msg)
}

// Println prints plain text to Out.
func (p *Printer) Println(a ...any) {
	fmt.Fprintln(p.Out, a...)
}

// table prints a titled, tab-aligned table.
func (p *Printer) table(title string, headers []string, rows [][]string) {
	fmt.Fprintln(p.Out, p.style(ansiBold, title))
	w := tabwriter.NewWriter(p.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, r := range rows {
		fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	w.Flush()
}

// panel prints a titled block with a rule above and below the header lines.
func (p *Printer) panel(title string, lines []string) {
	if title == "" {
		title = "(no subject)"
	}
	width := utf8.RuneCountInString(title) + 6
	for _, l := range lines {
		if n := utf8.RuneCountInString(l) + 2; n > width {
			width = n
		}
	}
	if width < 40 {
		width = 40
	}
	if width > 100 {
		width = 100
	}
	top := "── " + p.style(ansiBold, title) + " " + strings.Repeat("─", max(width-utf8.RuneCountInString(title)-4, 2))
	fmt.Fprintln(p.Out, top)
	for _, l := range lines {
		fmt.Fprintln(p.Out, l)
	}
	fmt.Fprintln(p.Out, strings.Repeat("─", width))
}

// field formats a "Label: value" line with a bold label.
func (p *Printer) field(label, value string) string {
	return fmt.Sprintf("%s %s", p.style(ansiBold, label+":"), value)
}

// Field prints a "Label: value" line to Out.
func (p *Printer) Field(label, value string) {
	fmt.Fprintln(p.Out, p.field(label, value))
}

// truncate shortens s to at most maxRunes runes, appending "…" when truncated.
func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	r := []rune(s)
	return string(r[:maxRunes-1]) + "…"
}
