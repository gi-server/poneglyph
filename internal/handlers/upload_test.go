package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"poneglyph/internal/models"
)

var testIPCounter int

func createMultipartUploadRequest(t *testing.T, fieldName string, files map[string][]byte) *http.Request {
	t.Helper()
	ResetUploadRateLimiter()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for filename, content := range files {
		part, err := writer.CreateFormFile(fieldName, filename)
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("failed to write content: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	testIPCounter++
	req := httptest.NewRequest("POST", "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.RemoteAddr = fmt.Sprintf("192.0.2.%d:54321", testIPCounter%250+1)
	return req
}

func TestUploadBatch_NoFiles(t *testing.T) {
	req := createMultipartUploadRequest(t, "files", map[string][]byte{})
	rr := httptest.NewRecorder()

	UploadBatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty files, got %d", rr.Code)
	}
}

func TestUploadBatch_TooManyFiles(t *testing.T) {
	files := make(map[string][]byte)
	for i := 1; i <= 11; i++ {
		files[fmt.Sprintf("file_%d.pdf", i)] = []byte("dummy pdf content")
	}

	req := createMultipartUploadRequest(t, "files", files)
	rr := httptest.NewRecorder()

	UploadBatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for 11 files (> 10 limit), got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "maximum allowed is 10") {
		t.Errorf("expected error message mentioning 10 files limit, got: %s", rr.Body.String())
	}
}

func TestUploadBatch_InvalidFormat(t *testing.T) {
	files := map[string][]byte{
		"script.sh": []byte("#!/bin/bash\necho hello"),
	}

	req := createMultipartUploadRequest(t, "files", files)
	rr := httptest.NewRecorder()

	UploadBatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for unsupported file extension, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "unsupported format") {
		t.Errorf("expected error message mentioning unsupported format, got: %s", rr.Body.String())
	}
}

func TestUploadBatch_FileTooLarge(t *testing.T) {
	hugeContent := make([]byte, 26*1024*1024)
	files := map[string][]byte{
		"huge.pdf": hugeContent,
	}

	req := createMultipartUploadRequest(t, "files", files)
	rr := httptest.NewRecorder()

	UploadBatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for file > 25MB, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "too large") {
		t.Errorf("expected error message mentioning too large, got: %s", rr.Body.String())
	}
}

func TestUploadBatch_Success(t *testing.T) {
	// Use temporary upload directory for test
	tempDir, err := os.MkdirTemp("", "poneglyph-upload-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Setenv("UPLOAD_DIR", tempDir)

	files := map[string][]byte{
		"invoice.pdf": []byte("%PDF-1.4 sample pdf content"),
		"photo.png":   []byte("\x89PNG\r\n\x1a\n sample png content"),
	}

	req := createMultipartUploadRequest(t, "files", files)
	rr := httptest.NewRecorder()

	UploadBatch(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var job models.Job
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if job.ID.IsZero() {
		t.Errorf("expected non-zero ObjectId job id")
	}
	hexID := job.ID.Hex()
	if len(hexID) != 24 {
		t.Errorf("expected 24-character hexadecimal ObjectId string, got %s", hexID)
	}
	if len(job.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(job.Files))
	}

	// Verify folder and files exist on disk
	jobDir := filepath.Join(tempDir, hexID)
	if stat, err := os.Stat(jobDir); err != nil || !stat.IsDir() {
		t.Fatalf("expected job directory %s to exist", jobDir)
	}

	pdfPath := filepath.Join(jobDir, "invoice.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		t.Errorf("expected file invoice.pdf in %s", jobDir)
	}

	pngPath := filepath.Join(jobDir, "photo.png")
	if _, err := os.Stat(pngPath); err != nil {
		t.Errorf("expected file photo.png in %s", jobDir)
	}
}
