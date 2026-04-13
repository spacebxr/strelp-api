package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/spacebxr/strelp-api/internal/crypto"
	"github.com/spacebxr/strelp-api/internal/database"
	"github.com/spacebxr/strelp-api/internal/models"
)

type Poller struct {
	DB            *database.Database
	EncryptionKey string
}

type ghEvent struct {
	Type string `json:"type"`
	Repo struct {
		Name    string `json:"name"`
		Private bool   `json:"private"`
	} `json:"repo"`
	Payload struct {
		Commits []struct {
			Message string `json:"message"`
		} `json:"commits"`
	} `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

type ghUser struct {
	Login string `json:"login"`
}

func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	p.pollAll(ctx)

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
	users, err := p.DB.GetAllGitHubUsers(ctx)
	if err != nil {
		log.Printf("[GitHub] Failed to fetch GitHub users: %v", err)
		return
	}

	log.Printf("[GitHub] Polling %d user(s)", len(users))
	for _, u := range users {
		rawToken, err := crypto.Decrypt(u.AccessToken, p.EncryptionKey)
		if err != nil {
			log.Printf("[GitHub] Failed to decrypt token for %s: %v", u.UserID, err)
			continue
		}
		p.pollUser(ctx, u.UserID, u.Username, rawToken, u.ShowPrivate, u.ShowPublic)
	}
}

func (p *Poller) pollUser(ctx context.Context, userID, ghUsername, token string, showPrivate, showPublic bool) {
	url := fmt.Sprintf("https://api.github.com/users/%s/events", ghUsername)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[GitHub] HTTP error for %s: %v", ghUsername, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[GitHub] Non-200 response for %s: %d", ghUsername, resp.StatusCode)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var events []ghEvent
	if err := json.Unmarshal(body, &events); err != nil {
		return
	}

	presence, err := p.DB.GetPresence(ctx, userID)
	if err != nil {
		return
	}

	for _, event := range events {
		if event.Type != "PushEvent" {
			continue
		}
		if event.Repo.Private && !showPrivate {
			continue
		}
		if !event.Repo.Private && !showPublic {
			continue
		}

		ghData := &models.GitHub{
			Username:  ghUsername,
			Repo:      event.Repo.Name,
			URL:       fmt.Sprintf("https://github.com/%s", event.Repo.Name),
			Private:   event.Repo.Private,
			UpdatedAt: event.CreatedAt.Unix(),
		}

		if len(event.Payload.Commits) > 0 {
			ghData.LastCommit = event.Payload.Commits[0].Message
		}

		presence.GitHub = ghData
		if err := p.DB.SetPresence(ctx, userID, presence); err != nil {
			log.Printf("[GitHub] Failed to save presence for %s: %v", userID, err)
		}
		break
	}
}

func ValidateToken(token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid token (status %d)", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var u ghUser
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}
	return u.Login, nil
}
