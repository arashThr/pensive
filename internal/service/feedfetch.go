package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/validations"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	feedRefreshInterval = time.Hour
	feedMaxBodyBytes    = 5 << 20
	feedMaxEntries      = 200
	feedMaxContentRunes = 20000
	feedMaxHTMLBytes    = 300 << 10
	feedUserAgent       = "Pensive/1.0 (+https://getpensive.com) feed fetcher"
)

// newFeedHTTPClient returns a client that refuses to connect to loopback,
// private and link-local addresses, since feed URLs come from users.
func newFeedHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: denyInternalAddresses}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			MaxIdleConnsPerHost:   2,
		},
	}
}

func denyInternalAddresses(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return fmt.Errorf("refusing to fetch internal address %s", host)
	}
	return nil
}

// fetchFeed downloads and parses a feed, using the stored validators so an
// unchanged feed costs a 304.
func (f *Feeds) fetchFeed(ctx context.Context, feed models.Feed) (models.FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.URL, nil)
	if err != nil {
		return models.FetchResult{}, err
	}
	req.Header.Set("User-Agent", feedUserAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/feed+json, application/xml;q=0.9, */*;q=0.8")
	if feed.ETag != "" {
		req.Header.Set("If-None-Match", feed.ETag)
	}
	if feed.LastModified != "" {
		req.Header.Set("If-Modified-Since", feed.LastModified)
	}

	resp, err := f.client().Do(req)
	if err != nil {
		return models.FetchResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return models.FetchResult{NotModified: true}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return models.FetchResult{}, fmt.Errorf("server responded %s", resp.Status)
	}

	parsed, err := gofeed.NewParser().Parse(io.LimitReader(resp.Body, feedMaxBodyBytes))
	if err != nil {
		return models.FetchResult{}, fmt.Errorf("not a valid feed: %w", err)
	}
	return models.FetchResult{
		Title:        strings.TrimSpace(parsed.Title),
		SiteURL:      parsed.Link,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		Entries:      entriesFromFeed(parsed, resp.Request.URL),
	}, nil
}

// discoverFeed resolves what a user pasted into a feed URL: either the URL is
// a feed itself, or it's a page that links to one.
func (f *Feeds) discoverFeed(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", feedUserAgent)
	resp, err := f.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server responded %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, feedMaxBodyBytes))
	if err != nil {
		return "", err
	}
	if _, err := gofeed.NewParser().Parse(bytes.NewReader(body)); err == nil {
		return resp.Request.URL.String(), nil
	}
	if link := feedLinkInHTML(body, resp.Request.URL); link != "" {
		return link, nil
	}
	return "", errors.New("no feed found at that address")
}

// feedLinkInHTML finds the first <link rel="alternate"> pointing at a feed.
func feedLinkInHTML(body []byte, base *url.URL) string {
	tokens := xhtml.NewTokenizer(bytes.NewReader(body))
	for {
		switch tokens.Next() {
		case xhtml.ErrorToken:
			return ""
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			tag, hasAttr := tokens.TagName()
			if string(tag) == "body" {
				return ""
			}
			if string(tag) != "link" || !hasAttr {
				continue
			}
			attrs := map[string]string{}
			for {
				key, val, more := tokens.TagAttr()
				attrs[strings.ToLower(string(key))] = string(val)
				if !more {
					break
				}
			}
			kind := strings.ToLower(attrs["type"])
			if strings.Contains(strings.ToLower(attrs["rel"]), "alternate") && attrs["href"] != "" &&
				(strings.Contains(kind, "rss") || strings.Contains(kind, "atom") || strings.Contains(kind, "feed+json")) {
				return resolveURL(base, attrs["href"])
			}
		}
	}
}

func entriesFromFeed(feed *gofeed.Feed, base *url.URL) []models.FeedEntry {
	now := time.Now()
	var entries []models.FeedEntry
	for _, item := range feed.Items {
		if len(entries) == feedMaxEntries {
			break
		}
		link := resolveURL(base, strings.TrimSpace(item.Link))
		if link == "" {
			continue
		}
		guid := strings.TrimSpace(item.GUID)
		if guid == "" {
			guid = link
		}
		content := item.Content
		if content == "" {
			content = item.Description
		}
		text := htmlToText(content)
		title := htmlToText(item.Title)
		if title == "" {
			title = truncateRunes(text, 120)
		}
		published := now
		if item.PublishedParsed != nil {
			published = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			published = *item.UpdatedParsed
		}
		if published.After(now) {
			published = now
		}
		author := ""
		if item.Author != nil {
			author = item.Author.Name
		}
		parsedLink, _ := url.Parse(link)
		canonical := *parsedLink // CanonicalURL modifies the URL it's given
		entries = append(entries, models.FeedEntry{
			GUID:         guid,
			URL:          link,
			CanonicalURL: validations.CanonicalURL(&canonical),
			ContentHTML:  feedHTML(content, parsedLink),
			Title:        title,
			Author:       strings.TrimSpace(author),
			Content:      truncateRunes(text, feedMaxContentRunes),
			PublishedAt:  published,
		})
	}
	return entries
}

var stripTags = bluemonday.StrictPolicy()

// readablePolicy keeps a post's formatting, links and images, and drops
// scripts, styles, iframes and event handlers.
var readablePolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

// feedHTML prepares a post's HTML for the reading view: relative links and
// images are resolved against the post URL, then the markup is sanitized.
// Oversized posts are skipped; the reading view falls back to plain text.
func feedHTML(raw string, base *url.URL) string {
	if strings.TrimSpace(raw) == "" || len(raw) > feedMaxHTMLBytes {
		return ""
	}
	return readablePolicy.Sanitize(absolutizeURLs(raw, base))
}

func absolutizeURLs(raw string, base *url.URL) string {
	nodes, err := xhtml.ParseFragment(strings.NewReader(raw), &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return raw
	}
	var fix func(*xhtml.Node)
	fix = func(n *xhtml.Node) {
		for i, a := range n.Attr {
			if a.Key == "href" || a.Key == "src" {
				if u, err := url.Parse(strings.TrimSpace(a.Val)); err == nil {
					n.Attr[i].Val = base.ResolveReference(u).String()
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			fix(c)
		}
	}
	var b strings.Builder
	for _, n := range nodes {
		fix(n)
		if err := xhtml.Render(&b, n); err != nil {
			return raw
		}
	}
	return b.String()
}

// htmlToText reduces feed HTML to plain text for indexing and snippets.
func htmlToText(s string) string {
	// Space before each tag so words in adjacent elements don't run together.
	text := stripTags.Sanitize(strings.ReplaceAll(s, "<", " <"))
	return strings.Join(strings.Fields(html.UnescapeString(text)), " ")
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func resolveURL(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}
