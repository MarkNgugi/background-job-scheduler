package database_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"background-job-scheduler/internal/database"
	"background-job-scheduler/internal/models"
)

func TestCreateGetAndListJob(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := database.InitSchema(db); err != nil {
		t.Fatalf("initialize schema: %v", err)
	}

	job := &models.Job{
		Name:        "Test email job",
		Type:        "email",
		Payload:     json.RawMessage(`{"to":"test@example.com"}`),
		Status:      models.StatusPending,
		ScheduledAt: time.Now().UTC().Add(time.Minute),
		MaxRetries:  3,
	}

	if err := database.CreateJob(db, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	if job.ID == 0 {
		t.Fatal("expected database to assign a job ID")
	}

	savedJob, err := database.GetJob(db, job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}

	if savedJob.Name != job.Name {
		t.Errorf("expected job name %q, got %q", job.Name, savedJob.Name)
	}

	if savedJob.Type != job.Type {
		t.Errorf("expected job type %q, got %q", job.Type, savedJob.Type)
	}

	if savedJob.Status != models.StatusPending {
		t.Errorf("expected pending status, got %q", savedJob.Status)
	}

	jobs, err := database.ListJobs(db, "")
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	pendingJobs, err := database.ListJobs(db, models.StatusPending)
	if err != nil {
		t.Fatalf("list pending jobs: %v", err)
	}

	if len(pendingJobs) != 1 {
		t.Errorf("expected 1 pending job, got %d", len(pendingJobs))
	}
}
