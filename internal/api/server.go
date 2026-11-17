package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"background-job-scheduler/internal/handlers"
)

func NewServer(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	jobHandler := handlers.NewJobHandler(db)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unhealthy",
				"error":  "database unavailable",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "healthy",
			"service": "background-job-scheduler",
		})
	})

	mux.HandleFunc("POST /api/v1/jobs", jobHandler.CreateJob)
	mux.HandleFunc("GET /api/v1/jobs", jobHandler.ListJobs)
	mux.HandleFunc("GET /api/v1/jobs/{id}", jobHandler.GetJob)
	mux.HandleFunc("DELETE /api/v1/jobs/{id}", jobHandler.CancelJob)

	return mux
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
