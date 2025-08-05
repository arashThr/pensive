package validations

import (
	"html"
	"regexp"
	"strconv"

	"github.com/arashthr/go-course/internal/logging"
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
