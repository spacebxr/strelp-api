package api

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true 
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

	log.Printf("[API] Client connected for streaming: %s", userID)

	// Send initial state
	presence, err := s.DB.GetPresence(r.Context(), userID)
	if err == nil {
		if err := conn.WriteJSON(presence); err != nil {
			log.Printf("[API] Error sending initial state: %v", err)
			return
		}
	}

	// Listen for notifications
	// Acquire a dedicated connection for LISTEN
	dbConn, err := s.DB.AcquireConn(r.Context())
	if err != nil {
		log.Printf("[API] Error acquiring conn for LISTEN: %v", err)
		return
	}
	defer dbConn.Release()

	// Execute LISTEN
	_, err = dbConn.Exec(r.Context(), "LISTEN presence_updates")
	if err != nil {
		log.Printf("[API] Error executing LISTEN: %v", err)
		return
	}

	for {
		// Wait for notification
		notification, err := dbConn.Conn().WaitForNotification(r.Context())
		if err != nil {
			log.Printf("[API] Notification error: %v", err)
			return
		}

		// Check if the notification is for the specific user we are watching
		// notification.Payload contains the user_id (sent by pg_notify trigger)
		if notification.Payload == userID {
			presence, err := s.DB.GetPresence(r.Context(), userID)
			if err == nil {
				if err := conn.WriteJSON(presence); err != nil {
					log.Printf("[API] Error streaming update: %v", err)
					return
				}
			}
		}
	}
}
