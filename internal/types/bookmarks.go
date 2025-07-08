package types

import "time"

type BookmarkId string

type BookmarkSearchResult struct {
	Id        BookmarkId
	Title     string
	Link      string
	Headline  string
	Thumbnail string
	CreatedAt time.Time
}
