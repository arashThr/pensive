package service

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/errors"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/types"
	"github.com/arashthr/pensive/internal/validations"
	"github.com/arashthr/pensive/web"
	"github.com/go-chi/chi/v5"
)

const (
	feedSchedulerInterval = time.Minute
	feedFetchBatch        = 40
	feedFetchWorkers      = 4
	feedRecentLimit       = 40
	opmlMaxBytes          = 2 << 20
)

// Feeds lets users follow RSS/Atom feeds so their posts become searchable
// alongside the library. A background scheduler keeps feeds fresh.
type Feeds struct {
	FeedModel     *models.FeedRepo
	BookmarkModel *models.BookmarkRepo
	// HTTPClient fetches feeds; nil uses a client that blocks internal addresses.
	HTTPClient *http.Client
	Templates  struct {
		Index web.Template
		Entry web.Template
		Saved web.Template
	}
	wake chan struct{}
}

func NewFeeds(feedModel *models.FeedRepo, bookmarkModel *models.BookmarkRepo) *Feeds {
	return &Feeds{
		FeedModel:     feedModel,
		BookmarkModel: bookmarkModel,
		HTTPClient:    newFeedHTTPClient(),
		wake:          make(chan struct{}, 1),
	}
}

func (f *Feeds) client() *http.Client {
	if f.HTTPClient == nil {
		f.HTTPClient = newFeedHTTPClient()
	}
	return f.HTTPClient
}

// StartScheduler refreshes due feeds every minute, or sooner when woken
// after an import.
func (f *Feeds) StartScheduler(ctx context.Context) {
	logging.Logger.With("flow", "feeds").Infow("Starting")
	ticker := time.NewTicker(feedSchedulerInterval)
	defer ticker.Stop()
	for {
		f.refreshDue(ctx)
		select {
		case <-ctx.Done():
			logging.Logger.With("flow", "feeds").Infow("Stopping")
			return
		case <-ticker.C:
		case <-f.wake:
		}
	}
}

func (f *Feeds) wakeScheduler() {
	select {
	case f.wake <- struct{}{}:
	default:
	}
}

func (f *Feeds) refreshDue(ctx context.Context) {
	feeds, err := f.FeedModel.DueFeeds(ctx, feedFetchBatch)
	if err != nil {
		logging.Logger.With("flow", "feeds").Errorw("load due feeds", "error", err)
		return
	}
	var wg sync.WaitGroup
	slots := make(chan struct{}, feedFetchWorkers)
	for _, feed := range feeds {
		wg.Add(1)
		slots <- struct{}{}
		go func(feed models.Feed) {
			defer wg.Done()
			defer func() { <-slots }()
			f.refresh(ctx, feed)
		}(feed)
	}
	wg.Wait()
}

// refresh fetches one feed and records the outcome, backing off on errors.
func (f *Feeds) refresh(ctx context.Context, feed models.Feed) {
	logger := logging.Logger.With("flow", "feeds", "feed_id", feed.ID, "url", feed.URL)
	res, err := f.fetchFeed(ctx, feed)
	if err != nil {
		logger.Warnw("fetch feed failed", "error", err)
		if err := f.FeedModel.SaveFetchError(ctx, feed.ID, truncateRunes(err.Error(), 300)); err != nil {
			logger.Errorw("record feed error", "error", err)
		}
		return
	}
	if err := f.FeedModel.SaveFetch(ctx, feed.ID, res, time.Now().Add(feedRefreshInterval)); err != nil {
		logger.Errorw("save fetched feed", "error", err)
	}
}

func (f *Feeds) Index(w http.ResponseWriter, r *http.Request) {
	f.renderIndex(w, r)
}

