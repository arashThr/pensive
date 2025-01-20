package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/arashthr/go-course/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Bookmark struct {
	ID     uint
	UserId uint
	Title  string
	Link   string
}

type BookmarkService struct {
	Pool *pgxpool.Pool
}

// TODO: Add validation of the db query inputs (Like Id)

func (service *BookmarkService) Create(title string, link string, userId uint) (*Bookmark, error) {
	bookmark := Bookmark{
		UserId: userId,
		Title:  title,
		Link:   link,
	}

	row := service.Pool.QueryRow(context.Background(),
		`INSERT INTO bookmarks (user_id, title, link)
		VALUES ($1, $2, $3) RETURNING id;`, userId, title, link)
	err := row.Scan(&bookmark.ID)
	if err != nil {
		return nil, fmt.Errorf("bookmark create: %w", err)
	}
	return &bookmark, nil
}

func (service *BookmarkService) ById(id uint) (*Bookmark, error) {
	bookmark := Bookmark{
		ID: id,
	}
	row := service.Pool.QueryRow(context.Background(),
		`SELECT user_id, title, link FROM bookmarks WHERE id = $1;`, id)
	err := row.Scan(&bookmark.UserId, &bookmark.Title, &bookmark.Link)
	if err != nil {
		// TODO: Test to make sure this works
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("bookmark by id: %w", err)
	}
	return &bookmark, nil
}

func (service *BookmarkService) ByUserId(userId uint) ([]Bookmark, error) {
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

func (service *BookmarkService) Delete(id uint) error {
	_, err := service.Pool.Exec(context.Background(),
		`DELETE FROM bookmarks WHERE id = $1;`, id)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	return nil
}
