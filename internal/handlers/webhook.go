package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"docunest/internal/database"
	"docunest/internal/models"

	"go.mongodb.org/mongo-driver/bson"
)

// greatSageWebhookPayload matches the JSON contract from Great Sage.
type greatSageWebhookPayload struct {
	DocumentID     int                     `json:"document_id"`
	Status         string                  `json:"status"` // "success" or "failed"
	OCRText        *string                 `json:"ocr_text"`
	Classification greatSageClassification `json:"classification"`
	ErrorMessage   *string                 `json:"error_message"`
}

type greatSageClassification struct {
	DocumentType  *string                `json:"document_type"`
	ExtractedData map[string]interface{} `json:"extracted_data"` // Canonical: flexible key-value extraction
	// Legacy identity fields — accepted for backward compatibility with older Great Sage versions.
	PersonName       *string `json:"person_name,omitempty"`
	DOB              *string `json:"dob,omitempty"`
	DocumentIDNumber *string `json:"document_id_number,omitempty"`
}

type jobWebhookFileResult struct {
	Filename       string                  `json:"filename"`
	Status         string                  `json:"status"`
	OCRText        *string                 `json:"ocr_text"`
	Classification greatSageClassification `json:"classification"`
	ErrorMessage   *string                 `json:"error_message"`
}

type jobWebhookPayload struct {
	JobID  string                 `json:"job_id"`
	Status string                 `json:"status"`
	Files  []jobWebhookFileResult `json:"files"`
}

// resolveExtractedData builds the canonical extracted_data JSON from a classification result.
func resolveExtractedData(c greatSageClassification) (extractedJSON []byte, personName, dob, docIDNumber *string) {
	data := c.ExtractedData
	if data == nil {
		data = make(map[string]interface{})
		if c.PersonName != nil {
			data["person_name"] = *c.PersonName
		}
		if c.DOB != nil {
			data["dob"] = *c.DOB
		}
		if c.DocumentIDNumber != nil {
			data["document_id_number"] = *c.DocumentIDNumber
		}
	}

	// Project legacy columns from extracted_data (single source of truth)
	if v, ok := data["person_name"].(string); ok {
		personName = &v
	}
	if v, ok := data["dob"].(string); ok {
		dob = &v
	}
	if v, ok := data["document_id_number"].(string); ok {
		docIDNumber = &v
	}

	extractedJSON, err := json.Marshal(data)
	if err != nil {
		extractedJSON = []byte("{}")
	}
	return extractedJSON, personName, dob, docIDNumber
}

