package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"poneglyph/internal/database"
	"poneglyph/internal/models"

	"github.com/alexedwards/argon2id"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var user models.User
		err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": userID}).Decode(&user)
		if err != nil || user.Role != "admin" {
			http.Error(w, "Forbidden: Admins only", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), "role", user.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUsers(w http.ResponseWriter, r *http.Request) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := database.GetCollection("users").Find(r.Context(), bson.M{}, opts)
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	users := []models.User{}
	if err := cursor.All(r.Context(), &users); err != nil {
		http.Error(w, "Failed to parse users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if len(req.Username) < 3 || len(req.Password) < 6 {
		http.Error(w, "Username or password too short", http.StatusBadRequest)
		return
	}

	hash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	adminID, _ := r.Context().Value(UserIDKey).(int)

	userID, err := database.GetNextSequence("users")
	if err != nil {
		http.Error(w, "Failed to allocate user ID", http.StatusInternalServerError)
		return
	}

	newUser := models.User{
		ID:           userID,
		Username:     req.Username,
		PasswordHash: hash,
		Role:         "user",
		AdminID:      &adminID,
		IsDisabled:   false,
		CreatedAt:    time.Now(),
	}

	_, err = database.GetCollection("users").InsertOne(r.Context(), newUser)
	if err != nil {
		http.Error(w, "Failed to create user (username may already exist)", http.StatusConflict)
		return
	}

	LogEvent(adminID, adminID, "user_created", map[string]interface{}{"created_user_id": userID, "username": req.Username})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "User created", "id": userID})
}

func DisableUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var user models.User
	err = database.GetCollection("users").FindOne(r.Context(), bson.M{"id": targetID, "role": bson.M{"$ne": "admin"}}).Decode(&user)
	if err != nil {
		http.Error(w, "User not found or cannot disable an admin", http.StatusForbidden)
		return
	}

	newDisabled := !user.IsDisabled
	_, err = database.GetCollection("users").UpdateOne(
		r.Context(),
		bson.M{"id": targetID},
		bson.M{"$set": bson.M{"is_disabled": newDisabled}},
	)
	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	adminID, _ := r.Context().Value(UserIDKey).(int)
	LogEvent(adminID, adminID, "user_toggled_disable", map[string]interface{}{"target_user_id": targetID, "now_disabled": newDisabled})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "User status updated", "is_disabled": newDisabled})
}

func ResetPassword(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Password) < 6 {
		http.Error(w, "Invalid payload or password too short", http.StatusBadRequest)
		return
	}

	hash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	res, err := database.GetCollection("users").UpdateOne(
		r.Context(),
		bson.M{"id": targetID, "role": bson.M{"$ne": "admin"}},
		bson.M{"$set": bson.M{"password_hash": hash}},
	)
	if err != nil || res.MatchedCount == 0 {
		http.Error(w, "User not found or cannot reset admin password", http.StatusForbidden)
		return
	}

	adminID, _ := r.Context().Value(UserIDKey).(int)
	LogEvent(adminID, adminID, "user_password_reset", map[string]interface{}{"target_user_id": targetID})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Password reset successfully"})
}

// WipeDatabase deletes all jobs data, audit logs, and removes uploaded job folders from disk.
// The caller must supply {"confirmation": "wipe my data"} in the request body.
func WipeDatabase(w http.ResponseWriter, r *http.Request) {
	adminID, ok := r.Context().Value(UserIDKey).(int)
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

	// Ensure this is an admin
	var user models.User
	err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": adminID}).Decode(&user)
	if err != nil || user.Role != "admin" {
		http.Error(w, "Forbidden: Only admins can wipe data", http.StatusForbidden)
		return
	}

	ctx := r.Context()
	// 1. Delete MongoDB collections data
	if database.DB != nil {
		database.GetCollection("jobs").DeleteMany(ctx, bson.M{})
		database.GetCollection("audit_logs").DeleteMany(ctx, bson.M{})
	}

	// 2. Clean uploads directory
	uploadBaseDir := os.Getenv("UPLOAD_DIR")
	if uploadBaseDir == "" {
		uploadBaseDir = "./uploads"
	}

	deletedFolders := 0
	if entries, err := os.ReadDir(uploadBaseDir); err == nil {
		for _, entry := range entries {
			fullPath := filepath.Join(uploadBaseDir, entry.Name())
			if err := os.RemoveAll(fullPath); err == nil {
				deletedFolders++
			} else {
				log.Printf("WipeDatabase: could not remove %s: %v", fullPath, err)
			}
		}
	}

	log.Printf("WipeDatabase: System wiped by admin %d — %d job folders removed from disk", adminID, deletedFolders)
	LogEvent(adminID, adminID, "data_wipe_completed", map[string]interface{}{"folders_deleted": deletedFolders})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":         "All job records and uploaded files have been permanently deleted",
		"folders_deleted": deletedFolders,
	})
}
