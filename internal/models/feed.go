package models

import (
	"context"
	"fmt"
	"time"

	"github.com/arashthr/pensive/internal/types"
	"github.com/arashthr/pensive/internal/validations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxFeedsPerUser caps active subscriptions to keep fetching affordable.
const MaxFeedsPerUser = 300

// Feed is a fetchable RSS/Atom/JSON feed, shared by everyone who follows it.
type Feed struct {
	ID           int64
	URL          string
	ETag         string
	LastModified string
}

// FeedSubscription is a feed as one user sees it.
type FeedSubscription struct {
	FeedID        int64
	URL           string
	Title         string
	SiteURL       string
	LastFetchedAt *time.Time
	ErrorCount    int
	LastError     string
	EntryCount    int
}

// FeedEntry is a post parsed from a feed, ready to store.
type FeedEntry struct {
	GUID         string
	URL          string
	CanonicalURL string
	Title        string
	Author       string
	Content      string
	ContentHTML  string
	PublishedAt  time.Time
}

// FeedEntryItem is a stored post shown in search results or the recent list.
// TitleHeadline and Headline carry validations.HighlightStart/Stop markers.
type FeedEntryItem struct {
	ID            int64
	FeedTitle     string
	URL           string
	Title         string
	TitleHeadline string
	Headline      string
	PublishedAt   time.Time
	// Library bookmark with the same link; empty if not saved
	SavedBookmarkID string
}

// FeedEntryDetail is a single post for the reading view.
type FeedEntryDetail struct {
	ID              int64
	FeedTitle       string
	URL             string
	Title           string
	Author          string
	Content         string
	ContentHTML     string
	PublishedAt     time.Time
	SavedBookmarkID string
}

// FeedStats summarises what a user's feeds have collected.
type FeedStats struct {
	Feeds int
	Posts int
	Since *time.Time
}

type FeedRepo struct {
	Pool *pgxpool.Pool
}

// visibleEntries limits feed_entries e to feeds user $2 follows, or used to
// follow; for a removed feed only posts collected before removal count.
const visibleEntries = `
		JOIN feed_subscriptions s ON s.feed_id = e.feed_id AND s.user_id = $2
			AND (s.unsubscribed_at IS NULL OR e.created_at <= s.unsubscribed_at)
		JOIN feeds f ON f.id = e.feed_id`

// Subscribe follows a feed, creating it if nobody follows it yet. Following a
// previously removed feed reactivates it.
func (r *FeedRepo) Subscribe(ctx context.Context, userID types.UserId, url, title string) (int64, error) {
	var feedID int64
	err := r.Pool.QueryRow(ctx, `
		WITH feed AS (
			INSERT INTO feeds (url) VALUES ($1)
			ON CONFLICT (url) DO UPDATE SET url = EXCLUDED.url
			RETURNING id
		)
		INSERT INTO feed_subscriptions (user_id, feed_id, title)
		SELECT $2, id, $3 FROM feed
		ON CONFLICT (user_id, feed_id) DO UPDATE SET
			unsubscribed_at = NULL,
			title = COALESCE(NULLIF(EXCLUDED.title, ''), feed_subscriptions.title)
		RETURNING feed_id`, url, userID, title).Scan(&feedID)
	if err != nil {
		return 0, fmt.Errorf("subscribe to feed: %w", err)
	}
	return feedID, nil
}

// Unsubscribe stops following a feed; already collected posts stay searchable.
func (r *FeedRepo) Unsubscribe(ctx context.Context, userID types.UserId, feedID int64) error {
	_, err := r.Pool.Exec(ctx, `
		UPDATE feed_subscriptions SET unsubscribed_at = NOW()
		WHERE user_id = $1 AND feed_id = $2 AND unsubscribed_at IS NULL`, userID, feedID)
	if err != nil {
		return fmt.Errorf("unsubscribe from feed: %w", err)
	}
	return nil
}

// Subscriptions lists the feeds a user currently follows.
func (r *FeedRepo) Subscriptions(ctx context.Context, userID types.UserId) ([]FeedSubscription, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT
			f.id AS feed_id,
			f.url,
			COALESCE(NULLIF(s.title, ''), NULLIF(f.title, ''), f.url) AS title,
			f.site_url,
			f.last_fetched_at,
			f.error_count,
			f.last_error,
			(SELECT COUNT(*) FROM feed_entries e WHERE e.feed_id = f.id)::int AS entry_count
		FROM feed_subscriptions s
		JOIN feeds f ON f.id = s.feed_id
		WHERE s.user_id = $1 AND s.unsubscribed_at IS NULL
		ORDER BY lower(COALESCE(NULLIF(s.title, ''), NULLIF(f.title, ''), f.url))`, userID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	subs, err := pgx.CollectRows(rows, pgx.RowToStructByName[FeedSubscription])
	if err != nil {
		return nil, fmt.Errorf("collect subscriptions: %w", err)
	}
	return subs, nil
}

// Stats counts active feeds and every post the user can search.
func (r *FeedRepo) Stats(ctx context.Context, userID types.UserId) (FeedStats, error) {
	var stats FeedStats
	err := r.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM feed_subscriptions WHERE user_id = $1 AND unsubscribed_at IS NULL)::int,
			(SELECT COUNT(*) FROM feed_entries e
				JOIN feed_subscriptions s ON s.feed_id = e.feed_id AND s.user_id = $1
					AND (s.unsubscribed_at IS NULL OR e.created_at <= s.unsubscribed_at))::int,
			(SELECT MIN(created_at) FROM feed_subscriptions WHERE user_id = $1)`, userID,
	).Scan(&stats.Feeds, &stats.Posts, &stats.Since)
	if err != nil {
		return FeedStats{}, fmt.Errorf("feed stats: %w", err)
	}
	return stats, nil
}

