package service

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/service/importer"
	"github.com/arashthr/go-course/internal/types"
	"github.com/arashthr/go-course/web"
)

type Importer struct {
	Templates struct {
		PocketImport     web.Template
		ImportProcessing web.Template
		ImportStatus     web.Template
	}
	ImportJobModel *models.ImportJobModel
	BookmarkModel  *models.BookmarkModel
}

// PocketImport displays the import/export page
func (p Importer) PocketImport(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title string
	}
	data.Title = "Import from Pocket"
	p.Templates.PocketImport.Execute(w, r, data)
}

// ProcessImport handles the file upload and creates an import job
func (p Importer) ProcessImport(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())

	// Parse multipart form (2MB max size)
	err := r.ParseMultipartForm(2 << 20)
	if err != nil {
		logger.Error("parse multipart form", "error", err)
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("import-file")
	if err != nil {
		logger.Error("get form file", "error", err)
		http.Error(w, "Failed to get uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		http.Error(w, "Only ZIP files are supported", http.StatusBadRequest)
		return
	}

	// Get form values
	source := r.FormValue("source")

	// Basic validation
	if source != "pocket" {
		http.Error(w, "Unsupported import source", http.StatusBadRequest)
		return
	}

	// Create permanent file with unique name
	filePath, err := importer.GetImportFilePath(user.ID, source)
	if err != nil {
		logger.Error("create upload directory", "error", err)
		http.Error(w, "Failed to process upload", http.StatusInternalServerError)
		return
	}

	permanentFile, err := os.Create(filePath)
	if err != nil {
		logger.Error("create permanent file", "error", err)
		http.Error(w, "Failed to process upload", http.StatusInternalServerError)
		return
	}
	defer permanentFile.Close()

	// Copy uploaded file to permanent location
	_, err = io.Copy(permanentFile, file)
	if err != nil {
		logger.Error("copy file to permanent location", "error", err)
		os.Remove(filePath) // Clean up on error
		http.Error(w, "Failed to process upload", http.StatusInternalServerError)
		return
	}

	// Basic ZIP validation - try to open it
	_, err = zip.OpenReader(filePath)
	if err != nil {
		logger.Error("validate zip file", "error", err)
		os.Remove(filePath) // Clean up invalid file
		http.Error(w, "Invalid ZIP file", http.StatusBadRequest)
		return
	}

	// Create import job
	job := models.ImportJob{
		UserID:   user.ID,
		Source:   source,
		FilePath: filePath,
	}

	createdJob, err := p.ImportJobModel.Create(job)
	if err != nil {
		logger.Error("create import job", "error", err)
		os.Remove(filePath) // Clean up on error
		http.Error(w, "Failed to create import job", http.StatusInternalServerError)
		return
	}

	logger.Info("import job created",
		"job_id", createdJob.ID,
		"user_id", user.ID,
		"source", source,
		"filename", header.Filename,
		"size", header.Size)

	// Redirect to processing page
	data := struct {
		Title  string
		Source string
		JobID  string
	}{
		Title:  "Import Processing",
		Source: strings.ToUpper(source[:1]) + source[1:], // Capitalize first letter
		JobID:  string(createdJob.ID),
	}

	p.Templates.ImportProcessing.Execute(w, r, data)
}

// ProcessExport handles exporting user data
func (p Importer) ProcessExport(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())

	logger.Info("export requested", "user_id", user.ID)

	// Get all bookmarks for the user
	bookmarks, err := p.getAllBookmarksForUser(user.ID)
	if err != nil {
		logger.Error("get all bookmarks for export", "error", err)
		http.Error(w, "Failed to retrieve bookmarks", http.StatusInternalServerError)
		return
	}

	if len(bookmarks) == 0 {
		http.Error(w, "No bookmarks found to export", http.StatusNotFound)
		return
	}

	// Create temporary directory for export files
	tempDir, err := os.MkdirTemp("", "export_*")
	if err != nil {
		logger.Error("create temp directory", "error", err)
		http.Error(w, "Failed to create export", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// Create CSV file
	csvPath := filepath.Join(tempDir, "part_000000.csv")
	err = p.createCSV(csvPath, bookmarks)
	if err != nil {
		logger.Error("create CSV file", "error", err)
		http.Error(w, "Failed to create export file", http.StatusInternalServerError)
		return
	}

	// Create ZIP file
	zipPath := filepath.Join(tempDir, "bookmarks_export.zip")
	err = p.createZip(zipPath, csvPath)
	if err != nil {
		logger.Error("create ZIP file", "error", err)
		http.Error(w, "Failed to create export archive", http.StatusInternalServerError)
		return
	}

	// Serve the ZIP file
	file, err := os.Open(zipPath)
	if err != nil {
		logger.Error("open export file", "error", err)
		http.Error(w, "Failed to open export file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info for Content-Length
	fileInfo, err := file.Stat()
	if err != nil {
		logger.Error("get file info", "error", err)
		http.Error(w, "Failed to get export file info", http.StatusInternalServerError)
		return
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("bookmarks_export_%s.zip", timestamp)

	// Set headers for file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))

	// Copy file to response
	_, err = io.Copy(w, file)
	if err != nil {
		logger.Error("copy export file to response", "error", err)
		return
	}

	logger.Info("export completed successfully", "user_id", user.ID, "bookmark_count", len(bookmarks), "file_size", fileInfo.Size())
}

// getAllBookmarksForUser retrieves all bookmarks for a user by paginating through all pages
// TODO: Improve so it gets all the bookmarks in one go
func (p Importer) getAllBookmarksForUser(userID types.UserId) ([]models.Bookmark, error) {
	var allBookmarks []models.Bookmark
	page := 1

	for {
		bookmarks, morePages, err := p.BookmarkModel.GetByUserId(userID, page)
		if err != nil {
			return nil, fmt.Errorf("get bookmarks page %d: %w", page, err)
		}

		allBookmarks = append(allBookmarks, bookmarks...)

		if !morePages {
			break
		}

		page++
	}

	return allBookmarks, nil
}

// createCSV creates a CSV file in the format expected by the Pocket importer
func (p Importer) createCSV(csvPath string, bookmarks []models.Bookmark) error {
	file, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header matching Pocket import format
	header := []string{"url", "title", "time_added", "status"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	// Write bookmark records
	for _, bookmark := range bookmarks {
		record := []string{
			bookmark.Link,
			bookmark.Title,
			strconv.FormatInt(bookmark.CreatedAt.Unix(), 10), // Unix timestamp
			"archive", // Status - using "archive" so it gets imported
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write CSV record: %w", err)
		}
	}

	return nil
}

// createZip creates a ZIP file containing the CSV
func (p Importer) createZip(zipPath, csvPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create ZIP file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add CSV file to ZIP
	csvFile, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("open CSV file: %w", err)
	}
	defer csvFile.Close()

	csvInfo, err := csvFile.Stat()
	if err != nil {
		return fmt.Errorf("get CSV file info: %w", err)
	}

	// Create file header
	header, err := zip.FileInfoHeader(csvInfo)
	if err != nil {
		return fmt.Errorf("create file header: %w", err)
	}
	header.Name = "part_000000.csv" // Exact name expected by importer
	header.Method = zip.Deflate

	// Add file to ZIP
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create ZIP entry: %w", err)
	}

	_, err = io.Copy(writer, csvFile)
	if err != nil {
		return fmt.Errorf("copy CSV to ZIP: %w", err)
	}

	return nil
}

// ImportStatus checks the status of an import job
func (p Importer) ImportStatus(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())
	jobID := types.ImportJobId(r.URL.Query().Get("job_id"))

	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	// Get job from database
	job, err := p.ImportJobModel.GetByID(jobID)
	if err != nil {
		logger.Error("get import job", "error", err, "job_id", jobID)
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Verify job belongs to the current user
	if job.UserID != user.ID {
		logger.Warn("unauthorized job access", "job_id", jobID, "job_user_id", job.UserID, "current_user_id", user.ID)
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	logger.Info("import status check", "user_id", user.ID, "job_id", jobID, "status", job.Status)

	data := struct {
		Title          string
		JobID          string
		Source         string
		ImportComplete bool
		ImportFailed   bool
		ImportedCount  int
		TotalItems     int
		ErrorMessage   *string
	}{
		Title:          "Import Status",
		JobID:          string(job.ID),
		Source:         strings.ToUpper(job.Source[:1]) + job.Source[1:], // Capitalize first letter
		ImportComplete: job.Status == "completed",
		ImportFailed:   job.Status == "failed",
		ImportedCount:  job.ImportedCount,
		TotalItems:     job.TotalItems,
		ErrorMessage:   job.ErrorMessage,
	}

	p.Templates.ImportStatus.Execute(w, r, data)
}
