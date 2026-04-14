package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/spacebxr/strelp-api/internal/api"
	"github.com/spacebxr/strelp-api/internal/database"
	"github.com/spacebxr/strelp-api/internal/github"
)

func main() {
	log.SetOutput(os.Stdout)
	
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("API_PORT")
	}
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required (e.g., postgres://user:pass@localhost:5432/dbname)")
	}

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if len(encryptionKey) < 16 {
		log.Fatal("ENCRYPTION_KEY is required and must be at least 16 characters")
	}

	db, err := database.NewDatabase(dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("PostgreSQL connected successfully")

	pollerCtx, pollerCancel := context.WithCancel(context.Background())
	defer pollerCancel()
	p := &github.Poller{DB: db, EncryptionKey: encryptionKey}
	go p.Start(pollerCtx)
	log.Println("GitHub poller started (5-minute interval)")

	server := api.NewServer(db)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: server.Router,
	}

	go func() {
		if err := server.Start(":" + port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen: %s\n", err)
		}
	}()

	log.Printf("API Server (Postgres) is running on port %s", port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit

	log.Println("Shutting down API server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("API server exited gracefully")
}
