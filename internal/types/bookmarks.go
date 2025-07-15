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
