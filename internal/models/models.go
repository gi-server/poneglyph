package models

import (
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

type JobFile struct {
	Filename string `json:"filename" bson:"filename"`
	Size     int64  `json:"size" bson:"size"`
	MimeType string `json:"mime_type,omitempty" bson:"mime_type,omitempty"`
}

type Job struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UploadedAt time.Time          `json:"uploaded_at" bson:"uploaded_at"`
	Files      []JobFile          `json:"files" bson:"files"`
}
