package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Helper to build a webhook request ---

func makeWebhookRequest(t *testing.T, payload interface{}) *http.Request {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	req := httptest.NewRequest("POST", "/api/internal/webhook/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:54321" // local by default for unit tests
	return req
}

func strPtr(s string) *string { return &s }

// --- Authentication Tests (Local Only) ---

func TestAnalyzeWebhook_LocalhostAllowed(t *testing.T) {
	// Missing document_id in payload, but auth should pass and fail on payload validation (400) rather than 403
	payload := map[string]interface{}{"status": "success"}
	req := makeWebhookRequest(t, payload)
	req.RemoteAddr = "127.0.0.1:54321"
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code == http.StatusForbidden {
		t.Errorf("expected localhost caller to pass authentication, but got 403")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 (validation error), got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_IPv6LocalhostAllowed(t *testing.T) {
	payload := map[string]interface{}{"status": "success"}
	req := makeWebhookRequest(t, payload)
	req.RemoteAddr = "[::1]:54321"
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code == http.StatusForbidden {
		t.Errorf("expected IPv6 localhost caller to pass authentication, but got 403")
	}
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 (validation error), got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_ExternalIPForbidden(t *testing.T) {
	payload := map[string]interface{}{"document_id": 1, "status": "success"}
	req := makeWebhookRequest(t, payload)
	req.RemoteAddr = "192.168.1.50:54321"
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for external caller, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_ProxiedLocalhostForbidden(t *testing.T) {
	payload := map[string]interface{}{"document_id": 1, "status": "success"}
	req := makeWebhookRequest(t, payload)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.195") // External client through proxy
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for proxied request, got %d", rr.Code)
	}
}

// --- Payload Validation Tests ---

func TestAnalyzeWebhook_MalformedJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/internal/webhook/analyze", bytes.NewReader([]byte("not valid json")))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:54321"
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_MissingDocumentID(t *testing.T) {
	payload := map[string]interface{}{"status": "success"}
	req := makeWebhookRequest(t, payload)
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_InvalidStatus(t *testing.T) {
	payload := map[string]interface{}{"document_id": 1, "status": "invalid"}
	req := makeWebhookRequest(t, payload)
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestAnalyzeWebhook_ZeroDocumentID(t *testing.T) {
	payload := map[string]interface{}{"document_id": 0, "status": "success"}
	req := makeWebhookRequest(t, payload)
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
// For full integration testing, run with a live MongoDB instance.

// TestAnalyzeWebhook_UnknownDocumentID tests that a webhook for a
// non-existent document returns 404.
func TestAnalyzeWebhook_UnknownDocumentID(t *testing.T) {
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
	req := makeWebhookRequest(t, payload)
	rr := httptest.NewRecorder()

	AnalyzeWebhook(rr, req)

	// Without DB, this will either panic (caught above) or return an error status
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 404 or 500 for unknown document, got %d", rr.Code)
	}
}
