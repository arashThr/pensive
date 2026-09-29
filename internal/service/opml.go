package service

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html/charset"
)

// OPMLFeed is one subscription listed in an OPML file.
type OPMLFeed struct {
	URL   string
	Title string
}

type opmlOutline struct {
	Text     string        `xml:"text,attr,omitempty"`
	Title    string        `xml:"title,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Type     string        `xml:"type,attr,omitempty"`
	Outlines []opmlOutline `xml:"outline"`
}

type opmlDocument struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    struct {
		Title string `xml:"title"`
	} `xml:"head"`
	Body struct {
		Outlines []opmlOutline `xml:"outline"`
	} `xml:"body"`
}

// ParseOPML returns every feed in an OPML export, flattening folders and
// dropping duplicates and non-HTTP(S) URLs.
func ParseOPML(r io.Reader) ([]OPMLFeed, error) {
	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charset.NewReaderLabel
	var doc opmlDocument
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse OPML: %w", err)
	}

	var feeds []OPMLFeed
	seen := map[string]bool{}
	var walk func([]opmlOutline)
	walk = func(outlines []opmlOutline) {
		for _, o := range outlines {
			feedURL := strings.TrimSpace(o.XMLURL)
			if u, err := url.Parse(feedURL); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && !seen[feedURL] {
				seen[feedURL] = true
				title := strings.TrimSpace(o.Title)
				if title == "" {
					title = strings.TrimSpace(o.Text)
				}
				feeds = append(feeds, OPMLFeed{URL: feedURL, Title: title})
			}
			walk(o.Outlines)
		}
	}
	walk(doc.Body.Outlines)
	return feeds, nil
}

// WriteOPML writes feeds as an OPML 2.0 document.
func WriteOPML(w io.Writer, title string, feeds []OPMLFeed) error {
	doc := opmlDocument{Version: "2.0"}
	doc.Head.Title = title
	for _, f := range feeds {
		doc.Body.Outlines = append(doc.Body.Outlines, opmlOutline{Text: f.Title, Title: f.Title, XMLURL: f.URL, Type: "rss"})
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(doc)
}
