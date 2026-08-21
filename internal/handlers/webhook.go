package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

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
	DocumentType     *string `json:"document_type"`
	PersonName       *string `json:"person_name"`
	DOB              *string `json:"dob"`
	DocumentIDNumber *string `json:"document_id_number"`
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
	if receivedSecret == "" || receivedSecret != expectedSecret {
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
	err = database.DB.QueryRow(
		"SELECT status FROM documents WHERE id = $1", payload.DocumentID,
	).Scan(&currentStatus)
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
		_, err = database.DB.Exec(`
			UPDATE documents
			SET ocr_text = $1,
			    document_type = $2,
			    person_name = $3,
			    dob = $4,
			    document_id_number = $5,
			    status = 'needs_review'
			WHERE id = $6 AND status IN ('processing', 'uploaded')
		`,
			payload.OCRText,
			payload.Classification.DocumentType,
			payload.Classification.PersonName,
			payload.Classification.DOB,
			payload.Classification.DocumentIDNumber,
			payload.DocumentID,
		)
		if err != nil {
			log.Printf("Webhook: failed to update document %d with success result: %v", payload.DocumentID, err)
			http.Error(w, "Failed to update document", http.StatusInternalServerError)
			return
		}
		log.Printf("Webhook: document %d → needs_review", payload.DocumentID)

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
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
}
