package validations

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

var spacesRegex *regexp.Regexp = regexp.MustCompile("[\t|\n]+")

var sanitization = bluemonday.UGCPolicy()

func CleanUpText(text string) string {
	return sanitization.Sanitize(
		spacesRegex.ReplaceAllLiteralString(text, " "),
	)
}
