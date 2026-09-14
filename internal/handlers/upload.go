package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"docunest/internal/database"
	"docunest/internal/models"
	"docunest/internal/services"
	"docunest/internal/storage"

	"go.mongodb.org/mongo-driver/bson"
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

	files := r.MultipartForm.File["documents"]
	if len(files) == 0 {
		http.Error(w, "No documents provided", http.StatusBadRequest)
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
		custFilter := bson.M{"id": customerID}
		if role != "admin" {
			custFilter["workspace_id"] = workspaceID
		}

		count, err := database.GetCollection("customers").CountDocuments(r.Context(), custFilter)
		if err != nil || count == 0 {
			http.Error(w, "Customer not found or access denied", http.StatusForbidden)
			return
		}
	}

	var docInfos []services.DocumentInfo
	var docIDs []int

	for _, handler := range files {
		file, err := handler.Open()
		if err != nil {
			log.Printf("Failed to open uploaded file: %v", err)
			continue
		}

		// MIME Validation: read first 512 bytes to determine real content type
		buffer := make([]byte, 512)
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			file.Close()
			continue
		}
		buffer = buffer[:n]
		if _, err := file.Seek(0, 0); err != nil {
			file.Close()
			continue
		}

		contentType := http.DetectContentType(buffer)
		if contentType != "application/pdf" && contentType != "image/jpeg" && contentType != "image/png" {
			file.Close()
			continue // Skip invalid file types
		}

		// Save file
		newFilename, filePath, err := storage.SaveFile(file, contentType)
		file.Close()
		if err != nil {
			log.Printf("Failed to save uploaded file: %v", err)
			continue
		}

		// Use the sanitized original filename for display only
		originalName := handler.Filename
		if len(originalName) > 255 {
			originalName = originalName[:255]
		}

		var cID *string
		if customerType == "existing" {
			cID = &customerID
		}

		docID, err := database.GetNextSequence("documents")
		if err != nil {
			os.Remove(filePath)
			log.Printf("Failed to generate document ID sequence: %v", err)
			continue
		}

		docRecord := models.Document{
			ID:            docID,
			WorkspaceID:   workspaceID,
			Filename:      newFilename,
			Filepath:      filePath,
			OriginalName:  originalName,
			Status:        "uploaded",
			CustomerID:    cID,
			ExtractedData: json.RawMessage("{}"),
			CreatedAt:     time.Now(),
		}

		_, err = database.GetCollection("documents").InsertOne(r.Context(), docRecord)
		if err != nil {
			os.Remove(filePath)
			log.Printf("Failed to create document record: %v", err)
			continue
		}

		// Log the upload event
		userID := r.Context().Value(UserIDKey).(int)
		LogEvent(workspaceID, userID, "document_uploaded", map[string]interface{}{
			"document_id": docID,
			"filename":    originalName,
		})

		docInfos = append(docInfos, services.DocumentInfo{
			ID:       docID,
			FilePath: filePath,
		})
		docIDs = append(docIDs, docID)
	}

	if len(docInfos) == 0 {
		http.Error(w, "No valid documents were uploaded", http.StatusBadRequest)
		return
	}

	// Submit the batch of documents to Great Sage's V2 API
	sendJobToGreatSage(docInfos)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": fmt.Sprintf("%d files uploaded successfully", len(docIDs)),
		"ids":     docIDs,
	})
}

// sendJobToGreatSage submits a batch of documents to Great Sage
// for asynchronous OCR and AI classification processing using V2 APIs.
func sendJobToGreatSage(docs []services.DocumentInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var ids []int
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}

	client, err := services.NewGreatSageClient()
	if err != nil {
		log.Printf("Great Sage client error: %v", err)
		database.GetCollection("documents").UpdateMany(ctx, bson.M{"id": bson.M{"$in": ids}}, bson.M{"$set": bson.M{"status": "failed"}})
		return
	}

	webhookURL := os.Getenv("API_BASE_URL")
	if webhookURL == "" {
		webhookURL = "http://backend:8080" // default for local docker
	}
	webhookURL = webhookURL + "/api/internal/webhook/jobs"

	jobID, err := client.SubmitJob(docs, webhookURL)
	if err != nil {
		log.Printf("Failed to submit job to Great Sage: %v", err)
		database.GetCollection("documents").UpdateMany(ctx, bson.M{"id": bson.M{"$in": ids}}, bson.M{"$set": bson.M{"status": "failed"}})
		return
	}

	// Job submitted — mark all as processing and store the job ID
	_, err = database.GetCollection("documents").UpdateMany(
		ctx,
		bson.M{"id": bson.M{"$in": ids}},
		bson.M{"$set": bson.M{"status": "processing", "job_id": jobID}},
	)
	if err != nil {
		log.Printf("Failed to update status to processing for job %s: %v", jobID, err)
	}
}
