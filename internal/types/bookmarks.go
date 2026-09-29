package types

import (
	"html/template"
	"time"
)

type BookmarkId string

// Extraction method constants
type ExtractionMethod string

const (
	ExtractionMethodServer          ExtractionMethod = "server-side"
	ExtractionMethodReadability     ExtractionMethod = "client-readability"
	ExtractionMethodReadabilityHTML ExtractionMethod = "client-readability-html"
	ExtractionMethodHTML            ExtractionMethod = "client-html"
)

// FeedSearchResult is a post from a followed feed, ready to render.
type FeedSearchResult struct {
	ID           int64
	FeedTitle    string
	URL          string
	TitleHTML    template.HTML
	HeadlineHTML template.HTML
	PublishedAt  time.Time
	Saved        bool
}

type BookmarkSearchResult struct {
	Id    BookmarkId
	Title string
	// Escaped title with search matches in <strong>; web UI only
	TitleHTML template.HTML `json:"-"`
	Link      string
	Hostname  string
	Headline  string
	Thumbnail string
	CreatedAt time.Time
}

type BookmarkListItem struct {
	Id        BookmarkId
	Title     string
	Link      string
	Hostname  string
	CreatedAt string
	Excerpt   string
}

type PagesData struct {
	Previous int
	Current  int
	Next     int
}

type PaginatedBookmarksType struct {
	Pages             PagesData
	MorePages         bool
	Bookmarks         []BookmarkListItem
	Count             int
	HasBookmarksAtAll bool
}

type CreateBookmarkRequest struct {
	Link          string     `json:"link"`
	HtmlContent   string     `json:"htmlContent"`
	Title         string     `json:"title"`
	Excerpt       string     `json:"excerpt"`
	Lang          string     `json:"lang"`
	SiteName      string     `json:"siteName"`
	TextContent   string     `json:"textContent"`
	PublishedTime *time.Time `json:"publishedTime"`
}
