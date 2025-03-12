package models

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/arashthr/go-course/errors"
	"github.com/arashthr/go-course/types"
	"github.com/go-shiori/go-readability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
}

type BookmarkService struct {
	Pool *pgxpool.Pool
}

// TODO: Add validation of the db query inputs (Like Id)
func (service *BookmarkService) Create(link string, userId types.UserId, source BookmarkSource) (*Bookmark, error) {
	// TODO: Check if the website exists
	article, err := readability.FromURL(link, 5*time.Second)
	// TODO: Check for the language
	if err != nil {
		return nil, fmt.Errorf("readability: %w", err)
	}
	fmt.Printf("%+v\n\n", article)
	bookmark := Bookmark{
		UserId: userId,
		Title:  article.Title,
		Link:   link,
	}
	bookmarkId := strings.ToLower(rand.Text())[:8]

	row := service.Pool.QueryRow(context.Background(),
		`INSERT INTO bookmarks (bookmark_id, user_id, title, link, content, excerpt, image_url, article_lang, site_name, published_time, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING bookmark_id;`,
		bookmarkId, userId, article.Title, link, article.TextContent,
		article.Excerpt, article.Image, article.Language, article.SiteName,
		article.PublishedTime, sourceMapping[source])
	err = row.Scan(&bookmark.BookmarkId)
	if err != nil {
		return nil, fmt.Errorf("bookmark create: %w", err)
	}

	return &bookmark, nil
}

func (service *BookmarkService) ById(id types.BookmarkId) (*Bookmark, error) {
	bookmark := Bookmark{
		BookmarkId: id,
	}
	row := service.Pool.QueryRow(context.Background(),
		`SELECT user_id, title, link FROM bookmarks WHERE bookmark_id = $1;`, id)
	err := row.Scan(&bookmark.UserId, &bookmark.Title, &bookmark.Link)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("bookmark by id: %w", err)
	}
	return &bookmark, nil
}

func (service *BookmarkService) ByUserId(userId types.UserId) ([]Bookmark, error) {
	rows, err := service.Pool.Query(context.Background(),
		`SELECT bookmark_id, title, link, excerpt FROM bookmarks WHERE user_id = $1;`, userId)
	if err != nil {
		return nil, fmt.Errorf("query bookmark by user id: %w", err)
	}
	defer rows.Close()
	var bookmarks []Bookmark
	// Iterate through the result set
	for rows.Next() {
		var bookmark Bookmark
		err := rows.Scan(&bookmark.BookmarkId, &bookmark.Title, &bookmark.Link, &bookmark.Excerpt)
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

func (service *BookmarkService) Update(bookmark *Bookmark) error {
	_, err := service.Pool.Exec(context.Background(),
		`UPDATE bookmarks SET link = $1, title = $2 WHERE bookmark_id = $3`,
		bookmark.Link, bookmark.Title, bookmark.BookmarkId,
	)
	if err != nil {
		return fmt.Errorf("update bookmark: %w", err)
	}
	return nil
}

func (service *BookmarkService) Delete(id types.BookmarkId) error {
	_, err := service.Pool.Exec(context.Background(),
		`DELETE FROM bookmarks WHERE bookmark_id = $1;`, id)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	return nil
}

func (service *BookmarkService) Search(userId types.UserId, query string) ([]Bookmark, error) {
	rows, err := service.Pool.Query(context.Background(), `
		SELECT 
		ts_headline(content, query, 'MaxFragments=2, StartSel=<strong>, StopSel=</strong>') as excerpt,
			url,
			title,
			ts_rank(search_vector, query) as rank
		FROM bookmarks,
			(SELECT CASE WHEN $1 = '' THEN plainto_tsquery('') ELSE plainto_tsquery($1) END) as query
		WHERE user_id = $2 AND
			query IS NOT NULL AND
			search_vector @@ query
		ORDER BY rank DESC
		LIMIT 10`, query, userId)

	if err != nil {
		return nil, fmt.Errorf("search bookmarks: %w", err)
	}

	bookmarks, err := pgx.CollectRows(rows, pgx.RowToStructByName[Bookmark])
	if err != nil {
		return nil, fmt.Errorf("collecting bookmark rows: %w", err)
	}
	return bookmarks, nil
}
