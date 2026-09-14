package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"docunest/internal/database"
	"docunest/internal/models"
	"docunest/internal/storage"

	"go.mongodb.org/mongo-driver/bson"
)

func GetStats(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	_, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var stats struct {
		TotalDocuments int `json:"total_documents"`
		TotalCustomers int `json:"total_customers"`
		ProcessedToday int `json:"processed_today"`
	}

	role, _ := r.Context().Value(RoleKey).(string)
	isAdmin := (role == "admin")

	docsColl := database.GetCollection("documents")
	custColl := database.GetCollection("customers")

	docFilter := bson.M{}
	custFilter := bson.M{}
	if !isAdmin {
		docFilter["workspace_id"] = workspaceID
		custFilter["workspace_id"] = workspaceID
	}

	totalDocs, err := docsColl.CountDocuments(r.Context(), docFilter)
	if err != nil {
		http.Error(w, "Failed to get total documents", http.StatusInternalServerError)
		return
	}
	stats.TotalDocuments = int(totalDocs)

	totalCusts, err := custColl.CountDocuments(r.Context(), custFilter)
	if err != nil {
		http.Error(w, "Failed to get total customers", http.StatusInternalServerError)
		return
	}
	stats.TotalCustomers = int(totalCusts)

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayFilter := bson.M{"created_at": bson.M{"$gte": startOfDay}}
	if !isAdmin {
		todayFilter["workspace_id"] = workspaceID
	}

	processedToday, err := docsColl.CountDocuments(r.Context(), todayFilter)
	if err != nil {
		http.Error(w, "Failed to get today's documents", http.StatusInternalServerError)
		return
	}
	stats.ProcessedToday = int(processedToday)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// WipeDatabase deletes all data and removes uploaded files from disk.
// The caller must supply {"confirmation": "wipe my data"} in the request body.
func WipeDatabase(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	userID, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Require an explicit confirmation phrase in the body
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	var body struct {
		Confirmation string `json:"confirmation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	if body.Confirmation != "wipe my data" {
		http.Error(w, "Confirmation phrase did not match", http.StatusBadRequest)
		return
	}

	// Ensure this is an admin!
	var user models.User
	err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": userID}).Decode(&user)
	if err != nil || user.Role != "admin" {
		http.Error(w, "Forbidden: Only admins can wipe data", http.StatusForbidden)
		return
	}

	LogEvent(workspaceID, userID, "data_wipe_started", map[string]interface{}{"action": "wipe my data"})

	// 1. Collect all file paths before deleting DB records
	docsColl := database.GetCollection("documents")
	cursor, err := docsColl.Find(r.Context(), bson.M{})
	if err != nil {
		log.Printf("WipeDatabase: failed to fetch documents: %v", err)
		http.Error(w, "Failed to initiate wipe", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	var filePaths []string
	for cursor.Next(r.Context()) {
		var doc models.Document
		if err := cursor.Decode(&doc); err == nil && doc.Filepath != "" {
			filePaths = append(filePaths, doc.Filepath)
		}
	}

	// 2. Delete collection records
	ctx := r.Context()
	database.GetCollection("document_shares").DeleteMany(ctx, bson.M{})
	database.GetCollection("audit_logs").DeleteMany(ctx, bson.M{})
	database.GetCollection("documents").DeleteMany(ctx, bson.M{})
	database.GetCollection("customers").DeleteMany(ctx, bson.M{})

	// 3. Delete files from disk
	absUploads, _ := filepath.Abs(storage.UploadDir)
	deleted, skipped := 0, 0
	for _, fp := range filePaths {
		abs, err := filepath.Abs(fp)
		if err != nil || len(abs) <= len(absUploads) || abs[:len(absUploads)] != absUploads {
			skipped++
			continue
		}
		if err := os.Remove(abs); err != nil {
			log.Printf("WipeDatabase: could not remove file %s: %v", abs, err)
			skipped++
		} else {
			deleted++
		}
	}

	log.Printf("WipeDatabase: System wiped — %d files deleted, %d skipped", deleted, skipped)
	LogEvent(workspaceID, userID, "data_wipe_completed", map[string]interface{}{"files_deleted": deleted})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "All data has been permanently deleted",
		"files_deleted": deleted,
		"files_skipped": skipped,
	})
}
