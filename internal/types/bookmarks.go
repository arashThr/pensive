package types

import "time"

type BookmarkId string

// Extraction method constants
type ExtractionMethod string

const (
	ExtractionMethodServer          ExtractionMethod = "server-side"
	ExtractionMethodReadability     ExtractionMethod = "client-readability"
	ExtractionMethodReadabilityHTML ExtractionMethod = "client-readability-html"
	ExtractionMethodHTML            ExtractionMethod = "client-html"
)

type BookmarkSearchResult struct {
	Id        BookmarkId
	Title     string
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
