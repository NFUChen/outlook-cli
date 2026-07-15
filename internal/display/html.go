package display

import (
	"html"
	"regexp"
	"strings"
)

var (
	brRe     = regexp.MustCompile(`<br\s*/?>`)
	pOpenRe  = regexp.MustCompile(`<p[^>]*>`)
	pCloseRe = regexp.MustCompile(`</p>`)
	tagRe    = regexp.MustCompile(`<[^>]+>`)
	htmlRe   = regexp.MustCompile(`(?i)<(html|div|p|br|table)\b`)
)

// StripHTML strips HTML tags for plain text display.
func StripHTML(text string) string {
	clean := brRe.ReplaceAllString(text, "\n")
	clean = pOpenRe.ReplaceAllString(clean, "\n")
	clean = pCloseRe.ReplaceAllString(clean, "")
	clean = tagRe.ReplaceAllString(clean, "")
	clean = strings.ReplaceAll(clean, "&nbsp;", " ")
	clean = html.UnescapeString(clean)
	return strings.TrimSpace(clean)
}

// LooksLikeHTML reports whether text appears to contain HTML markup.
func LooksLikeHTML(text string) bool {
	return htmlRe.MatchString(text)
}
