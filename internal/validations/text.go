package validations

import (
	"html"
	"regexp"

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
