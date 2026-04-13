package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/spacebxr/strelp-api/internal/bot"
	"github.com/spacebxr/strelp-api/internal/database"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN is required")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required (e.g., postgres://user:pass@localhost:5432/dbname)")
	}

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if len(encryptionKey) < 16 {
		log.Fatal("ENCRYPTION_KEY is required and must be at least 16 characters")
	}

	// Initialize Database
	db, err := database.NewDatabase(dbURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Println("PostgreSQL connected successfully")

	// Initialize Bot
	b, err := bot.NewBot(token, db, encryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	if err := b.Start(); err != nil {
		log.Fatalf("Failed to start bot: %v", err)
	}
	log.Println("Bot is now running (Postgres Engine). Press CTRL-C to exit.")

	guildID := os.Getenv("GUILD_ID")
	if err := b.RegisterCommands(guildID); err != nil {
		log.Printf("Error registering commands: %v", err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down bot...")
	b.Session.Close()
}
