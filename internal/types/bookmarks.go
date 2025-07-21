package types

import "time"

type BookmarkId string

type BookmarkSearchResult struct {
	Id        BookmarkId
	Title     string
	Link      string
	Hostname  string
	Headline  string
	Thumbnail string
	CreatedAt time.Time
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