// isLocalCaller verifies that the incoming request originates directly from localhost
// and was not forwarded through a reverse proxy from an external client.
func isLocalCaller(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// AnalyzeWebhook receives asynchronous processing results from Great Sage.
func AnalyzeWebhook(w http.ResponseWriter, r *http.Request) {
	// --- Authenticate the webhook caller (internal local only) ---
	if !isLocalCaller(r) {
		http.Error(w, "Forbidden: internal only", http.StatusForbidden)
		return
	}

	// --- Parse and validate the payload ---
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5 MB limit
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload greatSageWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if payload.DocumentID <= 0 {
		http.Error(w, "Invalid or missing document_id", http.StatusBadRequest)
		return
	}
	if payload.Status != "success" && payload.Status != "failed" {
		http.Error(w, "Invalid status — must be 'success' or 'failed'", http.StatusBadRequest)
		return
	}

	// --- Idempotency check ---
	var doc models.Document
	err = database.GetCollection("documents").FindOne(r.Context(), bson.M{"id": payload.DocumentID}).Decode(&doc)
	if err != nil {
		log.Printf("Webhook: document %d not found: %v", payload.DocumentID, err)
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	if doc.Status != "processing" && doc.Status != "uploaded" {
		// Document has already been updated — acknowledge but don't mutate.
		log.Printf("Webhook: document %d is already in status '%s', ignoring duplicate webhook", payload.DocumentID, doc.Status)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "already processed"})
		return
	}

	// --- Apply the result ---
	if payload.Status == "success" {
		extractedJSON, personName, dob, docIDNumber := resolveExtractedData(payload.Classification)

		update := bson.M{
			"$set": bson.M{
				"ocr_text":           payload.OCRText,
				"document_type":      payload.Classification.DocumentType,
				"extracted_data":     extractedJSON,
				"person_name":        personName,
				"dob":                dob,
				"document_id_number": docIDNumber,
				"status":             "needs_review",
			},
		}

		_, err = database.GetCollection("documents").UpdateOne(
			r.Context(),
			bson.M{"id": payload.DocumentID, "status": bson.M{"$in": []string{"processing", "uploaded"}}},
			update,
		)
		if err != nil {
			log.Printf("Webhook: failed to update document %d with success result: %v", payload.DocumentID, err)
			http.Error(w, "Failed to update document", http.StatusInternalServerError)
			return
		}
		log.Printf("Webhook: document %d → needs_review", payload.DocumentID)
		LogEvent(doc.WorkspaceID, 0, "document_processing_completed", map[string]interface{}{
			"document_id": payload.DocumentID,
			"status":      "needs_review",
		})

	} else {
		// status == "failed"
		updateSet := bson.M{
			"status": "failed",
		}
		if payload.OCRText != nil {
			updateSet["ocr_text"] = payload.OCRText
		}

		_, err = database.GetCollection("documents").UpdateOne(
			r.Context(),
			bson.M{"id": payload.DocumentID, "status": bson.M{"$in": []string{"processing", "uploaded"}}},
			bson.M{"$set": updateSet},
		)
		if err != nil {
			log.Printf("Webhook: failed to update document %d with failure result: %v", payload.DocumentID, err)
			http.Error(w, "Failed to update document", http.StatusInternalServerError)
			return
		}

		errMsg := "unknown error"
		if payload.ErrorMessage != nil {
			errMsg = *payload.ErrorMessage
		}
		log.Printf("Webhook: document %d → failed (%s)", payload.DocumentID, errMsg)
		LogEvent(doc.WorkspaceID, 0, "document_processing_failed", map[string]interface{}{
			"document_id":   payload.DocumentID,
			"error_message": errMsg,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
}

// JobWebhook receives asynchronous processing results from Great Sage's V2 jobs API.
func JobWebhook(w http.ResponseWriter, r *http.Request) {
	// --- Authenticate the webhook caller (internal local only) ---
	if !isLocalCaller(r) {
		http.Error(w, "Forbidden: internal only", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 20<<20) // 20 MB limit for batch results
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload jobWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if payload.JobID == "" {
		http.Error(w, "Invalid or missing job_id", http.StatusBadRequest)
		return
	}

	for _, fileResult := range payload.Files {
		// filename format: "<document_id>_<original_name>"
		parts := strings.SplitN(fileResult.Filename, "_", 2)
		if len(parts) < 2 {
			log.Printf("Webhook: invalid filename format '%s'", fileResult.Filename)
			continue
		}

		docID, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Printf("Webhook: failed to parse document_id from '%s'", fileResult.Filename)
			continue
		}

		var doc models.Document
		err = database.GetCollection("documents").FindOne(
			r.Context(),
			bson.M{"id": docID, "job_id": payload.JobID},
		).Decode(&doc)
		if err != nil {
			log.Printf("Webhook: document %d not found for job %s: %v", docID, payload.JobID, err)
			continue
		}

		if doc.Status != "processing" && doc.Status != "uploaded" {
			continue
		}

		if fileResult.Status == "success" {
			extractedJSON, personName, dob, docIDNumber := resolveExtractedData(fileResult.Classification)

			update := bson.M{
				"$set": bson.M{
					"ocr_text":           fileResult.OCRText,
					"document_type":      fileResult.Classification.DocumentType,
					"extracted_data":     extractedJSON,
					"person_name":        personName,
					"dob":                dob,
					"document_id_number": docIDNumber,
					"status":             "needs_review",
				},
			}

			_, err = database.GetCollection("documents").UpdateOne(
				r.Context(),
				bson.M{"id": docID},
				update,
			)
			if err != nil {
				log.Printf("Webhook: failed to update document %d: %v", docID, err)
				continue
			}
			LogEvent(doc.WorkspaceID, 0, "document_processing_completed", map[string]interface{}{
				"document_id": docID,
				"status":      "needs_review",
			})
		} else {
			updateSet := bson.M{"status": "failed"}
			if fileResult.OCRText != nil {
				updateSet["ocr_text"] = fileResult.OCRText
			}

			_, err = database.GetCollection("documents").UpdateOne(
				r.Context(),
				bson.M{"id": docID},
				bson.M{"$set": updateSet},
			)
			if err != nil {
				log.Printf("Webhook: failed to update document %d failure: %v", docID, err)
				continue
			}
			errMsg := "unknown error"
			if fileResult.ErrorMessage != nil {
				errMsg = *fileResult.ErrorMessage
			}
			LogEvent(doc.WorkspaceID, 0, "document_processing_failed", map[string]interface{}{
				"document_id":   docID,
				"error_message": errMsg,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
}
