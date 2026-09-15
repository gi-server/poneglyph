package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"poneglyph/internal/database"

	"go.mongodb.org/mongo-driver/bson"
)

type StatsResponse struct {
	TotalJobs      int64 `json:"total_jobs"`
	TotalFiles     int64 `json:"total_files"`
	TotalBytes     int64 `json:"total_bytes"`
	TotalUsers     int64 `json:"total_users"`
	ProcessedToday int64 `json:"processed_today"`
}

func GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var stats StatsResponse

	if database.DB == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
		return
	}

	jobsColl := database.GetCollection("jobs")
	usersColl := database.GetCollection("users")

	totalJobs, _ := jobsColl.CountDocuments(ctx, bson.M{})
	stats.TotalJobs = totalJobs

	totalUsers, _ := usersColl.CountDocuments(ctx, bson.M{})
	stats.TotalUsers = totalUsers

	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	processedToday, _ := jobsColl.CountDocuments(ctx, bson.M{"uploaded_at": bson.M{"$gte": startOfDay}})
	stats.ProcessedToday = processedToday

	// Compute total files & total bytes across jobs
	cursor, err := jobsColl.Find(ctx, bson.M{})
	if err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var job struct {
				Files []struct {
					Size int64 `bson:"size"`
				} `bson:"files"`
			}
			if err := cursor.Decode(&job); err == nil {
				stats.TotalFiles += int64(len(job.Files))
				for _, f := range job.Files {
					stats.TotalBytes += f.Size
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
