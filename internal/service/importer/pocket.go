package importer

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/types"
	"github.com/arashthr/pensive/internal/validations"
	"github.com/jackc/pgx/v5"
)

const uploadDir = "uploads/imports"

type ImportProcessor struct {
	ImportJobModel *models.ImportJobRepo
	BookmarkModel  *models.BookmarkRepo
	UserModel      *models.UserRepo
}

type PocketItem struct {
	Title     string
	URL       string
	TimeAdded time.Time
	Status    string
}

// Start begins processing import jobs in a loop
func (p *ImportProcessor) Start(ctx context.Context) {
	logger := logging.Logger.With("flow", "importer")
	ctx = loggercontext.WithLogger(ctx, logger)
	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
	defer ticker.Stop()

	logger.Infow("import processor started")

	for {
		select {
		case <-ctx.Done():
			logger.Infow("import processor stopping")
			return
		case <-ticker.C:
			jobs, err := p.ImportJobModel.GetPendingJobs(3) // Process up to 3 jobs concurrently
			if err != nil {
				logger.Errorw("get pending jobs", "error", err)
				continue
			}

			for _, job := range jobs {
				go p.processJob(ctx, job) // Process concurrently
			}
		}
	}
}

// processJob processes a single import job
func (p *ImportProcessor) processJob(ctx context.Context, jobID types.ImportJobId) {
	logger := loggercontext.Logger(ctx)
	tx, err := p.ImportJobModel.Pool.Begin(context.Background())
	if err != nil {
		logger.Errorw("begin transaction", "error", err)
		return
	}
	defer tx.Rollback(context.Background())

	job, err := models.PickupJob(tx, jobID)
	if err != nil {
		logger.Errorw("get job by id", "error", err, "job_id", jobID)
		return
	}

	if job == nil {
		logger.Infow("no job to process", "job_id", jobID)
		return
	}

	logger.Infow("processing import job", "source", job.Source, "file", job.FilePath, "job_id", job.ID)

	// Update status to processing
	if err := models.UpdateStatus(tx, job.ID, models.ImportJobStatusProcessing, nil); err != nil {
		logger.Errorw("update job status to processing", "error", err)
		return
	}

	// Process based on source
	switch job.Source {
	case "pocket":
		err = p.processPocketImport(ctx, tx, job)
	default:
		err = fmt.Errorf("unsupported import source: %s", job.Source)
	}

	// Update final status
	if err != nil {
		logger.Errorw("job processing failed", "error", err)
		logging.Telegram.SendMessage(fmt.Sprintf("job id %v processing failed for user %v", jobID, job.UserID))
		errorMsg := err.Error()
		if updateErr := models.UpdateStatus(tx, job.ID, models.ImportJobStatusFailed, &errorMsg); updateErr != nil {
			logger.Errorw("update job status to failed", "error", updateErr)
		}
	} else {
		logger.Infow("job processing completed", "job_id", job.ID)
		if updateErr := models.UpdateStatus(tx, job.ID, models.ImportJobStatusCompleted, nil); updateErr != nil {
			logger.Errorw("update job status to completed", "error", updateErr)
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		logger.Errorw("commit transaction", "error", err)
	}
}

// processPocketImport processes a Pocket ZIP export
func (p *ImportProcessor) processPocketImport(ctx context.Context, tx pgx.Tx, job *models.ImportJob) error {
	logger := loggercontext.Logger(ctx)
	// Open ZIP file
	reader, err := zip.OpenReader(job.FilePath)
	if err != nil {
		return fmt.Errorf("open zip file: %w", err)
	}
	defer reader.Close()

	// Find the main CSV file (part_000000.csv)
	var csvFile *zip.File
	for _, file := range reader.File {
		if strings.HasSuffix(strings.ToLower(file.Name), "part_000000.csv") {
			csvFile = file
			break
		}
	}

	if csvFile == nil {
		return fmt.Errorf("part_000000.csv not found in ZIP file")
	}

	// Parse CSV file
	items, err := p.parsePocketCSV(ctx, csvFile)
	if err != nil {
		return fmt.Errorf("parse pocket CSV: %w", err)
	}
	logger.Infow("parsed pocket CSV", "items", len(items), "user_id", job.UserID)

	// Update total items count
	if err := models.UpdateProgress(tx, job.ID, len(items), 0); err != nil {
		logger.Errorw("update total items count", "error", err)
	}

	user, err := p.UserModel.Get(job.UserID)
	if err != nil {
		logger.Errorw("get user for import", "error", err, "user_id", job.UserID)
		return fmt.Errorf("get user for import: %w", err)
	}

	// Import bookmarks
	importedCount := 0
	for _, item := range items {
		// Validate URL
		if !validations.IsURLValid(item.URL) {
			logger.Debugw("skipping invalid URL", "url", item.URL)
		} else if item.Status == "archive" {
			// Do not apply the premium status to the import
			_, err := p.BookmarkModel.Create(ctx, item.URL, user, models.Pocket)
			if err != nil {
				logger.Errorw("create bookmark failed", "error", err, "url", item.URL)
			}
		}
		importedCount++
		if err := models.UpdateProgress(tx, job.ID, len(items), importedCount); err != nil {
			logger.Errorw("update progress", "error", err)
		}
	}

	logger.Infow("import completed", "imported_count", importedCount, "total_items", len(items))
	return nil
}

// parsePocketCSV parses the Pocket CSV file and returns items
func (p *ImportProcessor) parsePocketCSV(ctx context.Context, csvFile *zip.File) ([]PocketItem, error) {
	logger := loggercontext.Logger(ctx)
	rc, err := csvFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open CSV file: %w", err)
	}
	defer rc.Close()

	reader := csv.NewReader(rc)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Find column indices
	var urlIndex, titleIndex, timeAddedIndex, statusIndex int = -1, -1, -1, -1
	for i, col := range header {
		switch strings.ToLower(strings.TrimSpace(col)) {
		case "url":
			urlIndex = i
		case "title":
			titleIndex = i
		case "time_added":
			timeAddedIndex = i
		case "status":
			statusIndex = i
		}
	}

	if urlIndex == -1 || titleIndex == -1 || timeAddedIndex == -1 || statusIndex == -1 {
		return nil, fmt.Errorf("required columns not found in CSV (url, title, time_added, status)")
	}

	var items []PocketItem
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV record: %w", err)
		}

		if len(record) <= urlIndex || len(record) <= titleIndex || len(record) <= timeAddedIndex {
			continue // Skip malformed records
		}

		// Parse time_added (Unix timestamp)
		timeAddedStr := strings.TrimSpace(record[timeAddedIndex])
		timeAddedUnix, err := strconv.ParseInt(timeAddedStr, 10, 64)
		if err != nil {
			logger.Debugw("invalid time_added, using current time", "time_added", timeAddedStr)
			timeAddedUnix = time.Now().Unix()
		}

		item := PocketItem{
			Title:     strings.TrimSpace(record[titleIndex]),
			URL:       strings.TrimSpace(record[urlIndex]),
			TimeAdded: time.Unix(timeAddedUnix, 0),
			Status:    strings.TrimSpace(record[statusIndex]),
		}

		items = append(items, item)
	}

	return items, nil
}

func GetImportFilePath(userId types.UserId, source string) (string, error) {
	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("create upload directory: %w", err)
	}
	// TODO: use temp dir
	// uploadDir := os.TempDir()
	filename := fmt.Sprintf("%s_%s.zip", strconv.Itoa(int(userId)), source)
	filePath := filepath.Join(uploadDir, filename)
	return filePath, nil
}
