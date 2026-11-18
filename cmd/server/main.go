package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"background-job-scheduler/internal/api"
	"background-job-scheduler/internal/database"
	"background-job-scheduler/internal/scheduler"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./scheduler.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	if err := database.InitSchema(db); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create the scheduler.
	jobScheduler := scheduler.New(db, time.Second, 10)

	// Start the scheduler in the background.
	go jobScheduler.Start(ctx)

	// Temporary queue consumer.
	// Step 5 will replace this with real workers.
	go func() {
		for {
			select {
			case job := <-jobScheduler.Jobs():
				log.Printf(
					"TEMPORARY CONSUMER: Received job ID=%d, Name=%s, Type=%s",
					job.ID,
					job.Name,
					job.Type,
				)

			case <-ctx.Done():
				return
			}
		}
	}()

	handler := api.NewServer(db)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("======================================")
	log.Println(" Background Job Scheduler API")
	log.Println("======================================")
	log.Printf("Database: %s", dbPath)
	log.Printf("Server running at http://localhost:%s", port)
	log.Println("Scheduler polling every second")
	log.Println("Health: GET /health")
	log.Println("Jobs:   /api/v1/jobs")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
