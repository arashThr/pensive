package models

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/types"
	"github.com/arashthr/go-course/internal/validations"
	"github.com/go-shiori/go-readability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BookmarkSource = int

const PageSize = 2

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
	Source        string
	Excerpt       string
	ImageUrl      string
	ArticleLang   string
	SiteName      string
	CreatedAt     time.Time
	PublishedTime *time.Time
}

type BookmarkModel struct {
	Pool *pgxpool.Pool
}

// TODO: Add validation of the db query inputs (Like Id)
func (service *BookmarkModel) Create(link string, userId types.UserId, source BookmarkSource) (*Bookmark, error) {
	// Check if the link already exists
	bookmark, err := service.GetByLink(userId, link)
	if err != nil {
		if !errors.Is(err, errors.ErrNotFound) {
			return nil, fmt.Errorf("failed to collect row: %w", err)
		}
	} else {
		return bookmark, nil
	}

	parsedURL, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	resp, err := getPage(link)
	if err != nil {
		return nil, fmt.Errorf("failed to get page: %w", err)
	}
	defer resp.Body.Close()

	article, err := readability.FromReader(resp.Body, parsedURL)
	// TODO: Check for the language
	if err != nil {
		return nil, fmt.Errorf("readability: %w", err)
	}

	bookmarkId := strings.ToLower(rand.Text())[:8]
	bookmark = &Bookmark{
		BookmarkId:    types.BookmarkId(bookmarkId),
		UserId:        userId,
		Title:         validations.CleanUpText(article.Title),
		Link:          link,
		Excerpt:       validations.CleanUpText(article.Excerpt),
		ImageUrl:      article.Image,
		PublishedTime: article.PublishedTime,
		ArticleLang:   article.Language,
		SiteName:      article.SiteName,
		Source:        sourceMapping[source],
	}

	if _, err := url.ParseRequestURI(article.Image); err != nil {
		slog.Warn("Failed to parse image URL", "error", err)
		bookmark.ImageUrl = ""
	}
	// TODO: It's not working as expected and escapes the HTML
	content := validations.CleanUpText(article.TextContent)

	// TODO: Add excerpt to bookmarks_content table
	_, err = service.Pool.Exec(context.Background(), `
		WITH inserted_bookmark AS (
			INSERT INTO users_bookmarks (
				bookmark_id, 
				user_id, 
				link, 
				title,
				source,
				excerpt, 
				image_url, 
				article_lang, 
				site_name, 
				published_time
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		)
		INSERT INTO bookmarks_contents (bookmark_id, title, excerpt, content)
		VALUES ($1, $4, $6, $11);`,
		bookmarkId, userId, link, article.Title, sourceMapping[source], bookmark.Excerpt,
		article.Image, article.Language, article.SiteName, article.PublishedTime, content)
	if err != nil {
		return nil, fmt.Errorf("bookmark create: %w", err)
	}

	return bookmark, nil
}

func (service *BookmarkModel) GetById(id types.BookmarkId) (*Bookmark, error) {
	bookmark := Bookmark{
		BookmarkId: id,
	}
	row := service.Pool.QueryRow(context.Background(),
		`SELECT user_id, title, link, excerpt, image_url, created_at FROM users_bookmarks WHERE bookmark_id = $1;`, id)
	err := row.Scan(&bookmark.UserId, &bookmark.Title, &bookmark.Link, &bookmark.Excerpt, &bookmark.ImageUrl, &bookmark.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("bookmark by id: %w", err)
	}
	return &bookmark, nil
}

func (service *BookmarkModel) GetByUserId(userId types.UserId, page int) ([]Bookmark, bool, error) {
	row := service.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM users_bookmarks WHERE user_id = $1`, userId)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return nil, false, fmt.Errorf("count bookmarks to get all by user ID: %w", err)
	}
	if count == 0 {
		return []Bookmark{}, false, nil
	}

	if page <= 0 || page >= 100 {
		return nil, false, fmt.Errorf("page number out of range")
	}
	page -= 1
	rows, err := service.Pool.Query(context.Background(),
		`SELECT bookmark_id, title, link, excerpt, created_at
		FROM users_bookmarks
		WHERE user_id = $1
		LIMIT $2
		OFFSET $3
		`, userId, PageSize, page*PageSize)
	if err != nil {
		return nil, false, fmt.Errorf("query bookmark by user id: %w", err)
	}
	defer rows.Close()
	// TODO: Get all the row elements
	var bookmarks []Bookmark
	// Iterate through the result set
	for rows.Next() {
		var bookmark Bookmark
		err := rows.Scan(&bookmark.BookmarkId, &bookmark.Title, &bookmark.Link, &bookmark.Excerpt, &bookmark.CreatedAt)
		if err != nil {
			return nil, false, fmt.Errorf("scan bookmark: %w", err)
		}
		bookmarks = append(bookmarks, bookmark)
	}
	if rows.Err() != nil {
		return nil, false, fmt.Errorf("iterating rows: %w", rows.Err())
	}
	morePages := PageSize+page*PageSize < count

	return bookmarks, morePages, nil
}

func (service *BookmarkModel) GetByLink(userId types.UserId, link string) (*Bookmark, error) {
	rows, err := service.Pool.Query(context.Background(),
		`SELECT *
		FROM users_bookmarks
		WHERE user_id = $1 AND link = $2`, userId, link)
	if err != nil {
		return nil, fmt.Errorf("query bookmark by link: %w", err)
	}
	bookmark, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Bookmark])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("bookmark by link: %w", err)
	}
	return &bookmark, nil
}

func (service *BookmarkModel) Update(bookmark *Bookmark) error {
	_, err := service.Pool.Exec(context.Background(),
		`UPDATE users_bookmarks SET link = $1, title = $2 WHERE bookmark_id = $3`,
		bookmark.Link, bookmark.Title, bookmark.BookmarkId,
	)
	if err != nil {
		return fmt.Errorf("update bookmark: %w", err)
	}
	return nil
}

func (service *BookmarkModel) Delete(id types.BookmarkId) error {
	_, err := service.Pool.Exec(context.Background(),
		`DELETE FROM users_bookmarks WHERE bookmark_id = $1;`, id)
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
		ts_headline(content, sq.query, 'MaxFragments=2, StartSel=<strong>, StopSel=</strong>') AS excerpt,
		ub.bookmark_id,
		ub.title,
		link,
		ub.excerpt,
		image_url,
		ts_rank(search_vector, sq.query) AS rank
	FROM users_bookmarks ub
	JOIN bookmarks_contents bc ON ub.bookmark_id = bc.bookmark_id
	CROSS JOIN search_query sq
	WHERE ub.user_id = $2
    	AND bc.search_vector @@ sq.query
	ORDER BY rank DESC
	LIMIT 10`, query, userId)

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

func getPage(link string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return resp, nil
}
