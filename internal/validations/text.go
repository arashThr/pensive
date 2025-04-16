package validations

import (
	"html"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/microcosm-cc/bluemonday"
)

var spacesRegex *regexp.Regexp = regexp.MustCompile("[\t|\n]+")

var sanitization = bluemonday.UGCPolicy()

func CleanUpText(text string) string {
	return html.UnescapeString(
		sanitization.Sanitize(
			spacesRegex.ReplaceAllLiteralString(text, " "),
		))
}

func GetPageOffset(pageStr string) int {
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		slog.Error("converting page to int", "page", pageStr, "error", err)
		return 1
	}
	if page <= 0 || page >= 100 {
		return 1
	}
	return page
}
