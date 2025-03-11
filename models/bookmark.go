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

type Bookmark struct {
	BookmarkId types.BookmarkId
	UserId     types.UserId
	Title      string
	Link       string
}

type BookmarkService struct {
	Pool *pgxpool.Pool
}

// TODO: Add validation of the db query inputs (Like Id)
func (service *BookmarkService) Create(link string, userId types.UserId) (*Bookmark, error) {
	// TODO: Check if the website exists
	article, err := readability.FromURL(link, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("readability: %w", err)
	}
	bookmark := Bookmark{
		UserId: userId,
		Title:  article.Title,
		Link:   link,
	}
	bookmarkId := strings.ToLower(rand.Text())[:8]

	row := service.Pool.QueryRow(context.Background(),
		`INSERT INTO bookmarks (bookmark_id, user_id, title, link)
		VALUES ($1, $2, $3, $4) RETURNING bookmark_id;`, bookmarkId, userId, bookmark.Title, link)
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
		`SELECT * FROM bookmarks WHERE user_id = $1;`, userId)
	if err != nil {
		return nil, fmt.Errorf("query bookmark by user id: %w", err)
	}
	bookmarks, err := pgx.CollectRows(rows, pgx.RowToStructByName[Bookmark])
	if err != nil {
		return nil, fmt.Errorf("collecting bookmark rows: %w", err)
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
	// TODO: Query Db
	// return []Bookmark{
	// 	{
	// 		BookmarkId: "mock1",
	// 		UserId:     userId,
	// 		Title:      "Mock Title 1",
	// 		Link:       "http://mocklink1.com",
	// 	},
	// 	{
	// 		BookmarkId: "mock2",
	// 		UserId:     userId,
	// 		Title:      "Mock Title 2",
	// 		Link:       "http://mocklink2.com",
	// 	},
	// }, nil

	rows, err := service.Pool.Query(context.Background(), `
		SELECT ts_headline(content, query,
		  'MaxFragments=2, StartSel=<strong>, StopSel=</strong>') as excerpt, url, title
		FROM bookmarks, plainto_tsquery(:searchQuery) as query
		WHERE to_tsvector(title || ' ' || content) @@ plainto_tsquery($1)`, query)

	if err != nil {
		return nil, fmt.Errorf("search bookmarks: %w", err)
	}

	var bookmarks []Bookmark
	for rows.Next() {
		var bookmark Bookmark
		err := rows.Scan(&bookmark.Title, &bookmark.Link)
		if err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		bookmarks = append(bookmarks, bookmark)
	}
	return bookmarks, nil
}
