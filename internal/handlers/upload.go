package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"docunest/internal/database"
	"docunest/internal/services"
	"docunest/internal/storage"
)

// uploadRateLimiter is a per-IP token bucket.
// We store the last-upload time per IP, pruned periodically so it doesn't grow forever.
var (
	uploadRateLimiter = make(map[string]time.Time)
	limiterMutex      sync.Mutex
	uploadRateLimit   = 5 * time.Second
)

// pruneRateLimiter removes entries older than 1 minute to prevent unbounded memory growth.
func pruneRateLimiter() {
	limiterMutex.Lock()
	defer limiterMutex.Unlock()
	cutoff := time.Now().Add(-1 * time.Minute)
	for ip, t := range uploadRateLimiter {
		if t.Before(cutoff) {
			delete(uploadRateLimiter, ip)
		}
	}
}

func init() {
	// Prune the rate limiter map every 5 minutes to prevent memory leaks.
	go func() {
		for range time.Tick(5 * time.Minute) {
			pruneRateLimiter()
		}
	}()
}

const maxUploadBytes = 15 << 20 // 15 MB hard cap

func UploadDocument(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	_, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Per-IP rate limiting
	clientIP := r.RemoteAddr
	limiterMutex.Lock()
	lastUpload, exists := uploadRateLimiter[clientIP]
	if exists && time.Since(lastUpload) < uploadRateLimit {
		limiterMutex.Unlock()
		http.Error(w, "Rate limit exceeded. Please wait before uploading again.", http.StatusTooManyRequests)
		return
	}
	uploadRateLimiter[clientIP] = time.Now()
	limiterMutex.Unlock()

	// Hard limit on request body size before parsing
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large or invalid form data", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("document")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// MIME Validation: read first 512 bytes to determine real content type
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	buffer = buffer[:n]
	if _, err := file.Seek(0, 0); err != nil {
		http.Error(w, "Failed to process file", http.StatusInternalServerError)
		return
	}

	contentType := http.DetectContentType(buffer)
	if contentType != "application/pdf" && contentType != "image/jpeg" && contentType != "image/png" {
		http.Error(w, "Invalid file type. Only PDF, JPEG, and PNG are accepted.", http.StatusBadRequest)
		return
	}

	// Customer Authorization: if an existing customer_id is provided, verify ownership
	customerType := r.FormValue("customer_type")
	customerID := r.FormValue("customer_id")

	if customerType == "existing" {
		if customerID == "" {
			http.Error(w, "Customer ID is required for existing customer", http.StatusBadRequest)
			return
		}

		role, _ := r.Context().Value(RoleKey).(string)
		if role != "admin" {
			// Verify the customer belongs to this user before accepting the ID
			var exists bool
			err := database.DB.QueryRow(
				"SELECT EXISTS(SELECT 1 FROM customers WHERE id = $1 AND workspace_id = $2)",
				customerID, workspaceID,
			).Scan(&exists)
			if err != nil || !exists {
				http.Error(w, "Customer not found or access denied", http.StatusForbidden)
				return
			}
		} else {
			var exists bool
			err := database.DB.QueryRow(
				"SELECT EXISTS(SELECT 1 FROM customers WHERE id = $1)",
				customerID,
			).Scan(&exists)
			if err != nil || !exists {
				http.Error(w, "Customer not found", http.StatusForbidden)
				return
			}
		}
	}

	// Save file — extension is derived from MIME type, never from user-supplied filename
	newFilename, filePath, err := storage.SaveFile(file, contentType)
	if err != nil {
		log.Printf("Failed to save uploaded file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Use the sanitized original filename for display only (never for storage)
	originalName := handler.Filename
	if len(originalName) > 255 {
		originalName = originalName[:255]
	}

	var docID int
	var cID *string
	if customerType == "existing" {
		cID = &customerID
	}

	query := `INSERT INTO documents (workspace_id, filename, filepath, original_name, status, customer_id) 
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err = database.DB.QueryRow(query, workspaceID, newFilename, filePath, originalName, "uploaded", cID).Scan(&docID)
	if err != nil {
		// Clean up the saved file if we couldn't create the DB record
		os.Remove(filePath)
		log.Printf("Failed to create document record: %v", err)
		http.Error(w, "Failed to create database record", http.StatusInternalServerError)
		return
	}

	// Log the upload event
	userID := r.Context().Value(UserIDKey).(int)
	LogEvent(workspaceID, userID, "document_uploaded", map[string]interface{}{
		"document_id": docID,
		"filename":    originalName,
	})

	// Submit the document to Great Sage for asynchronous OCR + AI processing.
	// The goroutine updates the status to "processing" on success or "failed" on error.
	go sendToGreatSage(docID, filePath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "File uploaded successfully",
		"id":      docID,
	})
}

// sendToGreatSage submits a document to the Great Sage intelligence service
// for asynchronous OCR and AI classification processing.
//
// On successful submission (HTTP 202), the document status is set to "processing".
// On failure, the document status is set to "failed" to prevent it from being
// permanently stuck.
func sendToGreatSage(docID int, filePath string) {
	client, err := services.NewGreatSageClient()
	if err != nil {
		log.Printf("Document %d: Great Sage client error: %v", docID, err)
		database.DB.Exec("UPDATE documents SET status = 'failed' WHERE id = $1", docID)
		return
	}

	err = client.SubmitDocument(docID, filePath)
	if err != nil {
		log.Printf("Document %d: failed to submit to Great Sage: %v", docID, err)
		database.DB.Exec("UPDATE documents SET status = 'failed' WHERE id = $1", docID)
		return
	}

	// Great Sage accepted the document — mark as processing
	_, err = database.DB.Exec("UPDATE documents SET status = 'processing' WHERE id = $1", docID)
	if err != nil {
		log.Printf("Document %d: failed to update status to processing: %v", docID, err)
	}
}
