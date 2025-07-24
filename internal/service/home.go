package service

import (
	"net/http"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/types"
	"github.com/arashthr/go-course/internal/validations"
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
	logger := context.Logger(r.Context())

	recent, err := h.getRecentBookmarksData(user.ID, 5, user.SubscriptionStatus)
	if err != nil {
		logger.Error("failed to get recent bookmarks data", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	data := struct {
		IsUserPremium  bool
		RecentBookmark types.RecentBookmarksType
	}{
		IsUserPremium:  user.SubscriptionStatus == models.SubscriptionStatusPremium,
		RecentBookmark: recent,
	}

	h.Templates.Home.Execute(w, r, data)
}

func (h Home) Search(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	logger := context.Logger(r.Context())

	query := r.FormValue("query")
	if query == "" {
		// Return recent bookmarks when no query
		h.RecentBookmarksResult(w, r)
		return
	}

	results, err := h.BookmarkModel.Search(user.ID, query, user.SubscriptionStatus)
	if err != nil {
		logger.Error("failed to search bookmarks", "error", err)
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
			Id:        r.Id,
			Title:     r.Title,
			Link:      r.Link,
			Hostname:  validations.ExtractHostname(r.Link),
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
	logger := context.Logger(r.Context())

	data, err := h.getRecentBookmarksData(user.ID, 5, user.SubscriptionStatus)
	if err != nil {
		logger.Error("failed to get recent bookmarks data", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	h.Templates.RecentResults.Execute(w, r, data)
}

// getRecentBookmarksData fetches recent bookmarks and returns the data structure
func (h Home) getRecentBookmarksData(userId types.UserId, limit int, subscriptionStatus models.SubscriptionStatus) (types.RecentBookmarksType, error) {
	bookmarks, err := h.BookmarkModel.GetRecentBookmarks(userId, limit, subscriptionStatus)
	if err != nil {
		return struct {
			Bookmarks         []types.RecentBookmark
			HasBookmarksAtAll bool
		}{}, err
	}

	var data struct {
		Bookmarks         []types.RecentBookmark
		HasBookmarksAtAll bool
	}

	for _, b := range bookmarks {
		excerpt := b.Excerpt
		if subscriptionStatus == models.SubscriptionStatusPremium && b.AIExcerpt != nil {
			excerpt = *b.AIExcerpt
		}
		data.Bookmarks = append(data.Bookmarks, types.RecentBookmark{
			Id:        b.Id,
			Title:     b.Title,
			Link:      b.Link,
			Hostname:  validations.ExtractHostname(b.Link),
			Excerpt:   excerpt,
			Thumbnail: b.ImageUrl,
			CreatedAt: b.CreatedAt.Format("Jan 02, 2006"),
		})
	}

	data.HasBookmarksAtAll = len(data.Bookmarks) > 0
	return data, nil
}
