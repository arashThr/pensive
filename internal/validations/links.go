package validations

import "net/url"

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

// ExtractHostname extracts the hostname from a URL
func ExtractHostname(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link // fallback to original link if parsing fails
	}
	return u.Host
}