func (f *Feeds) renderIndex(w http.ResponseWriter, r *http.Request, msgs ...web.NavbarMessage) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	var data struct {
		Title         string
		Subscriptions []models.FeedSubscription
		Recent        []types.FeedSearchResult
		Stats         models.FeedStats
		// Set when viewing a single feed (?feed=ID)
		Feed       *models.FeedSubscription
		Page       int
		PrevPage   int
		NextPage   int
		HasMore    bool
		PageParams string // query string for page links, minus page
	}
	data.Title = "Feeds"
	var err error
	if data.Subscriptions, err = f.FeedModel.Subscriptions(ctx, user.ID); err != nil {
		logger.Errorw("list feed subscriptions", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	var feedID int64
	if id, err := strconv.ParseInt(r.URL.Query().Get("feed"), 10, 64); err == nil {
		for i := range data.Subscriptions {
			if data.Subscriptions[i].FeedID == id {
				data.Feed = &data.Subscriptions[i]
				data.Title = data.Feed.Title
				data.PageParams = fmt.Sprintf("feed=%d&", id)
				feedID = id
			}
		}
	}
	data.Page = validations.GetPageOffset(r.URL.Query().Get("page"))
	data.PrevPage, data.NextPage = data.Page-1, data.Page+1

	// Fetch one extra post to know whether there's an older page.
	recent, err := f.FeedModel.Recent(ctx, user.ID, feedID, feedRecentLimit+1, (data.Page-1)*feedRecentLimit)
	if err != nil {
		logger.Errorw("list recent feed entries", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if len(recent) > feedRecentLimit {
		data.HasMore = true
		recent = recent[:feedRecentLimit]
	}
	data.Recent = feedSearchResults(recent)
	if data.Stats, err = f.FeedModel.Stats(ctx, user.ID); err != nil {
		logger.Errorw("feed stats", "error", err)
	}
	f.Templates.Index.Execute(w, r, data, msgs...)
}

// Add follows a single feed. The URL may be the feed itself or a page that
// links to it. The feed is fetched right away so its posts show up.
func (f *Feeds) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	rawURL := strings.TrimSpace(r.FormValue("url"))
	if !validations.IsURLValid(rawURL) {
		f.renderIndex(w, r, errorMessage("That doesn't look like a valid URL."))
		return
	}
	subs, err := f.FeedModel.Subscriptions(ctx, user.ID)
	if err != nil {
		logger.Errorw("list feed subscriptions", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if len(subs) >= models.MaxFeedsPerUser {
		f.renderIndex(w, r, errorMessage(fmt.Sprintf("You can follow up to %d feeds.", models.MaxFeedsPerUser)))
		return
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	feedURL, err := f.discoverFeed(fetchCtx, rawURL)
	if err != nil {
		logger.Infow("feed discovery failed", "url", rawURL, "error", err)
		f.renderIndex(w, r, errorMessage("Couldn't find a feed at that address: "+err.Error()))
		return
	}
	feedID, err := f.FeedModel.Subscribe(ctx, user.ID, feedURL, "")
	if err != nil {
		logger.Errorw("subscribe to feed", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	f.refresh(fetchCtx, models.Feed{ID: feedID, URL: feedURL})
	logger.Infow("feed added", "feed_id", feedID, "url", feedURL)
	f.renderIndex(w, r, web.NavbarMessage{Message: "Feed added. Its posts are now searchable."})
}

// Import syncs subscriptions with an uploaded OPML file: feeds new in the file
// are followed, feeds missing from it are unfollowed (their posts are kept).
func (f *Feeds) Import(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	r.Body = http.MaxBytesReader(w, r.Body, opmlMaxBytes)
	file, _, err := r.FormFile("opml")
	if err != nil {
		f.renderIndex(w, r, errorMessage("Please choose an OPML file (up to 2 MB)."))
		return
	}
	defer file.Close()
	fileFeeds, err := ParseOPML(file)
	if err != nil {
		f.renderIndex(w, r, errorMessage("That file isn't a valid OPML export."))
		return
	}
	// An empty file would otherwise unfollow everything.
	if len(fileFeeds) == 0 {
		f.renderIndex(w, r, errorMessage("No feeds found in that file. Nothing was changed."))
		return
	}

	subs, err := f.FeedModel.Subscriptions(ctx, user.ID)
	if err != nil {
		logger.Errorw("list feed subscriptions", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	diff := diffSubscriptions(subs, fileFeeds)

	var removedNames []string
	for _, sub := range diff.remove {
		if err := f.FeedModel.Unsubscribe(ctx, user.ID, sub.FeedID); err != nil {
			logger.Errorw("unsubscribe during import", "error", err, "feed_id", sub.FeedID)
			continue
		}
		removedNames = append(removedNames, sub.Title)
	}
	room := models.MaxFeedsPerUser - (len(subs) - len(diff.remove))
	added, skipped := 0, 0
	for _, feed := range diff.add {
		if added >= room {
			skipped++
			continue
		}
		if _, err := f.FeedModel.Subscribe(ctx, user.ID, feed.URL, feed.Title); err != nil {
			logger.Errorw("subscribe during import", "error", err, "url", feed.URL)
			skipped++
			continue
		}
		added++
	}
	f.wakeScheduler()
	logger.Infow("OPML imported", "added", added, "removed", len(diff.remove), "unchanged", diff.unchanged, "skipped", skipped)

	msg := fmt.Sprintf("Synced: %d added, %d removed, %d unchanged.", added, len(diff.remove), diff.unchanged)
	if len(removedNames) > 0 {
		msg += " No longer following: " + listNames(removedNames, 10) + "."
	}
	if added > 0 {
		msg += " New posts will appear within a few minutes."
	}
	if skipped > 0 {
		msg += fmt.Sprintf(" %d feeds were skipped (limit is %d).", skipped, models.MaxFeedsPerUser)
	}
	f.renderIndex(w, r, web.NavbarMessage{Message: msg})
}

type subscriptionDiff struct {
	add       []OPMLFeed
	remove    []models.FeedSubscription
	unchanged int
}

func diffSubscriptions(current []models.FeedSubscription, file []OPMLFeed) subscriptionDiff {
	var diff subscriptionDiff
	// Compare by FeedURLKey so http/https or trailing-slash differences between
	// the reader's export and Pensive don't count as removed + added.
	inFile := map[string]bool{}
	for _, feed := range file {
		inFile[models.FeedURLKey(feed.URL)] = true
	}
	followed := map[string]bool{}
	for _, sub := range current {
		key := models.FeedURLKey(sub.URL)
		followed[key] = true
		if inFile[key] {
			diff.unchanged++
		} else {
			diff.remove = append(diff.remove, sub)
		}
	}
	for _, feed := range file {
		key := models.FeedURLKey(feed.URL)
		if !followed[key] {
			followed[key] = true // a file listing two variants adds one feed
			diff.add = append(diff.add, feed)
		}
	}
	return diff
}

// Export downloads the user's subscriptions as OPML.
func (f *Feeds) Export(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := usercontext.User(ctx)
	subs, err := f.FeedModel.Subscriptions(ctx, user.ID)
	if err != nil {
		loggercontext.Logger(ctx).Errorw("list feed subscriptions", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	feeds := make([]OPMLFeed, len(subs))
	for i, sub := range subs {
		feeds[i] = OPMLFeed{URL: sub.URL, Title: sub.Title}
	}
	w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="pensive-feeds.opml"`)
	if err := WriteOPML(w, "Pensive feeds", feeds); err != nil {
		loggercontext.Logger(ctx).Errorw("write OPML", "error", err)
	}
}

func (f *Feeds) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := usercontext.User(ctx)
	feedID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err := f.FeedModel.Unsubscribe(ctx, user.ID, feedID); err != nil {
		loggercontext.Logger(ctx).Errorw("unsubscribe from feed", "error", err, "feed_id", feedID)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/feeds", http.StatusSeeOther)
}

// ShowEntry shows a post as its feed provides it, with Save and Open actions.
func (f *Feeds) ShowEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := usercontext.User(ctx)
	entryID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	entry, err := f.FeedModel.Entry(ctx, user.ID, entryID)
	if err != nil {
		loggercontext.Logger(ctx).Infow("feed entry not found", "entry_id", entryID, "error", err)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	data := struct {
		Title string
		models.FeedEntryDetail
		// Sanitized when the feed was fetched (see feedHTML); empty for
		// posts stored before HTML was kept, which show Content instead
		HTML template.HTML
	}{
		Title:           entry.Title,
		FeedEntryDetail: entry,
		HTML:            template.HTML(entry.ContentHTML),
	}
	f.Templates.Entry.Execute(w, r, data)
}

// SaveEntry adds a feed post to the library (htmx; returns the saved state).
func (f *Feeds) SaveEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	var data struct {
		BookmarkID types.BookmarkId
		Error      string
	}
	entryID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	link, err := f.FeedModel.EntryURL(ctx, user.ID, entryID)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	bookmark, err := f.BookmarkModel.Create(ctx, link, user, models.FeedSource)
	switch {
	case errors.Is(err, errors.ErrUnverifiedUserLimitExceeded):
		data.Error = "Verify your email to save more."
	case errors.Is(err, errors.ErrDailyLimitExceeded):
		data.Error = "Daily limit reached."
	case err != nil:
		logger.Errorw("save feed entry", "error", err, "entry_id", entryID)
		data.Error = "Couldn't save this page."
	default:
		data.BookmarkID = bookmark.Id
		logger.Infow("feed entry saved", "entry_id", entryID, "bookmark_id", bookmark.Id)
	}
	f.Templates.Saved.Execute(w, r, data)
}

// feedSearchResults escapes post titles and snippets for display, turning
// search-match markers into <strong>.
func feedSearchResults(entries []models.FeedEntryItem) []types.FeedSearchResult {
	results := make([]types.FeedSearchResult, len(entries))
	for i, e := range entries {
		results[i] = types.FeedSearchResult{
			ID:              e.ID,
			FeedTitle:       e.FeedTitle,
			URL:             e.URL,
			TitleHTML:       validations.HighlightedHTML(e.TitleHeadline),
			HeadlineHTML:    validations.HighlightedHTML(e.Headline),
			PublishedAt:     e.PublishedAt,
			SavedBookmarkID: e.SavedBookmarkID,
		}
	}
	return results
}

// listNames joins up to max names, summarizing the rest ("A, B and 3 more").
func listNames(names []string, max int) string {
	if len(names) <= max {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:max], ", "), len(names)-max)
}

func errorMessage(msg string) web.NavbarMessage {
	return web.NavbarMessage{Message: msg, IsError: true}
}
