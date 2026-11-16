package database

import (
	"database/sql"
	"fmt"
)

func InitSchema(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		payload TEXT NOT NULL DEFAULT '{}',

		status TEXT NOT NULL DEFAULT 'pending'
			CHECK (status IN (
				'pending',
				'running',
				'completed',
				'failed',
				'cancelled'
			)),

		scheduled_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,

		started_at INTEGER,
		completed_at INTEGER,

		attempts INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 3,
		last_error TEXT NOT NULL DEFAULT '',

		recurring_interval_seconds INTEGER NOT NULL DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_status_scheduled
	ON jobs(status, scheduled_at);

	CREATE INDEX IF NOT EXISTS idx_jobs_type
	ON jobs(type);

	CREATE TABLE IF NOT EXISTS job_attempts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id INTEGER NOT NULL,
		attempt_number INTEGER NOT NULL,
		started_at INTEGER NOT NULL,
		finished_at INTEGER,
		status TEXT NOT NULL,
		error TEXT NOT NULL DEFAULT '',

		FOREIGN KEY(job_id) REFERENCES jobs(id)
			ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_job_attempts_job_id
	ON job_attempts(job_id);
	`

	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("create database schema: %w", err)
	}

	return nil
}
