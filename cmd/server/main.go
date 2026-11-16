package main

import (
	"log"
	"os"

	"background-job-scheduler/internal/database"
)

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./scheduler.db"
	}

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	if err := database.InitSchema(db); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	log.Println("======================================")
	log.Println(" Background Job Scheduler")
	log.Println("======================================")
	log.Println("Database connected successfully.")
	log.Println("Database tables initialized.")
	log.Println("Scheduler setup is ready.")
}
