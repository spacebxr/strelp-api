package api

import (
	"encoding/json"
	"fmt"
	"net/http"

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
		for _, b := range profile.Badges {
			iconKey := b.Icon
			if iconKey == "" {
				iconKey = b.ID
			}
			badges = append(badges, models.Badge{
				ID:          b.ID,
				Description: b.Description,
				IconURL:     fmt.Sprintf("https://cdn.discordapp.com/badge-icons/%s.png", iconKey),
				Link:        b.Link,
			})
		}
		presence.Badges = badges
		presence.Nameplate = profile.NameplateURL()
		presence.NameplateLabel = profile.NameplateLabel()
		presence.ProfileEffectID = profile.ProfileEffectID()
		presence.ProfileEffectURL = profile.ProfileEffectURL()
		presence.User.Decoration = profile.DecorationURL()
		presence.User.Banner = profile.BannerURL()
		presence.User.Bio = profile.UserProfile.Bio
		presence.User.Pronouns = profile.UserProfile.Pronouns
		if profile.User.Clan != nil && profile.User.Clan.Tag != "" {
			presence.Clan = &models.Clan{
				Tag:      profile.User.Clan.Tag,
				BadgeURL: profile.ClanBadgeURL(),
			}
		}
	}

	applyPresenceMeta(presence)
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
