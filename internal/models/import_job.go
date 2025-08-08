package models

import (
	"context"
	"fmt"
	"time"

	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ImportJobStatus string

const (
	ImportJobStatusPending    ImportJobStatus = "pending"
	ImportJobStatusProcessing ImportJobStatus = "processing"
	ImportJobStatusCompleted  ImportJobStatus = "completed"
	ImportJobStatusFailed     ImportJobStatus = "failed"
)

type ImportJob struct {
	ID            types.ImportJobId `db:"id"`
	UserID        types.UserId      `db:"user_id"`
	Source        string            `db:"source"`
	FilePath      string            `db:"file_path"`
	Status        ImportJobStatus   `db:"status"`
	TotalItems    int               `db:"total_items"`
	ImportedCount int               `db:"imported_count"`
	ErrorMessage  *string           `db:"error_message,omitempty"`
	CreatedAt     time.Time         `db:"created_at"`
	StartedAt     *time.Time        `db:"started_at,omitempty"`
	CompletedAt   *time.Time        `db:"completed_at,omitempty"`
}

type ImportJobModel struct {
	Pool *pgxpool.Pool
}

// Create creates a new import job
func (m *ImportJobModel) Create(job ImportJob) (*ImportJob, error) {
	query := `
		INSERT INTO import_jobs (user_id, source, file_path, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	err := m.Pool.QueryRow(context.Background(), query,
		job.UserID, job.Source, job.FilePath, ImportJobStatusPending).
		Scan(&job.ID, &job.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert import job: %w", err)
	}

	job.Status = ImportJobStatusPending
	return &job, nil
}

// GetPendingJobs returns up to limit pending jobs
func (m *ImportJobModel) GetPendingJobs(limit int) ([]types.ImportJobId, error) {
	query := `
		SELECT id
		FROM import_jobs 
		WHERE status = $1
		ORDER BY created_at ASC 
		LIMIT $2`

	rows, err := m.Pool.Query(context.Background(), query, ImportJobStatusPending, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs, err := pgx.CollectRows(rows, pgx.RowTo[types.ImportJobId])
	if err != nil {
		return nil, fmt.Errorf("collect jobs rows: %w", err)
	}

	return jobs, rows.Err()
}

// PickupJob returns a job by ID
func (m *ImportJobModel) GetByID(jobID types.ImportJobId) (*ImportJob, error) {
	query := `
		SELECT *
		FROM import_jobs 
		WHERE id = $1`

	rows, err := m.Pool.Query(context.Background(), query, jobID)
	if err != nil {
		return nil, fmt.Errorf("get job by id: %w", err)
	}

	job, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[ImportJob])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("collect one job row: %w", err)
	}

	return &job, nil
}

func PickupJob(tx pgx.Tx, jobID types.ImportJobId) (*ImportJob, error) {
	rows, err := tx.Query(context.Background(), `
		SELECT *
		FROM import_jobs
		WHERE id = $1 AND status != $2
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED`, jobID, ImportJobStatusCompleted)

	if err != nil {
		return nil, fmt.Errorf("query rows for pickup job: %w", err)
	}

	job, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[ImportJob])
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("collect rows for pickup job: %w", err)
	}

	return &job, nil
}

// UpdateStatus updates job status and timestamps
func UpdateStatus(tx pgx.Tx, jobID types.ImportJobId, status ImportJobStatus, errorMessage *string) error {
	var query string
	var args []any

	switch status {
	case ImportJobStatusProcessing:
		query = `UPDATE import_jobs SET status = $1, started_at = NOW() WHERE id = $2`
		args = []any{status, jobID}
	case ImportJobStatusCompleted:
		query = `UPDATE import_jobs SET status = $1, completed_at = NOW() WHERE id = $2`
		args = []any{status, jobID}
	case ImportJobStatusFailed:
		query = `UPDATE import_jobs SET status = $1, completed_at = NOW(), error_message = $2 WHERE id = $3`
		args = []any{status, *errorMessage, jobID}
	default:
		query = `UPDATE import_jobs SET status = $1 WHERE id = $2`
		args = []any{status, jobID}
	}

	_, err := tx.Exec(context.Background(), query, args...)
	return err
}

// UpdateProgress updates job progress counters
func UpdateProgress(tx pgx.Tx, jobID types.ImportJobId, totalItems, importedCount int) error {
	query := `
		UPDATE import_jobs 
		SET total_items = $1, imported_count = $2
		WHERE id = $3`

	_, err := tx.Exec(context.Background(), query, totalItems, importedCount, jobID)
	return err
}

// GetByUserID returns jobs for a specific user
func (m *ImportJobModel) GetByUserID(userID types.UserId, limit int) ([]ImportJob, error) {
	query := `
		SELECT id, user_id, source, file_path, status, 
		       total_items, imported_count, error_message, 
		       created_at, started_at, completed_at
		FROM import_jobs 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2`

	rows, err := m.Pool.Query(context.Background(), query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByName[ImportJob])
	if err != nil {
		return nil, fmt.Errorf("collect jobs rows for get by user id: %w", err)
	}

	return jobs, rows.Err()
}
