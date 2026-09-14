package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"docunest/internal/database"
	"docunest/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
)

// Generate secure random token
func generateToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Failed to generate random token: %v", err)
	}
	return hex.EncodeToString(b)
}

func CreateShareLink(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	userID, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	docID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid document ID", http.StatusBadRequest)
		return
	}

	role, _ := r.Context().Value(RoleKey).(string)

	docFilter := bson.M{"id": docID}
	if role != "admin" {
		docFilter["workspace_id"] = workspaceID
	}

	count, err := database.GetCollection("documents").CountDocuments(r.Context(), docFilter)
	if err != nil || count == 0 {
		http.Error(w, "Document not found or access denied", http.StatusNotFound)
		return
	}

	var req struct {
		ExpiresInHours int  `json:"expires_in_hours"`
		SingleUse      bool `json:"single_use"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ExpiresInHours = 1 // default 1 hour
	}
	if req.ExpiresInHours <= 0 || req.ExpiresInHours > 72 {
		req.ExpiresInHours = 1
	}

	token := generateToken(32)
	expiresAt := time.Now().Add(time.Duration(req.ExpiresInHours) * time.Hour)

	shareRecord := models.DocumentShare{
		Token:      token,
		DocumentID: docID,
		ExpiresAt:  expiresAt,
		SingleUse:  req.SingleUse,
		IsRevoked:  false,
		CreatedAt:  time.Now(),
	}

	_, err = database.GetCollection("document_shares").InsertOne(r.Context(), shareRecord)
	if err != nil {
		http.Error(w, "Failed to create share link", http.StatusInternalServerError)
		return
	}

	LogEvent(workspaceID, userID, "share_link_created", map[string]interface{}{"document_id": docID, "expires_at": expiresAt})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":      token,
		"expires_at": expiresAt,
		"url":        "/api/share/" + token,
	})
}

func ViewSharedDocument(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	var share models.DocumentShare
	err := database.GetCollection("document_shares").FindOne(r.Context(), bson.M{"token": token}).Decode(&share)
	if err != nil {
		http.Error(w, "Invalid or expired share link", http.StatusNotFound)
		return
	}

	if share.IsRevoked {
		http.Error(w, "This share link has been revoked", http.StatusForbidden)
		return
	}

	if time.Now().After(share.ExpiresAt) {
		http.Error(w, "This share link has expired", http.StatusForbidden)
		return
	}

	if share.SingleUse {
		// Revoke it immediately
		database.GetCollection("document_shares").UpdateOne(r.Context(), bson.M{"token": token}, bson.M{"$set": bson.M{"is_revoked": true}})
	}

	var doc models.Document
	err = database.GetCollection("documents").FindOne(r.Context(), bson.M{"id": share.DocumentID}).Decode(&doc)
	if err != nil {
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	// Serve inline for browser preview; X-Content-Type-Options is set by middleware
	absPath, err := filepath.Abs(doc.Filepath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	absUploads, _ := filepath.Abs("./uploads")
	if len(absPath) <= len(absUploads) || absPath[:len(absUploads)] != absUploads {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	LogEvent(doc.WorkspaceID, 0, "share_link_accessed", map[string]interface{}{"document_id": doc.ID, "token_prefix": token[:8]})

	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, absPath)
}

func RevokeShareLink(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	userID, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	docID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid document ID", http.StatusBadRequest)
		return
	}
	token := vars["token"]

	role, _ := r.Context().Value(RoleKey).(string)

	docFilter := bson.M{"id": docID}
	if role != "admin" {
		docFilter["workspace_id"] = workspaceID
	}

	count, err := database.GetCollection("documents").CountDocuments(r.Context(), docFilter)
	if err != nil || count == 0 {
		http.Error(w, "Document not found or access denied", http.StatusNotFound)
		return
	}

	res, err := database.GetCollection("document_shares").UpdateOne(
		r.Context(),
		bson.M{"token": token, "document_id": docID},
		bson.M{"$set": bson.M{"is_revoked": true}},
	)
	if err != nil {
		http.Error(w, "Failed to revoke link", http.StatusInternalServerError)
		return
	}
	if res.MatchedCount == 0 {
		http.Error(w, "Share link not found", http.StatusNotFound)
		return
	}

	LogEvent(workspaceID, userID, "share_link_revoked", map[string]interface{}{"document_id": docID, "token_prefix": token[:8]})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Share link revoked successfully"})
}
