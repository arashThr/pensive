package models

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/types"
	"github.com/go-shiori/go-readability"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/microcosm-cc/bluemonday"
)

var sanitization = bluemonday.StrictPolicy()

type BookmarkSource = int

const (
	WebSource BookmarkSource = iota
	TelegramSource
	Api
)

var sourceMapping = map[BookmarkSource]string{
	WebSource:      "web",
	TelegramSource: "telegram",
	Api:            "api",
}

type Bookmark struct {
	BookmarkId    types.BookmarkId
	UserId        types.UserId
	Title         string
	Link          string
	Content       string
	Source        string
	Excerpt       string
	ImageUrl      string
	ArticleLang   string
	SiteName      string
	PublishedTime string
	CreatedAt     time.Time
}

type BookmarkModel struct {
	Pool *pgxpool.Pool
}

// TODO: Add validation of the db query inputs (Like Id)
func (service *BookmarkModel) Create(link string, userId types.UserId, source BookmarkSource) (*Bookmark, error) {
	// TODO: Check if the website exists
	article, err := readability.FromURL(link, 5*time.Second)
	// TODO: Check for the language
	if err != nil {
		return nil, fmt.Errorf("readability: %w", err)
	}

	fmt.Printf("%+v\n\n", article)
	bookmark := Bookmark{
		UserId:   userId,
		Title:    sanitization.Sanitize(article.Title),
		Link:     link,
		Excerpt:  sanitization.Sanitize(article.Excerpt),
		ImageUrl: article.Image,
	}
	bookmarkId := strings.ToLower(rand.Text())[:8]

	if _, err := url.ParseRequestURI(article.Image); err != nil {
		slog.Warn("Failed to parse image URL", "error", err)
		bookmark.ImageUrl = ""
	}
	content := sanitization.Sanitize(article.TextContent)

	row := service.Pool.QueryRow(context.Background(),
		`INSERT INTO bookmarks (bookmark_id, user_id, title, link, content, excerpt, image_url, article_lang, site_name, published_time, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING bookmark_id;`,
		bookmarkId, userId, article.Title, link, content,
		bookmark.Excerpt, article.Image, sanitization.Sanitize(article.Language), sanitization.Sanitize(article.SiteName),
		article.PublishedTime, sourceMapping[source])
	err = row.Scan(&bookmark.BookmarkId)
	if err != nil {
		return nil, fmt.Errorf("bookmark create: %w", err)
	}

	return &bookmark, nil
}

func (service *BookmarkModel) ById(id types.BookmarkId) (*Bookmark, error) {
	bookmark := Bookmark{
		BookmarkId: id,
	}
	row := service.Pool.QueryRow(context.Background(),
		`SELECT user_id, title, link FROM bookmarks WHERE bookmark_id = $1;`, id)
	err := row.Scan(&bookmark.UserId, &bookmark.Title, &bookmark.Link)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("bookmark by id: %w", err)
	}
	return &bookmark, nil
}

func (service *BookmarkModel) ByUserId(userId types.UserId) ([]Bookmark, error) {
	rows, err := service.Pool.Query(context.Background(),
		`SELECT bookmark_id, title, link, excerpt, created_at FROM bookmarks WHERE user_id = $1;`, userId)
	if err != nil {
		return nil, fmt.Errorf("query bookmark by user id: %w", err)
	}
	defer rows.Close()
	// TODO: Get all the row elements
	var bookmarks []Bookmark
	// Iterate through the result set
	for rows.Next() {
		var bookmark Bookmark
		err := rows.Scan(&bookmark.BookmarkId, &bookmark.Title, &bookmark.Link, &bookmark.Excerpt, &bookmark.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		bookmarks = append(bookmarks, bookmark)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating rows: %w", rows.Err())
	}

	return bookmarks, nil
}

func (service *BookmarkModel) Update(bookmark *Bookmark) error {
	_, err := service.Pool.Exec(context.Background(),
		`UPDATE bookmarks SET link = $1, title = $2 WHERE bookmark_id = $3`,
		bookmark.Link, bookmark.Title, bookmark.BookmarkId,
	)
	if err != nil {
		return fmt.Errorf("update bookmark: %w", err)
	}
	return nil
}

func (service *BookmarkModel) Delete(id types.BookmarkId) error {
	_, err := service.Pool.Exec(context.Background(),
		`DELETE FROM bookmarks WHERE bookmark_id = $1;`, id)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	return nil
}

type SearchResult struct {
	Headline   string
	BookmarkId types.BookmarkId
	Title      string
	Link       string
	Excerpt    string
	ImageUrl   string
	Rank       float32
}

func (service *BookmarkModel) Search(userId types.UserId, query string) ([]SearchResult, error) {
	rows, err := service.Pool.Query(context.Background(), `
	WITH search_query AS (
		SELECT plainto_tsquery(CASE WHEN $1 = '' THEN '' ELSE $1 END) AS query
	)
	SELECT
		ts_headline(content, search_query.query, 'MaxFragments=2, StartSel=<strong>, StopSel=</strong>') AS excerpt,
		bookmark_id,
		title,
		link,
		excerpt,
		image_url,
		ts_rank(search_vector, search_query.query) AS rank
	FROM bookmarks, search_query
	WHERE user_id = $2
		AND search_vector @@ search_query.query
	ORDER BY rank DESC
	LIMIT 10
	`, query, userId)

	if err != nil {
		return nil, fmt.Errorf("search bookmarks: %w", err)
	}

	defer rows.Close()
	var results []SearchResult
	// Iterate through the result set
	for rows.Next() {
		var result SearchResult
		err := rows.Scan(&result.Headline, &result.BookmarkId, &result.Title, &result.Link, &result.Excerpt, &result.ImageUrl, &result.Rank)
		if err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		results = append(results, result)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterating rows: %w", rows.Err())
	}
	return results, nil
}
