package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"docunest/internal/database"
	"docunest/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetCustomers(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	_, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	role, _ := r.Context().Value(RoleKey).(string)

	filter := bson.M{}
	if role != "admin" {
		filter["workspace_id"] = workspaceID
	}

	if query != "" {
		if len(query) > 100 {
			query = query[:100]
		}
		filter["name"] = bson.M{"$regex": primitive.Regex{Pattern: query, Options: "i"}}
	}

	limit := int64(50)
	if role == "admin" {
		limit = 100
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "name", Value: 1}}).
		SetLimit(limit)

	cursor, err := database.GetCollection("customers").Find(r.Context(), filter, opts)
	if err != nil {
		log.Printf("Failed to query customers: %v", err)
		http.Error(w, "Failed to query customers", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	customers := []models.Customer{}
	if err := cursor.All(r.Context(), &customers); err != nil {
		http.Error(w, "Failed to scan customers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

func GetCustomerDocuments(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	_, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	customerID := vars["id"]

	// Validate customer ID length
	if len(customerID) == 0 || len(customerID) > 50 {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	role, _ := r.Context().Value(RoleKey).(string)

	// Verify customer belongs to the authenticated user (IDOR protection) if not admin
	if role != "admin" {
		count, err := database.GetCollection("customers").CountDocuments(r.Context(), bson.M{
			"id":           customerID,
			"workspace_id": workspaceID,
		})
		if err != nil || count == 0 {
			http.Error(w, "Customer not found or access denied", http.StatusNotFound)
			return
		}
	}

	docFilter := bson.M{"customer_id": customerID}
	if role != "admin" {
		docFilter["workspace_id"] = workspaceID
	}

	docOpts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(100)

	cursor, err := database.GetCollection("documents").Find(r.Context(), docFilter, docOpts)
	if err != nil {
		log.Printf("Failed to fetch customer documents: %v", err)
		http.Error(w, "Failed to fetch documents", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	documents := []models.Document{}
	if err := cursor.All(r.Context(), &documents); err != nil {
		http.Error(w, "Failed to scan documents", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(documents)
}
