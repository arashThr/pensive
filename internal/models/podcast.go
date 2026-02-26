package models

import (
	"context"
	"fmt"
	"time"

	"github.com/arashthr/pensive/internal/errors"
	"github.com/arashthr/pensive/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PodcastScheduleStatus string

const (
	PodcastScheduleStatusPending    PodcastScheduleStatus = "pending"
	PodcastScheduleStatusProcessing PodcastScheduleStatus = "processing"
	PodcastScheduleStatusSent       PodcastScheduleStatus = "sent"
	PodcastScheduleStatusFailed     PodcastScheduleStatus = "failed"
	PodcastScheduleStatusTimedOut   PodcastScheduleStatus = "timed_out"
)

// PodcastScheduleMaxAttempts is the default maximum number of generation attempts before
// a schedule is marked as failed.
const PodcastScheduleMaxAttempts = 3

// PodcastProcessingTimeout is how long a schedule may sit in 'processing' before the
// scheduler considers it timed out and frees it for a retry.
const PodcastProcessingTimeout = 2 * time.Hour

type PodcastSchedule struct {
	ID              int                   `db:"id"`
	UserID          types.UserId          `db:"user_id"`
	NextPublishAt   time.Time             `db:"next_publish_at"`
	Status          PodcastScheduleStatus `db:"status"`
	Attempts        int                   `db:"attempts"`
	MaxAttempts     int                   `db:"max_attempts"`
	LastAttemptedAt *time.Time            `db:"last_attempted_at"`
	CreatedAt       time.Time             `db:"created_at"`
	UpdatedAt       time.Time             `db:"updated_at"`
}

type PodcastScheduleRepo struct {
	Pool *pgxpool.Pool
}

// Upsert creates or updates the schedule for a user.
// If the user disables their weekly podcast (enabled=false) it removes the schedule.
func (r *PodcastScheduleRepo) Upsert(userID types.UserId, nextPublishAt time.Time) error {
	_, err := r.Pool.Exec(context.Background(), `
		INSERT INTO podcast_schedules (user_id, next_publish_at, status, attempts, updated_at)
		VALUES ($1, $2, 'pending', 0, NOW())
		ON CONFLICT (user_id) DO UPDATE
		    SET next_publish_at = EXCLUDED.next_publish_at,
		        status          = 'pending',
		        attempts        = 0,
		        updated_at      = NOW()
		    WHERE podcast_schedules.status NOT IN ('processing')
	`, userID, nextPublishAt)
	if err != nil {
		return fmt.Errorf("upsert podcast schedule: %w", err)
	}
	return nil
}

// Delete removes the podcast schedule for a user (e.g. when they disable weekly podcasts).
func (r *PodcastScheduleRepo) Delete(userID types.UserId) error {
	_, err := r.Pool.Exec(context.Background(), `
		DELETE FROM podcast_schedules WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("delete podcast schedule: %w", err)
	}
	return nil
}

// GetDue returns all schedules that are past their publish time, are in a
// retriable state ('pending' or 'timed_out'), and have not yet exhausted
// their attempt budget.
func (r *PodcastScheduleRepo) GetDue() ([]PodcastSchedule, error) {
	rows, err := r.Pool.Query(context.Background(), `
		SELECT id, user_id, next_publish_at, status, attempts, max_attempts,
		       last_attempted_at, created_at, updated_at
		FROM podcast_schedules
		WHERE next_publish_at <= NOW()
		  AND status IN ('pending', 'timed_out')
		  AND attempts < max_attempts
		ORDER BY next_publish_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("get due podcast schedules: %w", err)
	}
	defer rows.Close()

	var schedules []PodcastSchedule
	for rows.Next() {
		var s PodcastSchedule
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.NextPublishAt, &s.Status, &s.Attempts,
			&s.MaxAttempts, &s.LastAttemptedAt, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan podcast schedule: %w", err)
		}
		schedules = append(schedules, s)
	}
	return schedules, rows.Err()
}

// MarkProcessing atomically claims a schedule for processing.
// Returns ErrNotFound if the schedule no longer exists or was already claimed.
func (r *PodcastScheduleRepo) MarkProcessing(id int) error {
	tag, err := r.Pool.Exec(context.Background(), `
		UPDATE podcast_schedules
		SET status            = 'processing',
		    attempts          = attempts + 1,
		    last_attempted_at = NOW(),
		    updated_at        = NOW()
		WHERE id = $1
		  AND status IN ('pending', 'timed_out')
		  AND attempts < max_attempts
	`, id)
	if err != nil {
		return fmt.Errorf("mark podcast schedule processing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.ErrNotFound
	}
	return nil
}

// MarkSent marks a schedule as sent and schedules the next episode.
func (r *PodcastScheduleRepo) MarkSent(id int, nextPublishAt time.Time) error {
	_, err := r.Pool.Exec(context.Background(), `
		UPDATE podcast_schedules
		SET status           = 'pending',
		    next_publish_at  = $2,
		    attempts         = 0,
		    updated_at       = NOW()
		WHERE id = $1
	`, id, nextPublishAt)
	if err != nil {
		return fmt.Errorf("mark podcast schedule sent: %w", err)
	}
	return nil
}

// MarkFailed increments the failure counter. If max_attempts is reached the
// status is set to 'failed', otherwise it reverts to 'pending' so it will
// be retried on the next scheduler tick.
func (r *PodcastScheduleRepo) MarkFailed(id int) error {
	_, err := r.Pool.Exec(context.Background(), `
		UPDATE podcast_schedules
		SET status     = CASE
		                     WHEN attempts >= max_attempts THEN 'failed'::podcast_schedule_status
		                     ELSE 'pending'::podcast_schedule_status
		                 END,
		    updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark podcast schedule failed: %w", err)
	}
	return nil
}

// ReapTimedOut scans for schedules that have been stuck in 'processing' for
// longer than PodcastProcessingTimeout and marks them as 'timed_out' so they
// can be retried (subject to their attempt budget).
func (r *PodcastScheduleRepo) ReapTimedOut() (int64, error) {
	tag, err := r.Pool.Exec(context.Background(), `
		UPDATE podcast_schedules
		SET status     = CASE
		                     WHEN attempts >= max_attempts THEN 'failed'::podcast_schedule_status
		                     ELSE 'timed_out'::podcast_schedule_status
		                 END,
		    updated_at = NOW()
		WHERE status = 'processing'
		  AND last_attempted_at < NOW() - $1::interval
	`, PodcastProcessingTimeout.String())
	if err != nil {
		return 0, fmt.Errorf("reap timed-out podcast schedules: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetByUserID returns the current schedule for a user, or ErrNotFound.
func (r *PodcastScheduleRepo) GetByUserID(userID types.UserId) (*PodcastSchedule, error) {
	var s PodcastSchedule
	err := r.Pool.QueryRow(context.Background(), `
		SELECT id, user_id, next_publish_at, status, attempts, max_attempts,
		       last_attempted_at, created_at, updated_at
		FROM podcast_schedules
		WHERE user_id = $1
	`, userID).Scan(
		&s.ID, &s.UserID, &s.NextPublishAt, &s.Status, &s.Attempts,
		&s.MaxAttempts, &s.LastAttemptedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("get podcast schedule by user: %w", err)
	}
	return &s, nil
}
