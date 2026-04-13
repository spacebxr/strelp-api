package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
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
