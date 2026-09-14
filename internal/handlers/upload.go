package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"poneglyph/internal/database"
	"poneglyph/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// MaxFileCount is the maximum number of files allowed in a single upload request.
	MaxFileCount = 10
	// MaxFileSize is the maximum size allowed for any single file (25 MB).
	MaxFileSize = 25 * 1024 * 1024 // 25 MB
	// MaxTotalRequestBytes limits the total request body (260 MB).
	MaxTotalRequestBytes = 260 * 1024 * 1024
)

// allowedExtensions defines the file extensions accepted for upload.
var allowedExtensions = map[string]string{
	// Images
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".bmp":  "image/bmp",
	// PDF
	".pdf": "application/pdf",
	// Word Documents
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".doc":  "application/msword",
}

// uploadRateLimiter prevents aggressive spamming.
var (
	uploadRateLimiter = make(map[string]time.Time)
	limiterMutex      sync.Mutex
	uploadRateLimit   = 1 * time.Second
)

// ResetUploadRateLimiter clears the upload rate limiter cache (used in testing).
func ResetUploadRateLimiter() {
	limiterMutex.Lock()
	defer limiterMutex.Unlock()
	uploadRateLimiter = make(map[string]time.Time)
}

func isAllowedFileExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := allowedExtensions[ext]
	return ok
}