// Search finds the user's feed posts matching query, best matches first.
func (r *FeedRepo) Search(ctx context.Context, userID types.UserId, query string, limit int) ([]FeedEntryItem, error) {
	query = sanitizeSearchQuery(query)
	if query == "" {
		return nil, nil
	}
	rows, err := r.Pool.Query(ctx, searchQueryCTE+`
		SELECT
			e.id,
			COALESCE(NULLIF(s.title, ''), NULLIF(f.title, ''), f.url) AS feed_title,
			e.url,
			e.title,
			ts_headline('english', e.title, sq.query, 'HighlightAll=true, StartSel=' || $3::text || ', StopSel=' || $4::text) AS title_headline,
			ts_headline('english', e.content, sq.query, 'MaxFragments=2, StartSel=' || $3::text || ', StopSel=' || $4::text) AS headline,
			e.published_at,
			COALESCE((SELECT li.id FROM library_items li WHERE li.user_id = $2 AND li.link = e.canonical_url LIMIT 1), '') AS saved_bookmark_id
		FROM feed_entries e`+visibleEntries+`
		CROSS JOIN search_query sq
		WHERE sq.query IS NOT NULL AND e.search_vector @@ sq.query
		ORDER BY ts_rank(e.search_vector, sq.query) DESC, e.published_at DESC
		LIMIT $5`, query, userID, validations.HighlightStart, validations.HighlightStop, limit)
	if err != nil {
		return nil, fmt.Errorf("search feed entries: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[FeedEntryItem])
	if err != nil {
		return nil, fmt.Errorf("collect feed search results: %w", err)
	}
	return items, nil
}

// Recent lists the newest posts from feeds the user follows.
func (r *FeedRepo) Recent(ctx context.Context, userID types.UserId, limit int) ([]FeedEntryItem, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT
			e.id,
			COALESCE(NULLIF(s.title, ''), NULLIF(f.title, ''), f.url) AS feed_title,
			e.url,
			e.title,
			e.title AS title_headline,
			left(e.content, 300) AS headline,
			e.published_at,
			COALESCE((SELECT li.id FROM library_items li WHERE li.user_id = $2 AND li.link = e.canonical_url LIMIT 1), '') AS saved_bookmark_id
		FROM feed_entries e`+visibleEntries+`
		WHERE s.unsubscribed_at IS NULL
		ORDER BY e.published_at DESC
		LIMIT $1`, limit, userID)
	if err != nil {
		return nil, fmt.Errorf("recent feed entries: %w", err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[FeedEntryItem])
	if err != nil {
		return nil, fmt.Errorf("collect recent feed entries: %w", err)
	}
	return items, nil
}

// Entry returns a post for the reading view if the user can see it.
func (r *FeedRepo) Entry(ctx context.Context, userID types.UserId, entryID int64) (FeedEntryDetail, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT
			e.id,
			COALESCE(NULLIF(s.title, ''), NULLIF(f.title, ''), f.url) AS feed_title,
			e.url,
			e.title,
			e.author,
			e.content,
			e.content_html,
			e.published_at,
			COALESCE((SELECT li.id FROM library_items li WHERE li.user_id = $2 AND li.link = e.canonical_url LIMIT 1), '') AS saved_bookmark_id
		FROM feed_entries e`+visibleEntries+`
		WHERE e.id = $1`, entryID, userID)
	if err != nil {
		return FeedEntryDetail{}, fmt.Errorf("get feed entry: %w", err)
	}
	entry, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[FeedEntryDetail])
	if err != nil {
		return FeedEntryDetail{}, fmt.Errorf("collect feed entry: %w", err)
	}
	return entry, nil
}

