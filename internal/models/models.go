package models

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	MongoID      primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	ID           int                `json:"id" bson:"id"`
	Username     string             `json:"username" bson:"username"`
	PasswordHash string             `json:"-" bson:"password_hash"`
	Role         string             `json:"role" bson:"role"`
	IsDisabled   bool               `json:"is_disabled" bson:"is_disabled"`
	AdminID      *int               `json:"admin_id,omitempty" bson:"admin_id,omitempty"`
	CreatedAt    time.Time          `json:"created_at" bson:"created_at"`
}

type Customer struct {
	MongoID     primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	ID          string             `json:"id" bson:"id"`
	WorkspaceID int                `json:"user_id" bson:"workspace_id"`
	Name        string             `json:"name" bson:"name"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
}

type Document struct {
	MongoID       primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	ID            int                `json:"id" bson:"id"`
	WorkspaceID   int                `json:"user_id" bson:"workspace_id"`
	Filename      string             `json:"filename" bson:"filename"`
	Filepath      string             `json:"-" bson:"filepath"`
	OriginalName  string             `json:"original_name" bson:"original_name"`
	Status        string             `json:"status" bson:"status"` // uploaded, processing, needs_review, completed, failed
	OCRText       *string            `json:"ocr_text,omitempty" bson:"ocr_text,omitempty"`
	DocumentType  *string            `json:"document_type,omitempty" bson:"document_type,omitempty"`
	ExtractedData json.RawMessage    `json:"extracted_data" bson:"extracted_data"` // Canonical source of truth for all extracted fields
	Confidence    *float64           `json:"confidence,omitempty" bson:"confidence,omitempty"`
	CustomerID    *string            `json:"customer_id,omitempty" bson:"customer_id,omitempty"`
	JobID         *string            `json:"job_id,omitempty" bson:"job_id,omitempty"`
	CreatedAt     time.Time          `json:"created_at" bson:"created_at"`
	// Legacy projection fields — synchronized mirrors of identity-document fields
	// inside extracted_data. Kept temporarily for backward compatibility.
	PersonName       *string `json:"person_name,omitempty" bson:"person_name,omitempty"`
	DOB              *string `json:"dob,omitempty" bson:"dob,omitempty"`
	DocumentIDNumber *string `json:"document_id_number,omitempty" bson:"document_id_number,omitempty"`
}

type AuditLog struct {
	MongoID     primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	ID          int                `json:"id" bson:"id"`
	WorkspaceID int                `json:"user_id" bson:"workspace_id"`
	ActorID     *int               `json:"actor_id,omitempty" bson:"actor_id,omitempty"`
	DocumentID  *int               `json:"document_id,omitempty" bson:"document_id,omitempty"`
	Action      string             `json:"action" bson:"action"`
	Details     interface{}        `json:"details,omitempty" bson:"details,omitempty"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
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
	MongoID    primitive.ObjectID `json:"-" bson:"_id,omitempty"`
	Token      string             `json:"token" bson:"token"`
	DocumentID int                `json:"document_id" bson:"document_id"`
	ExpiresAt  time.Time          `json:"expires_at" bson:"expires_at"`
	SingleUse  bool               `json:"single_use" bson:"single_use"`
	IsRevoked  bool               `json:"is_revoked" bson:"is_revoked"`
	CreatedAt  time.Time          `json:"created_at" bson:"created_at"`
}