// UploadBatch handles multi-file uploads (max 10 files, max 25MB each).
// It stores files in ./uploads/<document_id>/ and records the job in MongoDB.
func UploadBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Per-IP rate limiting (1 second cooldown)
	clientIP := r.RemoteAddr
	limiterMutex.Lock()
	lastUpload, exists := uploadRateLimiter[clientIP]
	if exists && time.Since(lastUpload) < uploadRateLimit {
		limiterMutex.Unlock()
		log.Printf("[JOB] REJECTED: Upload rate limit exceeded for %s", clientIP)
		http.Error(w, "Rate limit exceeded. Please wait a moment before uploading again.", http.StatusTooManyRequests)
		return
	}
	uploadRateLimiter[clientIP] = time.Now()
	limiterMutex.Unlock()

	log.Printf("[JOB] Incoming upload request from %s", clientIP)

	// Limit total body size to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, MaxTotalRequestBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Printf("[JOB] REJECTED: Multipart form error from %s: %v", clientIP, err)
		http.Error(w, "File too large or invalid multipart form data", http.StatusBadRequest)
		return
	}

	// Support form field names "files", "documents", or "file"
	fileHeaders := r.MultipartForm.File["files"]
	if len(fileHeaders) == 0 {
		fileHeaders = r.MultipartForm.File["documents"]
	}
	if len(fileHeaders) == 0 {
		fileHeaders = r.MultipartForm.File["file"]
	}
	if len(fileHeaders) == 0 {
		for _, headers := range r.MultipartForm.File {
			fileHeaders = append(fileHeaders, headers...)
		}
	}

	// Validation 1: At least 1 file
	if len(fileHeaders) == 0 {
		log.Printf("[JOB] REJECTED from %s: No files provided in request", clientIP)
		http.Error(w, "No files provided. Please select at least one file to upload.", http.StatusBadRequest)
		return
	}

	// Validation 2: Max 10 files
	if len(fileHeaders) > MaxFileCount {
		errMsg := fmt.Sprintf("Too many files: maximum allowed is %d files per batch, but received %d", MaxFileCount, len(fileHeaders))
		log.Printf("[JOB] REJECTED from %s: %s", clientIP, errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	// Validation 3 & 4: Check each file's size and extension before writing anything to disk
	for _, fh := range fileHeaders {
		if fh.Size > MaxFileSize {
			errMsg := fmt.Sprintf("File '%s' is too large (%0.2f MB). Maximum allowed size per file is 25 MB.", fh.Filename, float64(fh.Size)/(1024*1024))
			log.Printf("[JOB] REJECTED from %s: %s", clientIP, errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		if !isAllowedFileExtension(fh.Filename) {
			errMsg := fmt.Sprintf("File '%s' has an unsupported format. Allowed formats: Images (JPEG, PNG, WebP, GIF, TIFF, BMP), PDF, and Word documents (DOCX, DOC).", fh.Filename)
			log.Printf("[JOB] REJECTED from %s: %s", clientIP, errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
	}

	// Generate unique ObjectId for the job (primary key and storage folder name)
	objID := primitive.NewObjectID()
	jobID := objID.Hex()

	// Create job directory: ./uploads/<jobID>/
	uploadBaseDir := os.Getenv("UPLOAD_DIR")
	if uploadBaseDir == "" {
		uploadBaseDir = "./uploads"
	}
	jobDir := filepath.Join(uploadBaseDir, jobID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		log.Printf("[JOB %s] ERROR: Failed to create job directory %s: %v", jobID, jobDir, err)
		http.Error(w, "Failed to create storage directory", http.StatusInternalServerError)
		return
	}
	log.Printf("[JOB %s] Initialized directory: %s (processing %d files)", jobID, jobDir, len(fileHeaders))

	var savedFiles []models.JobFile
	var totalBytes int64

	// Save each file into ./uploads/<jobID>/
	for idx, fh := range fileHeaders {
		src, err := fh.Open()
		if err != nil {
			log.Printf("[JOB %s] ERROR: Failed to open uploaded file header '%s': %v", jobID, fh.Filename, err)
			http.Error(w, fmt.Sprintf("Failed to read file '%s'", fh.Filename), http.StatusBadRequest)
			return
		}

		cleanName := filepath.Base(fh.Filename)
		if cleanName == "" || cleanName == "." {
			cleanName = fmt.Sprintf("file_%d%s", idx+1, filepath.Ext(fh.Filename))
		}

		destPath := filepath.Join(jobDir, cleanName)
		dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			src.Close()
			log.Printf("[JOB %s] ERROR: Failed to create destination file '%s': %v", jobID, destPath, err)
			http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
			return
		}

		written, err := io.Copy(dst, src)
		src.Close()
		dst.Close()

		if err != nil {
			log.Printf("[JOB %s] ERROR: Failed to copy file contents to '%s': %v", jobID, destPath, err)
			http.Error(w, "Failed to write file to disk", http.StatusInternalServerError)
			return
		}

		ext := strings.ToLower(filepath.Ext(cleanName))
		mime := allowedExtensions[ext]
		totalBytes += written

		log.Printf("[JOB %s] Saved file [%d/%d]: '%s' (%0.2f KB, %s)", jobID, idx+1, len(fileHeaders), cleanName, float64(written)/1024, mime)

		savedFiles = append(savedFiles, models.JobFile{
			Filename: cleanName,
			Size:     written,
			MimeType: mime,
		})
	}

	job := models.Job{
		ID:         objID,
		UploadedAt: time.Now().UTC(),
		Files:      savedFiles,
	}

	// Persist job metadata in MongoDB
	if database.DB != nil {
		_, err := database.GetCollection("jobs").InsertOne(r.Context(), job)
		if err != nil {
			log.Printf("[JOB %s] ERROR: Failed to record job in MongoDB: %v", jobID, err)
		} else {
			log.Printf("[JOB %s] SUCCESS: Job registered in MongoDB (id=%s, files=%d, total_size=%0.2f MB)", jobID, jobID, len(savedFiles), float64(totalBytes)/(1024*1024))
		}
	} else {
		log.Printf("[JOB %s] SUCCESS: Job stored on disk (id=%s, files=%d, total_size=%0.2f MB)", jobID, jobID, len(savedFiles), float64(totalBytes)/(1024*1024))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

// UploadDocument maintains backward compatibility for callers expecting UploadDocument.
func UploadDocument(w http.ResponseWriter, r *http.Request) {
	UploadBatch(w, r)
}
