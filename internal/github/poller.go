package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/spacebxr/strelp/internal/database"
	"github.com/spacebxr/strelp/internal/models"
)

type Poller struct {
	DB *database.Database
}

type GHEvent struct {
	Type string `json:"type"`
	Repo struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"repo"`
	Payload struct {
		Commits []struct {
			Message string `json:"message"`
			SHA     string `json:"sha"`
		} `json:"commits"`
	} `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollAll(ctx)
		}
	}
}

func (p *Poller) pollAll(ctx context.Context) {
	log.Println("Polling GitHub for active users...")
}

func (p *Poller) PollUser(ctx context.Context, userID string, ghUsername string) {
	url := fmt.Sprintf("https://api.github.com/users/%s/events/public", ghUsername)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error polling GitHub for %s: %v", ghUsername, err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var events []GHEvent
	if err := json.Unmarshal(body, &events); err != nil {
		return
	}

	if len(events) > 0 {
		event := events[0]
		presence, err := p.DB.GetPresence(ctx, userID)
		if err != nil {
			return
		}

		ghData := &models.GitHub{
			Username:  ghUsername,
			Repo:      event.Repo.Name,
			URL:       fmt.Sprintf("https://github.com/%s", event.Repo.Name),
			UpdatedAt: event.CreatedAt.Unix(),
		}

		if event.Type == "PushEvent" && len(event.Payload.Commits) > 0 {
			ghData.LastCommit = event.Payload.Commits[0].Message
		}

		presence.GitHub = ghData
		p.DB.SetPresence(ctx, userID, presence)
	}
}
