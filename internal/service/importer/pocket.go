package importer

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/types"
	"github.com/arashthr/go-course/internal/validations"
)

const uploadDir = "uploads/imports"

type ImportProcessor struct {
	ImportJobModel *models.ImportJobModel
	BookmarkModel  *models.BookmarkModel
	UserModel      *models.UserModel
	Logger         *slog.Logger
}

type PocketItem struct {
	Title     string
	URL       string
	TimeAdded time.Time
	Status    string
}

// Start begins processing import jobs in a loop
func (p *ImportProcessor) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
	defer ticker.Stop()

	p.Logger.Info("import processor started")

	for {
		select {
		case <-ctx.Done():
			p.Logger.Info("import processor stopping")
			return
		case <-ticker.C:
			jobs, err := p.ImportJobModel.GetPendingJobs(3) // Process up to 3 jobs concurrently
			if err != nil {
				p.Logger.Error("get pending jobs", "error", err)
				continue
			}

			for _, job := range jobs {
				go p.processJob(job) // Process concurrently
			}
		}
	}
}

// processJob processes a single import job
func (p *ImportProcessor) processJob(job models.ImportJob) {
	slog.Info("processing import job", "source", job.Source, "file", job.FilePath)

	// Update status to processing
	if err := p.ImportJobModel.UpdateStatus(job.ID, "processing", nil); err != nil {
		slog.Error("update job status to processing", "error", err)
		return
	}

	// Process based on source
	var err error
	switch job.Source {
	case "pocket":
		err = p.processPocketImport(job)
	default:
		err = fmt.Errorf("unsupported import source: %s", job.Source)
	}

	// Update final status
	if err != nil {
		slog.Error("job processing failed", "error", err)
		errorMsg := err.Error()
		if updateErr := p.ImportJobModel.UpdateStatus(job.ID, "failed", &errorMsg); updateErr != nil {
			slog.Error("update job status to failed", "error", updateErr)
		}
	} else {
		slog.Info("job processing completed")
		if updateErr := p.ImportJobModel.UpdateStatus(job.ID, "completed", nil); updateErr != nil {
			slog.Error("update job status to completed", "error", updateErr)
		}
	}
}

// processPocketImport processes a Pocket ZIP export
func (p *ImportProcessor) processPocketImport(job models.ImportJob) error {
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
	items, err := p.parsePocketCSV(csvFile)
	if err != nil {
		return fmt.Errorf("parse pocket CSV: %w", err)
	}

	// Update total items count
	if err := p.ImportJobModel.UpdateProgress(job.ID, len(items), 0); err != nil {
		slog.Error("update total items count", "error", err)
	}

	user, err := p.UserModel.Get(job.UserID)
	if err != nil {
		slog.Error("get user for import", "error", err, "user_id", job.UserID)
		return fmt.Errorf("get user for import: %w", err)
	}

	// Import bookmarks
	importedCount := 0
	for i, item := range items {
		// Validate URL
		if !validations.IsURLValid(item.URL) {
			slog.Debug("skipping invalid URL", "url", item.URL)
		} else if item.Status == "archive" {
			// Do not apply the premium status to the import
			_, err := p.BookmarkModel.Create(item.URL, user, models.Pocket)
			if err != nil {
				slog.Error("create bookmark failed", "error", err, "url", item.URL)
			}
		}
		importedCount++
		// Update progress every 10 items or at the end
		if (i+1)%10 == 0 || i == len(items)-1 {
			if err := p.ImportJobModel.UpdateProgress(job.ID, len(items), importedCount); err != nil {
				slog.Error("update progress", "error", err)
			}
		}
	}

	slog.Info("import completed", "imported_count", importedCount, "total_items", len(items))
	return nil
}

// parsePocketCSV parses the Pocket CSV file and returns items
func (p *ImportProcessor) parsePocketCSV(csvFile *zip.File) ([]PocketItem, error) {
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
			slog.Debug("invalid time_added, using current time", "time_added", timeAddedStr)
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
