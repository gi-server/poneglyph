package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"docunest/internal/database"
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
// If the classification already contains an ExtractedData map, it is used directly.
// Otherwise, legacy identity fields are assembled into extracted_data for backward compatibility.
// Returns the JSON bytes for JSONB storage and the legacy projection values.
func resolveExtractedData(c greatSageClassification) (extractedJSON []byte, personName, dob, docIDNumber *string) {
	data := c.ExtractedData
	if data == nil {
		// Build extracted_data from legacy fields
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

// AnalyzeWebhook receives asynchronous processing results from Great Sage.
//
// This route does NOT use the normal user-session AuthMiddleware.
// Instead, it authenticates Great Sage via the X-Webhook-Secret header.
//
// Idempotency: if the document has already been moved past the processing
// state (e.g., to needs_review, completed, or already failed), the webhook
// is acknowledged but no further DB mutation occurs.
func AnalyzeWebhook(w http.ResponseWriter, r *http.Request) {
	// --- Authenticate the webhook caller ---
	expectedSecret := os.Getenv("PONEGLYPH_WEBHOOK_SECRET")
	if expectedSecret == "" {
		log.Println("WARNING: PONEGLYPH_WEBHOOK_SECRET is not configured")
		http.Error(w, "Server misconfiguration", http.StatusInternalServerError)
		return
	}

	receivedSecret := r.Header.Get("X-Webhook-Secret")
	if receivedSecret == "" || subtle.ConstantTimeCompare([]byte(receivedSecret), []byte(expectedSecret)) != 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
	// Only update documents that are still in "processing" (or "uploaded" if the
	// status update to "processing" raced). Do not overwrite documents that have
	// already transitioned to needs_review, completed, or been re-processed.
	var currentStatus string
	var workspaceID int
	err = database.DB.QueryRow(
		"SELECT status, workspace_id FROM documents WHERE id = $1", payload.DocumentID,
	).Scan(&currentStatus, &workspaceID)
	if err != nil {
		log.Printf("Webhook: document %d not found: %v", payload.DocumentID, err)
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	if currentStatus != "processing" && currentStatus != "uploaded" {
		// Document has already been updated — acknowledge but don't mutate.
		log.Printf("Webhook: document %d is already in status '%s', ignoring duplicate webhook", payload.DocumentID, currentStatus)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "already processed"})
		return
	}

	// --- Apply the result ---
	if payload.Status == "success" {
		extractedJSON, personName, dob, docIDNumber := resolveExtractedData(payload.Classification)

		_, err = database.DB.Exec(`
			UPDATE documents
			SET ocr_text = $1,
			    document_type = $2,
			    extracted_data = $3,
			    person_name = $4,
			    dob = $5,
			    document_id_number = $6,
			    status = 'needs_review'
			WHERE id = $7 AND status IN ('processing', 'uploaded')
		`,
			payload.OCRText,
			payload.Classification.DocumentType,
			extractedJSON,
			personName,
			dob,
			docIDNumber,
			payload.DocumentID,
		)
		if err != nil {
			log.Printf("Webhook: failed to update document %d with success result: %v", payload.DocumentID, err)
			http.Error(w, "Failed to update document", http.StatusInternalServerError)
			return
		}
		log.Printf("Webhook: document %d → needs_review", payload.DocumentID)
		LogEvent(workspaceID, 0, "document_processing_completed", map[string]interface{}{
			"document_id": payload.DocumentID,
			"status":      "needs_review",
		})

	} else {
		// status == "failed"
		// Preserve OCR text if available (graceful degradation: OCR succeeded, LLM failed)
		_, err = database.DB.Exec(`
			UPDATE documents
			SET ocr_text = COALESCE($1, ocr_text),
			    status = 'failed'
			WHERE id = $2 AND status IN ('processing', 'uploaded')
		`,
			payload.OCRText,
			payload.DocumentID,
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
		LogEvent(workspaceID, 0, "document_processing_failed", map[string]interface{}{
			"document_id":   payload.DocumentID,
			"error_message": errMsg,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
}

// JobWebhook receives asynchronous processing results from Great Sage's V2 jobs API.
// It applies the individual file classifications to their respective documents in Poneglyph.
func JobWebhook(w http.ResponseWriter, r *http.Request) {
	expectedSecret := os.Getenv("PONEGLYPH_WEBHOOK_SECRET")
	if expectedSecret == "" {
		http.Error(w, "Server misconfiguration", http.StatusInternalServerError)
		return
	}
	receivedSecret := r.Header.Get("X-Webhook-Secret")
	if receivedSecret == "" || subtle.ConstantTimeCompare([]byte(receivedSecret), []byte(expectedSecret)) != 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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

		var currentStatus string
		var workspaceID int
		err = database.DB.QueryRow(
			"SELECT status, workspace_id FROM documents WHERE id = $1 AND job_id = $2", 
			docID, payload.JobID,
		).Scan(&currentStatus, &workspaceID)
		if err != nil {
			log.Printf("Webhook: document %d not found for job %s: %v", docID, payload.JobID, err)
			continue
		}

		if currentStatus != "processing" && currentStatus != "uploaded" {
			continue
		}

		if fileResult.Status == "success" {
			extractedJSON, personName, dob, docIDNumber := resolveExtractedData(fileResult.Classification)

			_, err = database.DB.Exec(`
				UPDATE documents
				SET ocr_text = $1,
					document_type = $2,
					extracted_data = $3,
					person_name = $4,
					dob = $5,
					document_id_number = $6,
					status = 'needs_review'
				WHERE id = $7
			`,
				fileResult.OCRText,
				fileResult.Classification.DocumentType,
				extractedJSON,
				personName,
				dob,
				docIDNumber,
				docID,
			)
			if err != nil {
				log.Printf("Webhook: failed to update document %d: %v", docID, err)
				continue
			}
			LogEvent(workspaceID, 0, "document_processing_completed", map[string]interface{}{
				"document_id": docID,
				"status":      "needs_review",
			})
		} else {
			_, err = database.DB.Exec(`
				UPDATE documents
				SET ocr_text = COALESCE($1, ocr_text),
					status = 'failed'
				WHERE id = $2
			`,
				fileResult.OCRText,
				docID,
			)
			if err != nil {
				log.Printf("Webhook: failed to update document %d failure: %v", docID, err)
				continue
			}
			errMsg := "unknown error"
			if fileResult.ErrorMessage != nil {
				errMsg = *fileResult.ErrorMessage
			}
			LogEvent(workspaceID, 0, "document_processing_failed", map[string]interface{}{
				"document_id":   docID,
				"error_message": errMsg,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
}
