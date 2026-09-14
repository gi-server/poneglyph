package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"docunest/internal/database"
	"docunest/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetDocuments(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	_, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	role, _ := r.Context().Value(RoleKey).(string)

	filter := bson.M{}
	limit := int64(50)
	if role == "admin" {
		limit = 100
	} else {
		filter["workspace_id"] = workspaceID
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit)

	cursor, err := database.GetCollection("documents").Find(r.Context(), filter, opts)
	if err != nil {
		log.Printf("Failed to fetch documents: %v", err)
		http.Error(w, "Failed to fetch documents", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	var rawDocs []models.Document
	if err := cursor.All(r.Context(), &rawDocs); err != nil {
		http.Error(w, "Failed to parse documents", http.StatusInternalServerError)
		return
	}

	// Fetch customer names for documents with customer_id
	var customerIDs []string
	for _, doc := range rawDocs {
		if doc.CustomerID != nil && *doc.CustomerID != "" {
			customerIDs = append(customerIDs, *doc.CustomerID)
		}
	}

	customerMap := make(map[string]string)
	if len(customerIDs) > 0 {
		custCursor, err := database.GetCollection("customers").Find(r.Context(), bson.M{"id": bson.M{"$in": customerIDs}})
		if err == nil {
			var custs []models.Customer
			if err := custCursor.All(r.Context(), &custs); err == nil {
				for _, c := range custs {
					customerMap[c.ID] = c.Name
				}
			}
			custCursor.Close(r.Context())
		}
	}

	type DocumentWithCustomer struct {
		models.Document
		CustomerName *string `json:"customer_name,omitempty"`
	}

	documents := make([]DocumentWithCustomer, 0, len(rawDocs))
	for _, doc := range rawDocs {
		item := DocumentWithCustomer{Document: doc}
		if doc.CustomerID != nil {
			if name, exists := customerMap[*doc.CustomerID]; exists {
				item.CustomerName = &name
			}
		}
		documents = append(documents, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(documents)
}

// ViewDocument streams a document file to the browser after verifying ownership.
func ViewDocument(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(WorkspaceIDKey).(int)
	userID, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	docIDStr := vars["id"]
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		http.Error(w, "Invalid document ID", http.StatusBadRequest)
		return
	}

	role, _ := r.Context().Value(RoleKey).(string)

	filter := bson.M{"id": docID}
	if role != "admin" {
		filter["workspace_id"] = workspaceID
	}

	var doc models.Document
	err = database.GetCollection("documents").FindOne(r.Context(), filter).Decode(&doc)
	if err != nil {
		http.Error(w, "Document not found or access denied", http.StatusNotFound)
		return
	}

	// Sanitize path: resolve to absolute and ensure it stays within uploads dir
	absPath, err := filepath.Abs(doc.Filepath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	absUploads, err := filepath.Abs("./uploads")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Reject any path that doesn't begin with the uploads directory
	if len(absPath) <= len(absUploads) || absPath[:len(absUploads)] != absUploads {
		log.Printf("Path traversal attempt detected for doc %d: resolved to %s", docID, absPath)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	LogEvent(workspaceID, userID, "document_viewed", map[string]interface{}{"document_id": docID})

	// Serve inline for browser preview; X-Content-Type-Options is set by middleware
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, absPath)
}
