package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"poneglyph/internal/database"
	"poneglyph/internal/models"
)

type SSELogEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	WorkspaceID int                    `json:"workspace_id"`
	ActorID     int                    `json:"actor_id"`
	Action      string                 `json:"action"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

var (
	logClients = make(map[chan SSELogEvent]int) // maps client channel to workspace_id
	logMutex   sync.Mutex
)

func LogEvent(workspaceID int, actorID int, action string, details map[string]interface{}) {
	var dbActorID *int
	if actorID != 0 {
		dbActorID = &actorID
	}

	var docID *int
	if details != nil {
		if dID, ok := details["document_id"].(int); ok {
			docID = &dID
		}
	}

	logID, err := database.GetNextSequence("audit_logs")
	if err == nil {
		auditRecord := models.AuditLog{
			ID:          logID,
			WorkspaceID: workspaceID,
			ActorID:     dbActorID,
			DocumentID:  docID,
			Action:      action,
			Details:     details,
			CreatedAt:   time.Now(),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = database.GetCollection("audit_logs").InsertOne(ctx, auditRecord)
		if err != nil {
			log.Printf("Failed to insert audit log: %v", err)
		}
	}

	event := SSELogEvent{
		Timestamp:   time.Now(),
		WorkspaceID: workspaceID,
		ActorID:     actorID,
		Action:      action,
		Details:     details,
	}

	logMutex.Lock()
	for client, clientWorkspaceID := range logClients {
		if clientWorkspaceID == workspaceID {
			select {
			case client <- event:
			default:
			}
		}
	}
	logMutex.Unlock()
}

func StreamLogs(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := r.Context().Value(WorkspaceIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientChan := make(chan SSELogEvent, 100)

	logMutex.Lock()
	logClients[clientChan] = workspaceID
	logMutex.Unlock()

	defer func() {
		logMutex.Lock()
		delete(logClients, clientChan)
		close(clientChan)
		logMutex.Unlock()
	}()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case event := <-clientChan:
			eventData, err := json.Marshal(event)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", eventData)
				w.Header().Set("X-Accel-Buffering", "no")
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
	}
}
