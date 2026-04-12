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

	presence, err := s.Cache.GetPresence(r.Context(), userID)
	if err != nil {
		// If key not found, redis results in an error we can interpret
		// For simplicity, we assume any error means not found if we don't handle it specifically
		http.Error(w, "Presence not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(presence); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
