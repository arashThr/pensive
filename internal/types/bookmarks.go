package types

type BookmarkId string

type BookmarkSearchResult struct {
	Id       BookmarkId
	Title    string
	Link     string
	Headline string
}
