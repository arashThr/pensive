package models

import (
	"context"
	"fmt"
	"time"

	"github.com/arashthr/go-course/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ImportJob struct {
	ID            string       `json:"id"`
	UserID        types.UserId `json:"user_id"`
	Source        string       `json:"source"`
	ImportOption  string       `json:"import_option"`
	FilePath      string       `json:"file_path"`
	Status        string       `json:"status"` // pending, processing, completed, failed
	TotalItems    int          `json:"total_items"`
	ImportedCount int          `json:"imported_count"`
	ErrorMessage  *string      `json:"error_message,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	StartedAt     *time.Time   `json:"started_at,omitempty"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
}

type ImportJobModel struct {
	DB *pgxpool.Pool
}

// Create creates a new import job
func (m *ImportJobModel) Create(job ImportJob) (*ImportJob, error) {
	query := `
		INSERT INTO import_jobs (user_id, source, import_option, file_path, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	err := m.DB.QueryRow(context.Background(), query,
		job.UserID, job.Source, job.ImportOption, job.FilePath, "pending").
		Scan(&job.ID, &job.CreatedAt)

	if err != nil {
		return nil, err
	}

	job.Status = "pending"
	return &job, nil
}

// GetPendingJobs returns up to limit pending jobs
func (m *ImportJobModel) GetPendingJobs(limit int) ([]ImportJob, error) {
	query := `
		SELECT id, user_id, source, import_option, file_path, status, 
		       total_items, imported_count, error_message, 
		       created_at, started_at, completed_at
		FROM import_jobs 
		WHERE status = 'pending' 
		ORDER BY created_at ASC 
		LIMIT $1`

	rows, err := m.DB.Query(context.Background(), query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByName[ImportJob])
	if err != nil {
		return nil, fmt.Errorf("collect jobs rows: %w", err)
	}

	return jobs, rows.Err()
}

// GetByID returns a job by ID
func (m *ImportJobModel) GetByID(jobID string) (*ImportJob, error) {
	query := `
		SELECT id, user_id, source, import_option, file_path, status, 
		       total_items, imported_count, error_message, 
		       created_at, started_at, completed_at
		FROM import_jobs 
		WHERE id = $1`

	var job ImportJob
	err := m.DB.QueryRow(context.Background(), query, jobID).Scan(
		&job.ID, &job.UserID, &job.Source, &job.ImportOption, &job.FilePath,
		&job.Status, &job.TotalItems, &job.ImportedCount,
		&job.ErrorMessage, &job.CreatedAt, &job.StartedAt, &job.CompletedAt,
	)

	if err != nil {
		return nil, err
	}

	return &job, nil
}

// UpdateStatus updates job status and timestamps
func (m *ImportJobModel) UpdateStatus(jobID, status string, errorMessage *string) error {
	var query string
	var args []interface{}

	switch status {
	case "processing":
		query = `UPDATE import_jobs SET status = $1, started_at = NOW() WHERE id = $2`
		args = []interface{}{status, jobID}
	case "completed":
		query = `UPDATE import_jobs SET status = $1, completed_at = NOW() WHERE id = $2`
		args = []interface{}{status, jobID}
	case "failed":
		query = `UPDATE import_jobs SET status = $1, completed_at = NOW(), error_message = $2 WHERE id = $3`
		args = []interface{}{status, *errorMessage, jobID}
	default:
		query = `UPDATE import_jobs SET status = $1 WHERE id = $2`
		args = []interface{}{status, jobID}
	}

	_, err := m.DB.Exec(context.Background(), query, args...)
	return err
}

// UpdateProgress updates job progress counters
func (m *ImportJobModel) UpdateProgress(jobID string, totalItems, importedCount int) error {
	query := `
		UPDATE import_jobs 
		SET total_items = $1, imported_count = $2
		WHERE id = $3`

	_, err := m.DB.Exec(context.Background(), query, totalItems, importedCount, jobID)
	return err
}

// GetByUserID returns jobs for a specific user
func (m *ImportJobModel) GetByUserID(userID types.UserId, limit int) ([]ImportJob, error) {
	query := `
		SELECT id, user_id, source, import_option, file_path, status, 
		       total_items, imported_count, error_message, 
		       created_at, started_at, completed_at
		FROM import_jobs 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2`

	rows, err := m.DB.Query(context.Background(), query, userID, limit)
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
