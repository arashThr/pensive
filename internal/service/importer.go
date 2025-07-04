package service

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/service/importer"
	"github.com/arashthr/go-course/web"
)

type Importer struct {
	Templates struct {
		ImportExport     web.Template
		ImportProcessing web.Template
		ImportStatus     web.Template
	}
	ImportJobModel *models.ImportJobModel
}

// ImportExport displays the import/export page
func (p Importer) ImportExport(w http.ResponseWriter, r *http.Request) {
	p.Templates.ImportExport.Execute(w, r, nil)
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
		Source string
		JobID  string
	}{
		Source: strings.ToUpper(source[:1]) + source[1:], // Capitalize first letter
		JobID:  createdJob.ID,
	}

	p.Templates.ImportProcessing.Execute(w, r, data)
}

// ProcessExport handles exporting user data
func (p Importer) ProcessExport(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())

	logger.Info("export requested", "user_id", user.ID)

	// TODO: Implement actual export functionality
	// For now, just return an error indicating it's not implemented yet
	http.Error(w, "Export functionality coming soon", http.StatusNotImplemented)
}

// ImportStatus checks the status of an import job
func (p Importer) ImportStatus(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())
	jobID := r.URL.Query().Get("job_id")

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
		JobID          string
		Source         string
		ImportComplete bool
		ImportFailed   bool
		ImportedCount  int
		TotalItems     int
		ErrorMessage   *string
	}{
		JobID:          job.ID,
		Source:         strings.ToUpper(job.Source[:1]) + job.Source[1:], // Capitalize first letter
		ImportComplete: job.Status == "completed",
		ImportFailed:   job.Status == "failed",
		ImportedCount:  job.ImportedCount,
		TotalItems:     job.TotalItems,
		ErrorMessage:   job.ErrorMessage,
	}

	p.Templates.ImportStatus.Execute(w, r, data)
}
