package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"background-job-scheduler/internal/database"
	"background-job-scheduler/internal/models"
)

type JobHandler struct {
	DB *sql.DB
}

type createJobRequest struct {
	Name                     string          `json:"name"`
	Type                     string          `json:"type"`
	Payload                  json.RawMessage `json:"payload"`
	ScheduledAt              *time.Time      `json:"scheduled_at"`
	MaxRetries               *int            `json:"max_retries"`
	RecurringIntervalSeconds int64           `json:"recurring_interval_seconds"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewJobHandler(db *sql.DB) *JobHandler {
	return &JobHandler{DB: db}
}

func (h *JobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var request createJobRequest

	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Type = strings.TrimSpace(request.Type)

	if request.Name == "" {
		writeError(w, http.StatusBadRequest, "job name is required")
		return
	}

	if request.Type == "" {
		writeError(w, http.StatusBadRequest, "job type is required")
		return
	}

	if len(request.Payload) == 0 {
		request.Payload = json.RawMessage(`{}`)
	}

	if !json.Valid(request.Payload) {
		writeError(w, http.StatusBadRequest, "payload must be valid JSON")
		return
	}

	maxRetries := 3
	if request.MaxRetries != nil {
		maxRetries = *request.MaxRetries
	}

	if maxRetries < 0 || maxRetries > 20 {
		writeError(w, http.StatusBadRequest, "max_retries must be between 0 and 20")
		return
	}

	if request.RecurringIntervalSeconds < 0 {
		writeError(w, http.StatusBadRequest, "recurring interval cannot be negative")
		return
	}

	scheduledAt := time.Now().UTC()
	if request.ScheduledAt != nil {
		scheduledAt = request.ScheduledAt.UTC()
	}

	job := &models.Job{
		Name:                     request.Name,
		Type:                     request.Type,
		Payload:                  request.Payload,
		Status:                   models.StatusPending,
		ScheduledAt:              scheduledAt,
		MaxRetries:               maxRetries,
		RecurringIntervalSeconds: request.RecurringIntervalSeconds,
	}

	if err := database.CreateJob(h.DB, job); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create job")
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	statusValue := strings.TrimSpace(r.URL.Query().Get("status"))

	var status models.JobStatus

	if statusValue != "" {
		status = models.JobStatus(statusValue)

		switch status {
		case models.StatusPending,
			models.StatusRunning,
			models.StatusCompleted,
			models.StatusFailed,
			models.StatusCancelled:
		default:
			writeError(w, http.StatusBadRequest, "invalid job status")
			return
		}
	}

	jobs, err := database.ListJobs(h.DB, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not retrieve jobs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(jobs),
		"jobs":  jobs,
	})
}

func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := database.GetJob(h.DB, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}

		writeError(w, http.StatusInternalServerError, "could not retrieve job")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (h *JobHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.DB.Exec(`
		UPDATE jobs
		SET status = ?, updated_at = ?
		WHERE id = ? AND status = ?
	`,
		string(models.StatusCancelled),
		time.Now().UTC().UnixMilli(),
		id,
		string(models.StatusPending),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not cancel job")
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not confirm cancellation")
		return
	}

	if affected == 0 {
		job, err := database.GetJob(h.DB, id)
		if err != nil {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}

		writeError(
			w,
			http.StatusConflict,
			fmt.Sprintf("job cannot be cancelled because its status is %q", job.Status),
		)
		return
	}

	job, err := database.GetJob(h.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "job cancelled but could not retrieve it")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func parseJobID(r *http.Request) (int64, error) {
	rawID := r.PathValue("id")
	if rawID == "" {
		return 0, errors.New("job ID is required")
	}

	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("job ID must be a positive integer")
	}

	return id, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request must contain only one JSON object")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// The response has already been written; nothing else can be sent.
		return
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
