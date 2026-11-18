package scheduler

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"background-job-scheduler/internal/database"
	"background-job-scheduler/internal/models"
)

func TestClaimDueJobs(t *testing.T) {
	// Create a temporary database for this test.
	dbPath := filepath.Join(t.TempDir(), "scheduler_test.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// Initialize the database schema.
	if err := database.InitSchema(db); err != nil {
		t.Fatalf("initialize schema: %v", err)
	}

	// Create a job whose scheduled time is in the past.
	dueJob := &models.Job{
		Name:        "Due test job",
		Type:        "email",
		Payload:     json.RawMessage(`{"to":"test@example.com"}`),
		Status:      models.StatusPending,
		ScheduledAt: time.Now().UTC().Add(-time.Minute),
		MaxRetries:  3,
	}

	// Create a job whose scheduled time is in the future.
	futureJob := &models.Job{
		Name:        "Future test job",
		Type:        "email",
		Payload:     json.RawMessage(`{"to":"future@example.com"}`),
		Status:      models.StatusPending,
		ScheduledAt: time.Now().UTC().Add(time.Hour),
		MaxRetries:  3,
	}

	// Save both jobs in the database.
	if err := database.CreateJob(db, dueJob); err != nil {
		t.Fatalf("create due job: %v", err)
	}

	if err := database.CreateJob(db, futureJob); err != nil {
		t.Fatalf("create future job: %v", err)
	}

	// Create the scheduler engine.
	engine := New(db, time.Second, 10)

	// Claim jobs that are due.
	claimed, err := engine.claimDueJobs(context.Background())
	if err != nil {
		t.Fatalf("claim due jobs: %v", err)
	}

	// Only the due job should have been claimed.
	if len(claimed) != 1 {
		t.Fatalf("expected 1 claimed job, got %d", len(claimed))
	}

	// Confirm that the correct job was claimed.
	if claimed[0].ID != dueJob.ID {
		t.Errorf("expected job ID %d, got %d", dueJob.ID, claimed[0].ID)
	}

	// Confirm that the job status changed to running.
	if claimed[0].Status != models.StatusRunning {
		t.Errorf("expected running status, got %q", claimed[0].Status)
	}

	// Confirm that the job has a start time.
	if claimed[0].StartedAt == nil {
		t.Error("expected started_at to be set")
	}

	// Try claiming jobs again.
	claimedAgain, err := engine.claimDueJobs(context.Background())
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}

	// The already claimed job should not be claimed again.
	if len(claimedAgain) != 0 {
		t.Errorf("expected 0 jobs on second claim, got %d", len(claimedAgain))
	}

	// Retrieve the future job from the database.
	savedFutureJob, err := database.GetJob(db, futureJob.ID)
	if err != nil {
		t.Fatalf("retrieve future job: %v", err)
	}

	// Confirm that the future job remains pending.
	if savedFutureJob.Status != models.StatusPending {
		t.Errorf(
			"expected future job to remain pending, got %q",
			savedFutureJob.Status,
		)
	}
}
