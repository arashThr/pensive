package validations

import (
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"github.com/arashthr/pensive/internal/logging"
	"github.com/microcosm-cc/bluemonday"
)

var spacesRegex *regexp.Regexp = regexp.MustCompile("[\t|\n]+")

var policy = bluemonday.UGCPolicy()

func CleanUpText(text string) string {
	// Important: unescape first, then sanitize. This prevents encoded tags like
	// "&lt;script&gt;" from bypassing the sanitizer.
	unescaped := html.UnescapeString(spacesRegex.ReplaceAllLiteralString(text, " "))
	return policy.Sanitize(unescaped)
}

func GetPageOffset(pageStr string) int {
	if pageStr == "" {
		return 1
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		logging.Logger.Errorw("converting page to int", "page", pageStr, "error", err)
		return 1
	}
	if page <= 0 || page >= 100 {
		return 1
	}
	return page
}

func GetString(v any) string {
	if v == nil {
		return ""
	}

	var raw string
	switch x := v.(type) {
	case template.HTML:
		raw = string(x)
	case string:
		raw = x
	case *string:
		if x == nil {
			return ""
		}
		raw = *x
	default:
		raw = fmt.Sprint(v)
	}
	return raw
}

// Markers wrapped around search matches by Postgres ts_headline. They are
// Unicode private-use characters, so they never occur in real page text.
const (
	HighlightStart = ""
	HighlightStop  = ""
)

// HighlightedHTML escapes plain text and turns match markers into <strong>
// tags, so untrusted text (like page titles) can be shown with highlights.
func HighlightedHTML(text string) template.HTML {
	escaped := html.EscapeString(text)
	escaped = strings.ReplaceAll(escaped, HighlightStart, "<strong>")
	escaped = strings.ReplaceAll(escaped, HighlightStop, "</strong>")
	return template.HTML(escaped)
}
