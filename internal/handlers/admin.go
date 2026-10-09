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

// validRoles is the complete set of assignable roles.
var validRoles = map[string]bool{
	"worker":    true,
	"admin":     true,
	"developer": true,
}

// roleRank returns a numeric rank so we can enforce hierarchy:
// developer (3) > admin (2) > worker (1)
func roleRank(role string) int {
	switch role {
	case "developer":
		return 3
	case "admin":
		return 2
	case "worker":
		return 1
	default:
		return 0
	}
}

// AdminMiddleware allows access to users with role "admin" OR "developer".
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var user models.User
		err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": userID}).Decode(&user)
		if err != nil || (user.Role != "admin" && user.Role != "developer") {
			http.Error(w, "Forbidden: Admins and Developers only", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), RoleKey, user.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DeveloperMiddleware allows access ONLY to users with role "developer".
func DeveloperMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(UserIDKey).(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var user models.User
		err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": userID}).Decode(&user)
		if err != nil || user.Role != "developer" {
			http.Error(w, "Forbidden: Developers only", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), RoleKey, user.Role)
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
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if len(req.Username) < 3 || len(req.Password) < 6 {
		http.Error(w, "Username or password too short", http.StatusBadRequest)
		return
	}

	// Default role is "worker" if not specified
	if req.Role == "" {
		req.Role = "worker"
	}
	if !validRoles[req.Role] {
		http.Error(w, "Invalid role. Must be one of: worker, admin, developer", http.StatusBadRequest)
		return
	}

	// Enforce hierarchy: you can only create users with a role strictly below yours
	actorID, _ := r.Context().Value(UserIDKey).(int)
	var actorUser models.User
	if err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": actorID}).Decode(&actorUser); err != nil {
		http.Error(w, "Could not verify actor role", http.StatusInternalServerError)
		return
	}
	if roleRank(req.Role) >= roleRank(actorUser.Role) {
		http.Error(w, "Forbidden: you cannot create a user with equal or higher role than your own", http.StatusForbidden)
		return
	}

	hash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	userID, err := database.GetNextSequence("users")
	if err != nil {
		http.Error(w, "Failed to allocate user ID", http.StatusInternalServerError)
		return
	}

	newUser := models.User{
		ID:           userID,
		Username:     req.Username,
		PasswordHash: hash,
		Role:         req.Role,
		AdminID:      &actorID,
		IsDisabled:   false,
		CreatedAt:    time.Now(),
	}

	_, err = database.GetCollection("users").InsertOne(r.Context(), newUser)
	if err != nil {
		http.Error(w, "Failed to create user (username may already exist)", http.StatusConflict)
		return
	}

	LogEvent(actorID, actorID, "user_created", map[string]interface{}{"created_user_id": userID, "username": req.Username, "role": req.Role})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "User created", "id": userID, "role": req.Role})
}

func DisableUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Fetch actor
	actorID, _ := r.Context().Value(UserIDKey).(int)
	var actorUser models.User
	if err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": actorID}).Decode(&actorUser); err != nil {
		http.Error(w, "Could not verify actor role", http.StatusInternalServerError)
		return
	}

	// Fetch target
	var target models.User
	if err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": targetID}).Decode(&target); err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Cannot disable someone with equal or higher role
	if roleRank(target.Role) >= roleRank(actorUser.Role) {
		http.Error(w, "Forbidden: cannot disable a user with equal or higher role", http.StatusForbidden)
		return
	}

	newDisabled := !target.IsDisabled
	_, err = database.GetCollection("users").UpdateOne(
		r.Context(),
		bson.M{"id": targetID},
		bson.M{"$set": bson.M{"is_disabled": newDisabled}},
	)
	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	LogEvent(actorID, actorID, "user_toggled_disable", map[string]interface{}{"target_user_id": targetID, "now_disabled": newDisabled})

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

	// Fetch actor
	actorID, _ := r.Context().Value(UserIDKey).(int)
	var actorUser models.User
	if err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": actorID}).Decode(&actorUser); err != nil {
		http.Error(w, "Could not verify actor role", http.StatusInternalServerError)
		return
	}

	// Fetch target
	var target models.User
	if err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": targetID}).Decode(&target); err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Cannot reset password of someone with equal or higher role
	if roleRank(target.Role) >= roleRank(actorUser.Role) {
		http.Error(w, "Forbidden: cannot reset password of a user with equal or higher role", http.StatusForbidden)
		return
	}

	hash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	res, err := database.GetCollection("users").UpdateOne(
		r.Context(),
		bson.M{"id": targetID},
		bson.M{"$set": bson.M{"password_hash": hash}},
	)
	if err != nil || res.MatchedCount == 0 {
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	LogEvent(actorID, actorID, "user_password_reset", map[string]interface{}{"target_user_id": targetID})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Password reset successfully"})
}

// WipeDatabase deletes all jobs data, audit logs, and removes uploaded job folders from disk.
// DEVELOPER ONLY — enforced by DeveloperMiddleware on the route.
// The caller must supply {"confirmation": "wipe my data"} in the request body.
func WipeDatabase(w http.ResponseWriter, r *http.Request) {
	actorID, ok := r.Context().Value(UserIDKey).(int)
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

	// Defence-in-depth: double-check role inside handler
	var user models.User
	err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": actorID}).Decode(&user)
	if err != nil || user.Role != "developer" {
		http.Error(w, "Forbidden: Only developers can wipe data", http.StatusForbidden)
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

	log.Printf("WipeDatabase: System wiped by developer %d — %d job folders removed from disk", actorID, deletedFolders)
	LogEvent(actorID, actorID, "data_wipe_completed", map[string]interface{}{"folders_deleted": deletedFolders})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":         "All job records and uploaded files have been permanently deleted",
		"folders_deleted": deletedFolders,
	})
}
