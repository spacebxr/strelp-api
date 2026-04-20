package api

import (
	"encoding/json"
	"net/http"

	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spacebxr/strelp-api/internal/discord"
	"github.com/spacebxr/strelp-api/internal/models"
)

func (s *Server) handleGetPresence(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	presence, err := s.DB.GetPresence(r.Context(), userID)
	if err != nil {
		http.Error(w, "Presence not found", http.StatusNotFound)
		return
	}

	profile, err := discord.FetchProfile(userID)
	if err == nil && profile != nil {

		var badges []models.Badge

		if profile.Badges != nil {
			for _, b := range profile.Badges {
				badges = append(badges, models.Badge{
					ID: b.ID,
				})
			}
		}
		presence.Badges = badges
		presence.Nameplate = profile.User.Collectibles.Nameplate.Asset
		presence.ClanTag = profile.User.Clan.Tag
	}

	if len(presence.Activities) > 5 {
		presence.Activities = presence.Activities[len(presence.Activities)-5:]
	}

	for i := range presence.Activities {
		start := presence.Activities[i].Timestamps.Start
		if start != 0 {
			presence.Activities[i].Duration = time.Now().Unix() - start
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(presence); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) handlePollerStatus(w http.ResponseWriter, r *http.Request) {
	active, err := s.DB.CountGitHubUsers(r.Context())
	if err != nil {
		http.Error(w, "Failed to get poller status", http.StatusInternalServerError)
		return
	}

	total, err := s.DB.CountAllGitHubUsers(r.Context())
	if err != nil {
		http.Error(w, "Failed to get poller status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                "ok",
		"currently_polling":     active,
		"total_accounts_polled": total,
	})
}
