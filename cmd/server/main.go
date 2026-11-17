package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"background-job-scheduler/internal/api"
	"background-job-scheduler/internal/database"
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

	handler := api.NewServer(db)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("======================================")
	log.Println(" Background Job Scheduler API")
	log.Println("======================================")
	log.Printf("Database: %s\n", dbPath)
	log.Printf("Server running at http://localhost:%s\n", port)
	log.Println("Health: GET /health")
	log.Println("Jobs:   /api/v1/jobs")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
