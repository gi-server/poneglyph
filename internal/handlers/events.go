package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"docunest/internal/database"
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
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		log.Printf("Failed to marshal log details: %v", err)
		detailsJSON = []byte("{}")
	}

	var dbActorID interface{}
	if actorID == 0 {
		dbActorID = nil // System action, no associated user
	} else {
		dbActorID = actorID
	}

	_, err = database.DB.Exec("INSERT INTO audit_logs (workspace_id, actor_id, action, details) VALUES ($1, $2, $3, $4)", workspaceID, dbActorID, action, string(detailsJSON))
	if err != nil {
		log.Printf("Failed to insert audit log: %v", err)
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

