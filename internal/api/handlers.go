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
