package api

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the presence API
	},
}

func (s *Server) handleStreamPresence(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Client connected for streaming presence: %s", userID)

	// Fetch initial state
	presence, err := s.Cache.GetPresence(r.Context(), userID)
	if err == nil {
		if err := conn.WriteJSON(presence); err != nil {
			log.Printf("Error sending initial state: %v", err)
			return
		}
	}

	// Subscribe to updates
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	pubsub := s.Cache.Subscribe(ctx, userID)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case msg := <-ch:
			// Payload from Redis is already JSON string of models.Presence
			// We send it as raw message since it's already serialized
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				log.Printf("Error streaming update to client: %v", err)
				return
			}
		case <-r.Context().Done():
			log.Printf("Client disconnected (context done): %s", userID)
			return
		}
	}
}
