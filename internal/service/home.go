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

	page := validations.GetPageOffset(r.FormValue("page"))
	paginatedData, err := h.getPaginatedBookmarksData(user, page)
	if err != nil {
		logger.Errorw("failed to get paginated bookmarks", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	data := struct {
		Title                string
		IsUserPremium        bool
		EmailVerified        bool
		Pages                types.PagesData
		MorePages            bool
		Bookmarks            []types.BookmarkListItem
		Count                int
		HasBookmarksAtAll    bool
		RemainingBookmarks   int
		RemainingAIQuestions int
	}{
		Title:             "Home",
		IsUserPremium:     user.IsSubscriptionPremium(),
		EmailVerified:     user.EmailVerified,
		Pages:             paginatedData.Pages,
		MorePages:         paginatedData.MorePages,
		Bookmarks:         paginatedData.Bookmarks,
		Count:             paginatedData.Count,
		HasBookmarksAtAll: paginatedData.HasBookmarksAtAll,
	}

	// Get remaining bookmarks for unverified users
	remaining, err := h.BookmarkModel.GetRemainingBookmarks(user)
	if err != nil {
		logger.Warnw("failed to get remaining bookmarks", "error", err)
		data.RemainingBookmarks = 0
	} else {
		data.RemainingBookmarks = remaining
	}

	// Get remaining AI questions for this month
	remainingAI, err := h.BookmarkModel.GetRemainingAIQuestions(user)
	if err != nil {
		logger.Warnw("failed to get remaining AI questions", "error", err)
		data.RemainingAIQuestions = 0
	} else {
		data.RemainingAIQuestions = remainingAI
	}

	h.Templates.Home.Execute(w, r, data)
}

func (h Home) Search(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	query := r.FormValue("query")
	page := validations.GetPageOffset(r.FormValue("page"))

	if query == "" {
		// Return paginated bookmarks when no query
		data, err := h.getPaginatedBookmarksData(user, page)
		if err != nil {
			logger.Errorw("failed to get paginated bookmarks", "error", err)
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}

		h.Templates.RecentResults.Execute(w, r, data)
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

	// Check and increment AI question limit
	if err := h.BookmarkModel.CheckAndIncrementAIQuestionLimit(r.Context(), user); err != nil {
		logger.Warnw("AI question limit exceeded", "error", err, "user_id", user.ID)

		// Render a user-friendly error message
		errorMsg := "You've reached your daily limit for AI questions. "
		if user.IsSubscriptionPremium() {
			errorMsg += "You seem to have used all your questions for today. Contact me if you need more."
		} else {
			errorMsg += "Upgrade to premium for more questions, or wait until tomorrow."
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

// getPaginatedBookmarksData fetches paginated bookmarks and returns the data structure
func (h Home) getPaginatedBookmarksData(user *models.User, page int) (types.PaginatedBookmarksType, error) {
	bookmarks, count, morePages, err := h.BookmarkModel.GetByUserId(user.ID, page)
	if err != nil {
		return types.PaginatedBookmarksType{}, err
	}

	var data types.PaginatedBookmarksType
	data.Count = count
	data.Pages = types.PagesData{
		Previous: page - 1,
		Current:  page,
		Next:     page + 1,
	}
	data.MorePages = morePages

	for _, b := range bookmarks {
		excerpt := b.Excerpt
		if b.AIExcerpt != nil {
			excerpt = *b.AIExcerpt
		}
		cleanedExcerpt := validations.CleanUpText(excerpt)
		data.Bookmarks = append(data.Bookmarks, types.BookmarkListItem{
			Id:        b.Id,
			Title:     b.Title,
			Link:      b.Link,
			CreatedAt: b.CreatedAt.Format("Jan 02"),
			Excerpt:   cleanedExcerpt,
		})
	}

	data.HasBookmarksAtAll = len(data.Bookmarks) > 0
	return data, nil
}
