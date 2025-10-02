package service

import (
	"net/http"
	"strings"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/types"
	"github.com/arashthr/pensive/internal/validations"
	"github.com/arashthr/pensive/web"
)

type Home struct {
	Templates struct {
		Home          web.Template
		RecentResults web.Template
		SearchResults web.Template
		ChatAnswer    web.Template
	}
	BookmarkModel *models.BookmarkRepo
}

func (h Home) Index(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	recent, err := h.getRecentBookmarksData(user, 5)
	if err != nil {
		logger.Errorw("failed to get recent bookmarks data", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	data := struct {
		Title              string
		IsUserPremium      bool
		EmailVerified      bool
		RecentBookmark     types.RecentBookmarksType
		RemainingBookmarks int
	}{
		Title:          "Home",
		IsUserPremium:  user.IsSubscriptionPremium(),
		EmailVerified:  user.EmailVerified,
		RecentBookmark: recent,
	}

	// Get remaining bookmarks for unverified users
	remaining, err := h.BookmarkModel.GetRemainingBookmarks(user)
	if err != nil {
		logger.Warnw("failed to get remaining bookmarks", "error", err)
		data.RemainingBookmarks = 0
	} else {
		data.RemainingBookmarks = remaining
	}

	h.Templates.Home.Execute(w, r, data)
}

func (h Home) Search(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	query := r.FormValue("query")
	if query == "" {
		// Return recent bookmarks when no query
		h.RecentBookmarksResult(w, r)
		return
	}

	results, err := h.BookmarkModel.Search(user, query)
	if err != nil {
		logger.Errorw("failed to search bookmarks", "error", err)
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

// AskQuestion handles RAG-based question answering about bookmarks
func (h Home) AskQuestion(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	question := r.FormValue("question")
	if question == "" {
		http.Error(w, "Question is required", http.StatusBadRequest)
		return
	}

	// Call the RAG method
	response, err := h.BookmarkModel.AskQuestion(r.Context(), user, question)
	if err != nil {
		logger.Errorw("failed to answer question", "error", err, "question", question)

		// Render a user-friendly error message instead of HTTP error
		errorMsg := "I'm having trouble answering your question right now. "

		// Check for specific error types
		errStr := err.Error()
		if strings.Contains(errStr, "overloaded") || strings.Contains(errStr, "503") {
			errorMsg += "The AI service is currently overloaded. Please try again in a moment."
		} else if strings.Contains(errStr, "couldn't find any relevant bookmarks") {
			errorMsg += "I couldn't find any bookmarks related to your question."
		} else {
			errorMsg += "Please try again or rephrase your question."
		}

		data := struct {
			Answer  string
			Sources []types.BookmarkSearchResult
			IsError bool
		}{
			Answer:  errorMsg,
			Sources: []types.BookmarkSearchResult{},
			IsError: true,
		}

		h.Templates.ChatAnswer.Execute(w, r, data)
		return
	}

	// Prepare response data
	var sources []types.BookmarkSearchResult
	for _, bookmark := range response.SourceBookmarks {
		sources = append(sources, types.BookmarkSearchResult{
			Id:        bookmark.Id,
			Title:     bookmark.Title,
			Link:      bookmark.Link,
			Hostname:  validations.ExtractHostname(bookmark.Link),
			Headline:  bookmark.Headline,
			Thumbnail: bookmark.ImageUrl,
			CreatedAt: bookmark.CreatedAt,
		})
	}

	data := struct {
		Answer  string
		Sources []types.BookmarkSearchResult
		IsError bool
	}{
		Answer:  response.Answer,
		Sources: sources,
		IsError: false,
	}

	h.Templates.ChatAnswer.Execute(w, r, data)
}

func (h Home) RecentBookmarksResult(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	data, err := h.getRecentBookmarksData(user, 5)
	if err != nil {
		logger.Errorw("failed to get recent bookmarks data", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	h.Templates.RecentResults.Execute(w, r, data)
}

// getRecentBookmarksData fetches recent bookmarks and returns the data structure
func (h Home) getRecentBookmarksData(user *models.User, limit int) (types.RecentBookmarksType, error) {
	bookmarks, err := h.BookmarkModel.GetRecentBookmarks(user, limit)
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
		if b.AIExcerpt != nil {
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
