package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// GreatSageClient handles communication with the Great Sage document intelligence service.
type GreatSageClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewGreatSageClient creates a client from environment variables.
// Returns an error if required configuration is missing.
func NewGreatSageClient() (*GreatSageClient, error) {
	baseURL := os.Getenv("GREAT_SAGE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("GREAT_SAGE_URL is not configured")
	}

	apiKey := os.Getenv("GREAT_SAGE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GREAT_SAGE_API_KEY is not configured")
	}

	return &GreatSageClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			// Timeout covers the full round-trip for the submission request only.
			// Great Sage returns 202 quickly; this is NOT the processing timeout.
			Timeout: 30 * time.Second,
		},
	}, nil
}

// SubmitDocument sends a document file to Great Sage for asynchronous processing.
// It returns nil on successful submission (HTTP 202) or an error otherwise.
func (c *GreatSageClient) SubmitDocument(docID int, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the file field
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err = io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Add document_id field
	if err := writer.WriteField("document_id", strconv.Itoa(docID)); err != nil {
		return fmt.Errorf("failed to write document_id field: %w", err)
	}

	writer.Close()

	url := c.BaseURL + "/api/v1/analyze"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("Great Sage unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		log.Printf("Document %d submitted to Great Sage successfully", docID)
		return nil
	}

	// Read a limited error response body for diagnostics
	errBody := make([]byte, 512)
	n, _ := io.ReadAtLeast(resp.Body, errBody, 1)
	errBody = errBody[:n]

	return fmt.Errorf("Great Sage returned HTTP %d: %s", resp.StatusCode, string(errBody))
}

// DocumentInfo holds information about a document to submit to Great Sage.
type DocumentInfo struct {
	ID       int
	FilePath string
}

// SubmitJob sends a batch of documents to Great Sage's V2 jobs API.
// It returns the Job ID returned by Great Sage, or an error.
func (c *GreatSageClient) SubmitJob(docs []DocumentInfo, webhookURL string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, doc := range docs {
		file, err := os.Open(doc.FilePath)
		if err != nil {
			return "", fmt.Errorf("failed to open file %s: %w", doc.FilePath, err)
		}
		defer file.Close()

		// Include the Poneglyph document ID in the filename so it can be parsed from the webhook
		filename := fmt.Sprintf("%d_%s", doc.ID, filepath.Base(doc.FilePath))
		part, err := writer.CreateFormFile("files", filename)
		if err != nil {
			return "", fmt.Errorf("failed to create form file for %s: %w", filename, err)
		}
		if _, err = io.Copy(part, file); err != nil {
			return "", fmt.Errorf("failed to copy file content for %s: %w", filename, err)
		}
	}

	if webhookURL != "" {
		if err := writer.WriteField("webhook_url", webhookURL); err != nil {
			return "", fmt.Errorf("failed to write webhook_url field: %w", err)
		}
	}

	writer.Close()

	url := c.BaseURL + "/api/v2/jobs"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Great Sage unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
		var result struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			log.Printf("Job submitted to Great Sage successfully, Job ID: %s", result.ID)
			return result.ID, nil
		}
		return "", fmt.Errorf("failed to parse Great Sage response")
	}

	errBody := make([]byte, 512)
	n, _ := io.ReadAtLeast(resp.Body, errBody, 1)
	errBody = errBody[:n]

	return "", fmt.Errorf("Great Sage returned HTTP %d: %s", resp.StatusCode, string(errBody))
}
