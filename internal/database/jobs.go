package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"background-job-scheduler/internal/models"
)

func CreateJob(db *sql.DB, job *models.Job) error {
	now := time.Now().UTC()

	if len(job.Payload) == 0 {
		job.Payload = json.RawMessage(`{}`)
	}

	if !json.Valid(job.Payload) {
		return fmt.Errorf("job payload must be valid JSON")
	}

	if job.Status == "" {
		job.Status = models.StatusPending
	}

	if job.ScheduledAt.IsZero() {
		job.ScheduledAt = now
	}

	if job.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	job.CreatedAt = now
	job.UpdatedAt = now

	result, err := db.Exec(`
		INSERT INTO jobs (
			name,
			type,
			payload,
			status,
			scheduled_at,
			created_at,
			updated_at,
			attempts,
			max_retries,
			last_error,
			recurring_interval_seconds
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		job.Name,
		job.Type,
		string(job.Payload),
		string(job.Status),
		job.ScheduledAt.UnixMilli(),
		job.CreatedAt.UnixMilli(),
		job.UpdatedAt.UnixMilli(),
		job.Attempts,
		job.MaxRetries,
		job.LastError,
		job.RecurringIntervalSeconds,
	)
	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	job.ID, err = result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get inserted job ID: %w", err)
	}

	return nil
}

func GetJob(db *sql.DB, id int64) (*models.Job, error) {
	row := db.QueryRow(`
		SELECT
			id, name, type, payload, status,
			scheduled_at, created_at, updated_at,
			started_at, completed_at,
			attempts, max_retries, last_error,
			recurring_interval_seconds
		FROM jobs
		WHERE id = ?
	`, id)

	job, err := scanJob(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job %d not found", id)
		}
		return nil, err
	}

	return job, nil
}

func ListJobs(db *sql.DB, status models.JobStatus) ([]models.Job, error) {
	query := `
		SELECT
			id, name, type, payload, status,
			scheduled_at, created_at, updated_at,
			started_at, completed_at,
			attempts, max_retries, last_error,
			recurring_interval_seconds
		FROM jobs
	`

	var rows *sql.Rows
	var err error

	if status != "" {
		query += " WHERE status = ?"
		query += " ORDER BY created_at DESC"
		rows, err = db.Query(query, string(status))
	} else {
		query += " ORDER BY created_at DESC"
		rows, err = db.Query(query)
	}

	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]models.Job, 0)

	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (*models.Job, error) {
	var job models.Job
	var payload string
	var status string

	var scheduledAt int64
	var createdAt int64
	var updatedAt int64
	var startedAt sql.NullInt64
	var completedAt sql.NullInt64

	err := row.Scan(
		&job.ID,
		&job.Name,
		&job.Type,
		&payload,
		&status,
		&scheduledAt,
		&createdAt,
		&updatedAt,
		&startedAt,
		&completedAt,
		&job.Attempts,
		&job.MaxRetries,
		&job.LastError,
		&job.RecurringIntervalSeconds,
	)
	if err != nil {
		return nil, fmt.Errorf("scan job: %w", err)
	}

	job.Payload = json.RawMessage(payload)
	job.Status = models.JobStatus(status)
	job.ScheduledAt = time.UnixMilli(scheduledAt).UTC()
	job.CreatedAt = time.UnixMilli(createdAt).UTC()
	job.UpdatedAt = time.UnixMilli(updatedAt).UTC()

	if startedAt.Valid {
		value := time.UnixMilli(startedAt.Int64).UTC()
		job.StartedAt = &value
	}

	if completedAt.Valid {
		value := time.UnixMilli(completedAt.Int64).UTC()
		job.CompletedAt = &value
	}

	return &job, nil
}
