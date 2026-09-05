package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"docunest/internal/database"
	"docunest/internal/models"

	"github.com/gorilla/mux"
)

// resolveCustomerName extracts a suitable customer display name from extracted_data.
// It checks for the most likely entity name field depending on document type:
// person_name (identity docs), vendor (invoices), merchant (receipts).
func resolveCustomerName(extractedData map[string]interface{}) string {
	for _, key := range []string{"person_name", "vendor", "merchant"} {
		if v, ok := extractedData[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func ConfirmDocument(w http.ResponseWriter, r *http.Request) {
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

	// Verify the document belongs to this user and is pending review
	var currentStatus string
	var errDB error
	if role == "admin" {
		errDB = database.DB.QueryRow(
			"SELECT status FROM documents WHERE id = $1",
			docID,
		).Scan(&currentStatus)
	} else {
		errDB = database.DB.QueryRow(
			"SELECT status FROM documents WHERE id = $1 AND workspace_id = $2",
			docID, workspaceID,
		).Scan(&currentStatus)
	}
	if errDB != nil {
		http.Error(w, "Document not found or access denied", http.StatusNotFound)
		return
	}
	if currentStatus != "needs_review" {
		http.Error(w, "Document is not pending review", http.StatusBadRequest)
		return
	}

	// Limit body size to prevent large payload attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1<<15) // 32 KB max
	var req models.ReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Input validation
	if len(req.DocumentType) == 0 || len(req.DocumentType) > 100 {
		http.Error(w, "Document type must be between 1 and 100 characters", http.StatusBadRequest)
		return
	}
	if len(req.CustomerID) > 50 {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	// Resolve extracted_data: prefer canonical ExtractedData, fall back to legacy fields
	var extractedMap map[string]interface{}
	if len(req.ExtractedData) > 0 {
		if err := json.Unmarshal(req.ExtractedData, &extractedMap); err != nil {
			http.Error(w, "Invalid extracted_data JSON", http.StatusBadRequest)
			return
		}
	} else {
		// Legacy fallback: build extracted_data from old fields
		extractedMap = make(map[string]interface{})
		if req.PersonName != "" {
			extractedMap["person_name"] = req.PersonName
		}
		if req.DOB != "" {
			extractedMap["dob"] = req.DOB
		}
		if req.DocumentIDNumber != "" {
			extractedMap["document_id_number"] = req.DocumentIDNumber
		}
	}

	extractedJSON, err := json.Marshal(extractedMap)
	if err != nil {
		http.Error(w, "Failed to serialize extracted data", http.StatusInternalServerError)
		return
	}

	// Project legacy columns from extracted_data (single source of truth)
	var personName, dob, docIDNumber *string
	if v, ok := extractedMap["person_name"].(string); ok {
		personName = &v
	}
	if v, ok := extractedMap["dob"].(string); ok {
		dob = &v
	}
	if v, ok := extractedMap["document_id_number"].(string); ok {
		docIDNumber = &v
	}

	// Determine customer name for new customer creation
	customerName := resolveCustomerName(extractedMap)
	if customerName == "" {
		customerName = "Customer"
	}

	finalCustomerID := req.CustomerID

	if finalCustomerID == "new" || finalCustomerID == "" {
		// Create a new customer using a cryptographic UUID from the DB serial
		// The name is derived from extracted_data — user has already verified it
		var newID string
		err = database.DB.QueryRow(
			"INSERT INTO customers (id, workspace_id, name) VALUES (gen_random_uuid()::text, $1, $2) RETURNING id",
			workspaceID, customerName,
		).Scan(&newID)
		if err != nil {
			log.Printf("Failed to create new customer: %v", err)
			http.Error(w, "Failed to create new customer", http.StatusInternalServerError)
			return
		}
		finalCustomerID = newID
	} else if role != "admin" {
		// IDOR check: ensure the provided customer_id belongs to this user
		var exists bool
		err = database.DB.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM customers WHERE id = $1 AND workspace_id = $2)",
			finalCustomerID, workspaceID,
		).Scan(&exists)
		if err != nil || !exists {
			http.Error(w, "Customer not found or access denied", http.StatusForbidden)
			return
		}
	}

	if role == "admin" {
		_, err = database.DB.Exec(`
			UPDATE documents 
			SET document_type = $1, extracted_data = $2, person_name = $3, dob = $4, document_id_number = $5, customer_id = $6, status = 'completed'
			WHERE id = $7
		`, req.DocumentType, extractedJSON, personName, dob, docIDNumber, finalCustomerID, docID)
	} else {
		_, err = database.DB.Exec(`
			UPDATE documents 
			SET document_type = $1, extracted_data = $2, person_name = $3, dob = $4, document_id_number = $5, customer_id = $6, status = 'completed'
			WHERE id = $7 AND workspace_id = $8
		`, req.DocumentType, extractedJSON, personName, dob, docIDNumber, finalCustomerID, docID, workspaceID)
	}
	if err != nil {
		log.Printf("Failed to update document %d: %v", docID, err)
		http.Error(w, "Failed to update document", http.StatusInternalServerError)
		return
	}

	// Audit log — record the human review action with canonical extracted_data
	LogEvent(workspaceID, userID, "confirm_ai_review", map[string]interface{}{
		"document_id":    docID,
		"document_type":  req.DocumentType,
		"extracted_data": extractedMap,
		"customer_id":    finalCustomerID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Document reviewed and completed successfully"})
}
