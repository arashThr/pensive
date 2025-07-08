package service

import (
	"net/http"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/types"
	"github.com/arashthr/go-course/web"
)

type Home struct {
	Templates struct {
		Home          web.Template
		RecentResults web.Template
		SearchResults web.Template
	}
	BookmarkModel *models.BookmarkModel
}

func (h Home) Index(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())

	// Get recent bookmarks (limit to 5)
	bookmarks, err := h.BookmarkModel.GetRecentBookmarks(user.ID, 5)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	type RecentBookmark struct {
		Id        types.BookmarkId
		Title     string
		Link      string
		Excerpt   string
		Thumbnail string
		CreatedAt string
	}

	var data struct {
		Bookmarks         []RecentBookmark
		HasBookmarksAtAll bool
	}

	for _, b := range bookmarks {
		data.Bookmarks = append(data.Bookmarks, RecentBookmark{
			Id:        b.BookmarkId,
			Title:     b.Title,
			Link:      b.Link,
			Excerpt:   b.Excerpt,
			Thumbnail: b.ImageUrl,
			CreatedAt: b.CreatedAt.Format("Jan 02, 2006"),
		})
	}

	data.HasBookmarksAtAll = len(data.Bookmarks) > 0

	h.Templates.Home.Execute(w, r, data)
}

func (h Home) Search(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("query")
	if query == "" {
		// Return recent bookmarks when no query
		h.RecentBookmarksResult(w, r)
		return
	}

	user := context.User(r.Context())
	results, err := h.BookmarkModel.Search(user.ID, query)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	var data struct {
		Bookmarks  []types.BookmarkSearchResult
		Query      string
		HasResults bool
	}

	data.Query = query
	for _, r := range results {
		data.Bookmarks = append(data.Bookmarks, types.BookmarkSearchResult{
			Id:        r.BookmarkId,
			Title:     r.Title,
			Link:      r.Link,
			Headline:  r.Headline,
			Thumbnail: r.ImageUrl,
			CreatedAt: r.CreatedAt,
		})
	}

	data.HasResults = len(data.Bookmarks) > 0

	h.Templates.SearchResults.Execute(w, r, data)
}

func (h Home) RecentBookmarksResult(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())

	// Get recent bookmarks (limit to 5)
	bookmarks, err := h.BookmarkModel.GetRecentBookmarks(user.ID, 5)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	type RecentBookmark struct {
		Id        types.BookmarkId
		Title     string
		Link      string
		Excerpt   string
		Thumbnail string
		CreatedAt string
	}

	var data struct {
		Bookmarks         []RecentBookmark
		HasBookmarksAtAll bool
	}

	for _, b := range bookmarks {
		data.Bookmarks = append(data.Bookmarks, RecentBookmark{
			Id:        b.BookmarkId,
			Title:     b.Title,
			Link:      b.Link,
			Excerpt:   b.Excerpt,
			Thumbnail: b.ImageUrl,
			CreatedAt: b.CreatedAt.Format("Jan 02, 2006"),
		})
	}

	data.HasBookmarksAtAll = len(data.Bookmarks) > 0

	h.Templates.RecentResults.Execute(w, r, data)
}
