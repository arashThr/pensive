package types

import "time"

type BookmarkId string

// Extraction method constants
type ExtractionMethod string

const (
	ExtractionMethodServer      ExtractionMethod = "server-side"
	ExtractionMethodReadability ExtractionMethod = "client-readability"
	ExtractionMethodHTML        ExtractionMethod = "client-html-extraction"
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

type RecentBookmark struct {
	Id        BookmarkId
	Title     string
	Link      string
	Hostname  string
	Excerpt   string
	Thumbnail string
	CreatedAt string
}

type RecentBookmarksType struct {
	Bookmarks         []RecentBookmark
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
