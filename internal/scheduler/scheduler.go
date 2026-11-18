package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"background-job-scheduler/internal/database"
	"background-job-scheduler/internal/models"
)

type Scheduler struct {
	DB        *sql.DB
	Interval  time.Duration
	BatchSize int
	Queue     chan models.Job
}

func New(db *sql.DB, interval time.Duration, batchSize int) *Scheduler {
	if interval <= 0 {
		interval = time.Second
	}

	if batchSize <= 0 {
		batchSize = 10
	}

	return &Scheduler{
		DB:        db,
		Interval:  interval,
		BatchSize: batchSize,
		Queue:     make(chan models.Job, batchSize),
	}
}

func (s *Scheduler) Jobs() <-chan models.Job {
	return s.Queue
}

func (s *Scheduler) Start(ctx context.Context) {
	log.Println("Scheduler engine started")
	log.Printf("Polling interval: %s", s.Interval)
	log.Printf("Batch size: %d", s.BatchSize)

	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	// Check immediately instead of waiting for the first tick.
	s.dispatchDueJobs(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler engine stopped")
			return

		case <-ticker.C:
			s.dispatchDueJobs(ctx)
		}
	}
}

func (s *Scheduler) dispatchDueJobs(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	jobs, err := s.claimDueJobs(ctx)
	if err != nil {
		log.Printf("Scheduler error: %v", err)
		return
	}

	for _, job := range jobs {
		select {
		case s.Queue <- job:
			log.Printf("Job %d dispatched to queue", job.ID)

		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) claimDueJobs(ctx context.Context) ([]models.Job, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().UnixMilli()

	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM jobs
		WHERE status = ?
		  AND scheduled_at <= ?
		ORDER BY scheduled_at ASC, id ASC
		LIMIT ?
	`,
		string(models.StatusPending),
		now,
		s.BatchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("find due jobs: %w", err)
	}

	var ids []int64

	for rows.Next() {
		var id int64

		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("read due job ID: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate due jobs: %w", err)
	}

	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close due jobs rows: %w", err)
	}

	if len(ids) == 0 {
		return []models.Job{}, nil
	}

	claimedIDs := make([]int64, 0, len(ids))

	for _, id := range ids {
		result, err := tx.ExecContext(ctx, `
			UPDATE jobs
			SET status = ?,
			    started_at = ?,
			    updated_at = ?
			WHERE id = ?
			  AND status = ?
			  AND scheduled_at <= ?
		`,
			string(models.StatusRunning),
			now,
			now,
			id,
			string(models.StatusPending),
			now,
		)
		if err != nil {
			return nil, fmt.Errorf("claim job %d: %w", id, err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("check claimed job %d: %w", id, err)
		}

		if affected == 1 {
			claimedIDs = append(claimedIDs, id)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claimed jobs: %w", err)
	}

	jobs := make([]models.Job, 0, len(claimedIDs))

	for _, id := range claimedIDs {
		job, err := database.GetJob(s.DB, id)
		if err != nil {
			return nil, fmt.Errorf("retrieve claimed job %d: %w", id, err)
		}

		jobs = append(jobs, *job)
	}

	return jobs, nil
}
