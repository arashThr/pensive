package service

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/arashthr/pensive/internal/models"
	"github.com/mmcdole/gofeed"
)

func TestParseOPML(t *testing.T) {
	doc := `<?xml version="1.0" encoding="ISO-8859-1"?>
<opml version="2.0">
  <head><title>My feeds</title></head>
  <body>
    <outline text="Tech">
      <outline text="Julia Evans" type="rss" xmlUrl="https://jvns.ca/atom.xml" htmlUrl="https://jvns.ca"/>
      <outline title="Dup" xmlUrl=" https://jvns.ca/atom.xml "/>
    </outline>
    <outline text="Caf` + "\xe9" + `" xmlUrl="https://example.com/feed"/>
    <outline text="Not a feed" xmlUrl="javascript:alert(1)"/>
    <outline text="Folder without feeds"/>
  </body>
</opml>`
	feeds, err := ParseOPML(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := []OPMLFeed{
		{URL: "https://jvns.ca/atom.xml", Title: "Julia Evans"},
		{URL: "https://example.com/feed", Title: "Café"},
	}
	if len(feeds) != len(want) {
		t.Fatalf("got %d feeds %+v, want %d", len(feeds), feeds, len(want))
	}
	for i := range want {
		if feeds[i] != want[i] {
			t.Errorf("feed %d = %+v, want %+v", i, feeds[i], want[i])
		}
	}

	if _, err := ParseOPML(strings.NewReader("not xml")); err == nil {
		t.Error("expected an error for invalid OPML")
	}
}

func TestOPMLRoundTrip(t *testing.T) {
	in := []OPMLFeed{{URL: "https://a.example/feed", Title: "A & B"}, {URL: "https://b.example/rss", Title: "B"}}
	var buf bytes.Buffer
	if err := WriteOPML(&buf, "Pensive feeds", in); err != nil {
		t.Fatal(err)
	}
	out, err := ParseOPML(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != in[0] || out[1] != in[1] {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
}

func TestDiffSubscriptions(t *testing.T) {
	current := []models.FeedSubscription{
		{FeedID: 1, URL: "https://keep.example/feed"},
		{FeedID: 2, URL: "https://gone.example/feed"},
	}
	file := []OPMLFeed{{URL: "https://keep.example/feed"}, {URL: "https://new.example/feed"}}
	diff := diffSubscriptions(current, file)
	if diff.unchanged != 1 || len(diff.remove) != 1 || diff.remove[0] != 2 ||
		len(diff.add) != 1 || diff.add[0].URL != "https://new.example/feed" {
		t.Errorf("unexpected diff: %+v", diff)
	}
}

func TestHTMLToText(t *testing.T) {
	got := htmlToText(`<p>Hello&nbsp;<b>world</b></p><p>Next</p><script>alert(1)</script> a &lt; b`)
	if got != "Hello world Next a < b" {
		t.Errorf("htmlToText = %q", got)
	}
}

func TestEntriesFromFeed(t *testing.T) {
	rss := `<rss version="2.0"><channel><title>Blog</title><link>https://blog.example/</link>
<item><title>First &amp; best</title><link>/posts/1</link><description>&lt;p&gt;Body text&lt;/p&gt;</description><pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate></item>
<item><title>No link</title></item>
</channel></rss>`
	feed, err := gofeed.NewParser().Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("https://blog.example/feed.xml")
	entries := entriesFromFeed(feed, base)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (items without a link are skipped)", len(entries))
	}
	e := entries[0]
	if e.URL != "https://blog.example/posts/1" || e.GUID != e.URL || e.Title != "First & best" || e.Content != "Body text" || e.PublishedAt.Year() != 2006 {
		t.Errorf("unexpected entry: %+v", e)
	}
}

func TestFeedLinkInHTML(t *testing.T) {
	page := `<html><head><link rel="stylesheet" href="/s.css"><link rel="alternate" type="application/atom+xml" href="/atom.xml"></head><body></body></html>`
	base, _ := url.Parse("https://site.example/blog/")
	if got := feedLinkInHTML([]byte(page), base); got != "https://site.example/atom.xml" {
		t.Errorf("feedLinkInHTML = %q", got)
	}
}

func TestFeedClientBlocksInternalAddresses(t *testing.T) {
	f := &Feeds{HTTPClient: newFeedHTTPClient()}
	for _, u := range []string{"http://127.0.0.1:9/feed", "http://10.0.0.1/feed", "http://169.254.169.254/latest/meta-data"} {
		_, err := f.fetchFeed(context.Background(), models.Feed{URL: u})
		if err == nil || !strings.Contains(err.Error(), "refusing to fetch internal address") {
			t.Errorf("%s: expected internal address to be refused, got %v", u, err)
		}
	}
}

func TestFeedHTML(t *testing.T) {
	base, _ := url.Parse("https://blog.example/posts/1")
	got := feedHTML(`<p>Hi <a href="/about">me</a></p><img src="img/a.png" onerror="alert(1)"><script>alert(1)</script><iframe src="https://evil.example"></iframe>`, base)
	for _, want := range []string{`href="https://blog.example/about"`, `target="_blank"`, `src="https://blog.example/posts/img/a.png"`} {
		if !strings.Contains(got, want) {
			t.Errorf("feedHTML missing %s in %s", want, got)
		}
	}
	for _, bad := range []string{"<script", "onerror", "<iframe"} {
		if strings.Contains(got, bad) {
			t.Errorf("feedHTML kept %s: %s", bad, got)
		}
	}
	if feedHTML("   ", base) != "" {
		t.Error("blank content should produce no HTML")
	}
}

func TestEntriesFromFeedKeepsLinkScheme(t *testing.T) {
	rss := `<rss version="2.0"><channel><item><title>Post</title><link>http://old.example/p</link><description>&lt;a href="/about"&gt;me&lt;/a&gt;</description></item></channel></rss>`
	feed, err := gofeed.NewParser().Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatal(err)
	}
	e := entriesFromFeed(feed, nil)[0]
	if e.CanonicalURL != "https://old.example/p" || !strings.Contains(e.ContentHTML, `href="http://old.example/about"`) {
		t.Errorf("canonical %q, html %q: canonicalizing must not change the base used for links", e.CanonicalURL, e.ContentHTML)
	}
}
