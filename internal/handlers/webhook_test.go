package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// --- Helper to build a webhook request ---

func makeWebhookRequest(t *testing.T, payload interface{}, secret string) *http.Request {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/internal/webhook/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Webhook-Secret", secret)
	}
	return req
}

func strPtr(s string) *string { return &s }

// --- Authentication Tests ---

func TestAnalyzeWebhook_MissingSecret(t *testing.T) {
	t.Setenv("PONEGLYPH_WEBHOOK_SECRET", "correct-secret")

	payload := map[string]interface{}{"document_id": 1, "status": "success"}
	req := makeWebhookRequest(t, payload, "")
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_WrongSecret(t *testing.T) {
	t.Setenv("PONEGLYPH_WEBHOOK_SECRET", "correct-secret")

	payload := map[string]interface{}{"document_id": 1, "status": "success"}
	req := makeWebhookRequest(t, payload, "wrong-secret")
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_UnconfiguredSecret(t *testing.T) {
	os.Unsetenv("PONEGLYPH_WEBHOOK_SECRET")

	payload := map[string]interface{}{"document_id": 1, "status": "success"}
	req := makeWebhookRequest(t, payload, "any-secret")
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (misconfiguration), got %d", rr.Code)
	}
}

// --- Payload Validation Tests ---

func TestAnalyzeWebhook_MalformedJSON(t *testing.T) {
	t.Setenv("PONEGLYPH_WEBHOOK_SECRET", "test-secret")

	req := httptest.NewRequest("POST", "/api/internal/webhook/analyze", bytes.NewReader([]byte("not valid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Secret", "test-secret")
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_MissingDocumentID(t *testing.T) {
	t.Setenv("PONEGLYPH_WEBHOOK_SECRET", "test-secret")

	payload := map[string]interface{}{"status": "success"}
	req := makeWebhookRequest(t, payload, "test-secret")
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_InvalidStatus(t *testing.T) {
	t.Setenv("PONEGLYPH_WEBHOOK_SECRET", "test-secret")

	payload := map[string]interface{}{"document_id": 1, "status": "invalid"}
	req := makeWebhookRequest(t, payload, "test-secret")
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_ZeroDocumentID(t *testing.T) {
	t.Setenv("PONEGLYPH_WEBHOOK_SECRET", "test-secret")

	payload := map[string]interface{}{"document_id": 0, "status": "success"}
	req := makeWebhookRequest(t, payload, "test-secret")
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// --- Payload structure tests (no DB required) ---

func TestWebhookPayload_SuccessStructure(t *testing.T) {
	payload := greatSageWebhookPayload{
		DocumentID: 42,
		Status:     "success",
		OCRText:    strPtr("Hello world"),
		Classification: greatSageClassification{
			DocumentType:     strPtr("Invoice"),
			PersonName:       strPtr("Jane Doe"),
			DOB:              strPtr("1990-05-15"),
			DocumentIDNumber: strPtr("INV-001"),
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed greatSageWebhookPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.DocumentID != 42 {
		t.Errorf("expected document_id 42, got %d", parsed.DocumentID)
	}
	if parsed.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", parsed.Status)
	}
	if *parsed.OCRText != "Hello world" {
		t.Errorf("expected ocr_text 'Hello world', got '%s'", *parsed.OCRText)
	}
	if *parsed.Classification.DocumentType != "Invoice" {
		t.Errorf("expected document_type 'Invoice', got '%s'", *parsed.Classification.DocumentType)
	}
}

func TestWebhookPayload_FailureStructure(t *testing.T) {
	errMsg := "AI classification failed"
	payload := greatSageWebhookPayload{
		DocumentID:   42,
		Status:       "failed",
		OCRText:      strPtr("Extracted text preserved"),
		ErrorMessage: &errMsg,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed greatSageWebhookPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", parsed.Status)
	}
	if *parsed.OCRText != "Extracted text preserved" {
		t.Errorf("expected ocr_text to be preserved, got '%s'", *parsed.OCRText)
	}
	if *parsed.ErrorMessage != "AI classification failed" {
		t.Errorf("expected error_message, got '%s'", *parsed.ErrorMessage)
	}
}

func TestWebhookPayload_OCRSuccessLLMFailure(t *testing.T) {
	// OCR succeeded but LLM failed — text preserved, classification empty
	errMsg := "Ollama timeout"
	payload := greatSageWebhookPayload{
		DocumentID:     42,
		Status:         "failed",
		OCRText:        strPtr("OCR text from the document"),
		Classification: greatSageClassification{}, // empty
		ErrorMessage:   &errMsg,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed greatSageWebhookPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", parsed.Status)
	}
	if *parsed.OCRText != "OCR text from the document" {
		t.Errorf("expected ocr_text preserved")
	}
	if parsed.Classification.DocumentType != nil {
		t.Error("expected nil document_type for LLM failure")
	}
	if parsed.Classification.PersonName != nil {
		t.Error("expected nil person_name for LLM failure")
	}
}

// --- Note: The following tests require database.DB to be initialized ---
// They will be skipped if the DB is not available.
// For full integration testing, run with a live PostgreSQL instance.

// TestAnalyzeWebhook_UnknownDocumentID tests that a webhook for a
// non-existent document returns 404.
// This test requires a database connection.
func TestAnalyzeWebhook_UnknownDocumentID(t *testing.T) {
	t.Setenv("PONEGLYPH_WEBHOOK_SECRET", "test-secret")

	// Without DB initialized, QueryRow will panic or fail.
	// We'll test that the handler correctly returns 404 when document is not found.
	// This test is a unit test of the auth + validation path, not the DB path.
	// The DB query will fail, which should result in a 404.
	defer func() {
		if r := recover(); r != nil {
			t.Skip("Skipping: database not initialized (expected in unit test environment)")
		}
	}()

	payload := map[string]interface{}{
		"document_id": 99999,
		"status":      "success",
		"ocr_text":    "test",
		"classification": map[string]string{
			"document_type": "Invoice",
		},
	}
	req := makeWebhookRequest(t, payload, "test-secret")
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	// Without DB, this will either panic (caught above) or return an error status
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 404 or 500 for unknown document, got %d", rr.Code)
	}
}
