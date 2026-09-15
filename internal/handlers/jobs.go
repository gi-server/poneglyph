package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"poneglyph/internal/database"
	"poneglyph/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetJobs returns a list of recent uploaded jobs.
func GetJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobs := []models.Job{}

	if database.DB != nil {
		opts := options.Find().SetSort(bson.D{{Key: "uploaded_at", Value: -1}}).SetLimit(100)
		cursor, err := database.GetCollection("jobs").Find(ctx, bson.M{}, opts)
		if err == nil {
			defer cursor.Close(ctx)
			if err := cursor.All(ctx, &jobs); err != nil {
				jobs = []models.Job{}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// DownloadJobFile serves an uploaded document from disk.
func DownloadJobFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]
	filename := vars["filename"]

	// Security: prevent directory traversal
	cleanFilename := filepath.Base(filename)
	if cleanFilename == "." || cleanFilename == "/" || strings.Contains(jobID, "..") || strings.Contains(jobID, "/") || strings.Contains(jobID, "\\") {
		http.Error(w, "Invalid path parameters", http.StatusBadRequest)
		return
	}

	uploadBaseDir := os.Getenv("UPLOAD_DIR")
	if uploadBaseDir == "" {
		uploadBaseDir = "./uploads"
	}

	filePath := filepath.Join(uploadBaseDir, jobID, cleanFilename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "inline; filename=\""+cleanFilename+"\"")
	http.ServeFile(w, r, filePath)
}
