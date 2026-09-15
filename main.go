package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"poneglyph/internal/database"
	"poneglyph/internal/handlers"
	"poneglyph/internal/models"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
)

//go:embed all:public
var embeddedPublic embed.FS

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)
		log.Printf("[HTTP] %s %s from %s -> %d (%v)", r.Method, r.URL.Path, r.RemoteAddr, lrw.statusCode, duration)
	})
}

func main() {
	// Load .env file if present (local development only)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — relying on environment variables")
	}

	// Initialize Auth (loads JWT_SECRET from environment)
	handlers.InitAuth()

	// Initialize Database
	if err := database.ConnectDB(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := database.InitSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	handlers.SeedAdminUser()

	// Router setup
	r := mux.NewRouter()

	// Apply request logging and security headers
	r.Use(RequestLogger)
	r.Use(handlers.SecurityHeaders)

	// Public API routes
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/login", handlers.Login).Methods("POST")
	api.HandleFunc("/upload", handlers.UploadBatch).Methods("POST")
	api.HandleFunc("/documents/upload", handlers.UploadBatch).Methods("POST")

	// Protected routes (Session required)
	protected := api.PathPrefix("/").Subrouter()
	protected.Use(handlers.AuthMiddleware)
	protected.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value(handlers.UserIDKey).(int)
		var user models.User
		err := database.GetCollection("users").FindOne(r.Context(), bson.M{"id": userID}).Decode(&user)
		role := user.Role
		username := user.Username
		if err != nil {
			role = "user"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "role": role, "username": username})
	}).Methods("GET")
	protected.HandleFunc("/logout", handlers.Logout).Methods("POST")
	protected.HandleFunc("/stats", handlers.GetStats).Methods("GET")
	protected.HandleFunc("/jobs", handlers.GetJobs).Methods("GET")
	protected.HandleFunc("/documents", handlers.GetJobs).Methods("GET")
	protected.HandleFunc("/jobs/{id}/files/{filename}", handlers.DownloadJobFile).Methods("GET")

	// Admin routes
	admin := protected.PathPrefix("/admin").Subrouter()
	admin.Use(handlers.AdminMiddleware)
	admin.HandleFunc("/users", handlers.GetUsers).Methods("GET")
	admin.HandleFunc("/users", handlers.CreateUser).Methods("POST")
	admin.HandleFunc("/users/{id}/disable", handlers.DisableUser).Methods("POST")
	admin.HandleFunc("/users/{id}/reset_password", handlers.ResetPassword).Methods("POST")
	admin.HandleFunc("/wipe", handlers.WipeDatabase).Methods("POST")
	admin.HandleFunc("/logs/stream", handlers.StreamLogs).Methods("GET")

	// ServiceWorker cleanup route: automatically kills and unregisters any legacy browser service worker from :8080
	r.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Write([]byte(`self.addEventListener('install', (e) => { self.skipWaiting(); });
self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then((keys) => Promise.all(keys.map((k) => caches.delete(k))))
      .then(() => self.registration.unregister())
  );
});
`))
	}).Methods("GET")

	// Static files & SPA routing
	publicSubFS, _ := fs.Sub(embeddedPublic, "public")
	embeddedFileServer := http.FileServer(http.FS(publicSubFS))

	serveSPA := func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
		cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")
		if cleanPath != "" && cleanPath != "." {
			if publicSubFS != nil {
				if f, err := publicSubFS.Open(cleanPath); err == nil {
					stat, err := f.Stat()
					f.Close()
					if err == nil && !stat.IsDir() {
						embeddedFileServer.ServeHTTP(w, r)
						return
					}
				}
			}
		}

		if publicSubFS != nil {
			if data, err := fs.ReadFile(publicSubFS, "index.html"); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(data)
				return
			}
		}
		http.NotFound(w, r)
	}

	r.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If running in production mode, serve the embedded production SPA
		if os.Getenv("ENV") == "production" {
			serveSPA(w, r)
			return
		}

		// In development: port 8080 is STRICTLY an API backend. No HTML is served.
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"service":   "Poneglyph Backend API",
				"status":    "online",
				"port":      8080,
				"endpoints": "/api/*",
				"frontend":  "http://localhost:5173",
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":    "Endpoint not found on Go API",
			"frontend": "http://localhost:5173",
		})
	}))

	bindAddr := os.Getenv("BIND_ADDR")
	if bindAddr == "" {
		bindAddr = "127.0.0.1:8080"
	}
	log.Printf("Server listening on http://%s", bindAddr)
	if err := http.ListenAndServe(bindAddr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
