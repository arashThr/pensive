package validations

import (
	"net/url"
	"sort"
	"strings"
)

func IsURLValid(link string) bool {
	if link == "" || len(link) > 2048 {
		return false
	}
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	return u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

func CanonicalURL(u *url.URL) string {
	// It's 2025! We don't expect the URL to be http
	if u.Scheme != "https" {
		u.Scheme = "https"
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimSuffix(u.Path, "/")

	if u.RawQuery != "" {
		q := u.Query()
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		sortedQuery := url.Values{}
		for _, k := range keys {
			sortedQuery[k] = q[k]
		}
		u.RawQuery = sortedQuery.Encode()
	}

	return u.String()
}

// ExtractHostname extracts the hostname from a URL
func ExtractHostname(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link // fallback to original link if parsing fails
	}
	return u.Host
}
