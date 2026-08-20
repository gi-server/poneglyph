package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewGreatSageClient_MissingURL(t *testing.T) {
	os.Unsetenv("GREAT_SAGE_URL")
	os.Unsetenv("GREAT_SAGE_API_KEY")

	_, err := NewGreatSageClient()
	if err == nil {
		t.Fatal("expected error when GREAT_SAGE_URL is not set")
	}
}

func TestNewGreatSageClient_MissingAPIKey(t *testing.T) {
	t.Setenv("GREAT_SAGE_URL", "http://localhost:8000")
	os.Unsetenv("GREAT_SAGE_API_KEY")

	_, err := NewGreatSageClient()
	if err == nil {
		t.Fatal("expected error when GREAT_SAGE_API_KEY is not set")
	}
}

func TestNewGreatSageClient_ValidConfig(t *testing.T) {
	t.Setenv("GREAT_SAGE_URL", "http://localhost:8000")
	t.Setenv("GREAT_SAGE_API_KEY", "test-key")

	client, err := NewGreatSageClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.BaseURL != "http://localhost:8000" {
		t.Errorf("expected BaseURL 'http://localhost:8000', got '%s'", client.BaseURL)
	}
	if client.APIKey != "test-key" {
		t.Errorf("expected APIKey 'test-key', got '%s'", client.APIKey)
	}
}

// createTestFile creates a temporary file with the given content and returns its path.
func createTestFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "test-document.pdf")
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return fp
}

func TestSubmitDocument_HTTP202(t *testing.T) {
	// Mock Great Sage server that returns 202
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/analyze" {
			t.Errorf("expected path /api/v1/analyze, got %s", r.URL.Path)
		}

		// Verify API key header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "test-api-key" {
			t.Errorf("expected X-API-Key 'test-api-key', got '%s'", apiKey)
		}

		// Verify multipart content type
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			t.Error("expected Content-Type header to be set")
		}

		// Parse multipart form to verify file and document_id
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("failed to parse multipart form: %v", err)
		}

		// Verify document_id
		docID := r.FormValue("document_id")
		if docID != "42" {
			t.Errorf("expected document_id '42', got '%s'", docID)
		}

		// Verify file field exists
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("expected file field: %v", err)
		}
		defer file.Close()
		if header.Filename != "test-document.pdf" {
			t.Errorf("expected filename 'test-document.pdf', got '%s'", header.Filename)
		}

		// Verify file content
		content, _ := io.ReadAll(file)
		if string(content) != "fake pdf content" {
			t.Errorf("unexpected file content: %s", string(content))
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":     "Document accepted for processing",
			"document_id": 42,
		})
	}))
	defer server.Close()

	filePath := createTestFile(t, "fake pdf content")

	client := &GreatSageClient{
		BaseURL:    server.URL,
		APIKey:     "test-api-key",
		HTTPClient: server.Client(),
	}

	err := client.SubmitDocument(42, filePath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestSubmitDocument_HTTP401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
	}))
	defer server.Close()

	filePath := createTestFile(t, "content")

	client := &GreatSageClient{
		BaseURL:    server.URL,
		APIKey:     "wrong-key",
		HTTPClient: server.Client(),
	}

	err := client.SubmitDocument(1, filePath)
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
}

func TestSubmitDocument_HTTP500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	filePath := createTestFile(t, "content")

	client := &GreatSageClient{
		BaseURL:    server.URL,
		APIKey:     "key",
		HTTPClient: server.Client(),
	}

	err := client.SubmitDocument(1, filePath)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestSubmitDocument_ServerUnavailable(t *testing.T) {
	// Use a URL that will fail to connect
	client := &GreatSageClient{
		BaseURL:    "http://127.0.0.1:1", // nothing listening
		APIKey:     "key",
		HTTPClient: &http.Client{},
	}

	filePath := createTestFile(t, "content")

	err := client.SubmitDocument(1, filePath)
	if err == nil {
		t.Fatal("expected error when server is unavailable")
	}
}

func TestSubmitDocument_FileNotFound(t *testing.T) {
	client := &GreatSageClient{
		BaseURL:    "http://localhost:8000",
		APIKey:     "key",
		HTTPClient: &http.Client{},
	}

	err := client.SubmitDocument(1, "/nonexistent/path/file.pdf")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
