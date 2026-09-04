package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	IsDisabled   bool      `json:"is_disabled"`
	CreatedAt    time.Time `json:"created_at"`
}

type Customer struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Document struct {
	ID            int             `json:"id"`
	UserID        int             `json:"user_id"`
	Filename      string          `json:"filename"`
	Filepath      string          `json:"-"`
	OriginalName  string          `json:"original_name"`
	Status        string          `json:"status"` // uploaded, processing, needs_review, completed, failed
	OCRText       *string         `json:"ocr_text,omitempty"`
	DocumentType  *string         `json:"document_type,omitempty"`
	ExtractedData json.RawMessage `json:"extracted_data"` // Canonical source of truth for all extracted fields
	Confidence    *float64        `json:"confidence,omitempty"`
	CustomerID    *string         `json:"customer_id,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	// Legacy projection fields — synchronized mirrors of identity-document fields
	// inside extracted_data. Kept temporarily for backward compatibility.
	PersonName       *string `json:"person_name,omitempty"`
	DOB              *string `json:"dob,omitempty"`
	DocumentIDNumber *string `json:"document_id_number,omitempty"`
}

type AuditLog struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	DocumentID int       `json:"document_id"`
	Action     string    `json:"action"`
	Details    *string   `json:"details,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type ReviewRequest struct {
	DocumentType  string          `json:"document_type"`
	CustomerID    string          `json:"customer_id"`
	ExtractedData json.RawMessage `json:"extracted_data"` // Canonical: dynamic fields from the review form
	// Legacy fields — accepted from older clients for backward compatibility.
	// If ExtractedData is provided, these are ignored.
	PersonName       string `json:"person_name,omitempty"`
	DOB              string `json:"dob,omitempty"`
	DocumentIDNumber string `json:"document_id_number,omitempty"`
}


type DocumentShare struct {
	Token      string    `json:"token"`
	DocumentID int       `json:"document_id"`
	ExpiresAt  time.Time `json:"expires_at"`
	SingleUse  bool      `json:"single_use"`
	IsRevoked  bool      `json:"is_revoked"`
	CreatedAt  time.Time `json:"created_at"`
}