// EntryURL returns a post's link if the user can see it.
func (r *FeedRepo) EntryURL(ctx context.Context, userID types.UserId, entryID int64) (string, error) {
	var url string
	err := r.Pool.QueryRow(ctx, `
		SELECT e.url FROM feed_entries e`+visibleEntries+`
		WHERE e.id = $1`, entryID, userID).Scan(&url)
	if err != nil {
		return "", fmt.Errorf("get feed entry: %w", err)
	}
	return url, nil
}

// DueFeeds returns feeds that at least one user follows and are due a fetch.
func (r *FeedRepo) DueFeeds(ctx context.Context, limit int) ([]Feed, error) {
	rows, err := r.Pool.Query(ctx, `
		SELECT f.id, f.url, f.etag, f.last_modified
		FROM feeds f
		WHERE f.next_fetch_at <= NOW()
			AND EXISTS (SELECT 1 FROM feed_subscriptions s WHERE s.feed_id = f.id AND s.unsubscribed_at IS NULL)
		ORDER BY f.next_fetch_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("due feeds: %w", err)
	}
	feeds, err := pgx.CollectRows(rows, pgx.RowToStructByName[Feed])
	if err != nil {
		return nil, fmt.Errorf("collect due feeds: %w", err)
	}
	return feeds, nil
}

// FetchResult is what a successful fetch learned about a feed.
type FetchResult struct {
	NotModified  bool
	Title        string
	SiteURL      string
	ETag         string
	LastModified string
	Entries      []FeedEntry
}

// SaveFetch stores new posts (existing ones are left untouched) and schedules
// the next fetch.
func (r *FeedRepo) SaveFetch(ctx context.Context, feedID int64, res FetchResult, next time.Time) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin save fetch: %w", err)
	}
	defer tx.Rollback(ctx)

	if !res.NotModified {
		batch := &pgx.Batch{}
		for _, e := range res.Entries {
			batch.Queue(`
				INSERT INTO feed_entries (feed_id, guid, url, canonical_url, title, author, content, content_html, published_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				ON CONFLICT (feed_id, guid) DO UPDATE SET content_html = EXCLUDED.content_html
				WHERE feed_entries.content_html = '' AND EXCLUDED.content_html <> ''`,
				feedID, e.GUID, e.URL, e.CanonicalURL, e.Title, e.Author, e.Content, e.ContentHTML, e.PublishedAt)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return fmt.Errorf("insert feed entries: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE feeds SET
			title = COALESCE(NULLIF($2, ''), title),
			site_url = COALESCE(NULLIF($3, ''), site_url),
			etag = CASE WHEN $6 THEN etag ELSE $4 END,
			last_modified = CASE WHEN $6 THEN last_modified ELSE $5 END,
			last_fetched_at = NOW(),
			next_fetch_at = $7,
			error_count = 0,
			last_error = ''
		WHERE id = $1`,
		feedID, res.Title, res.SiteURL, res.ETag, res.LastModified, res.NotModified, next)
	if err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	return tx.Commit(ctx)
}

// SaveFetchError records a failed fetch and backs off: 2h, 4h, ... up to a day.
func (r *FeedRepo) SaveFetchError(ctx context.Context, feedID int64, message string) error {
	_, err := r.Pool.Exec(ctx, `
		UPDATE feeds SET
			error_count = error_count + 1,
			last_error = $2,
			next_fetch_at = NOW() + LEAST(interval '1 hour' * power(2, error_count + 1), interval '24 hours')
		WHERE id = $1`, feedID, message)
	if err != nil {
		return fmt.Errorf("record feed error: %w", err)
	}
	return nil
}
