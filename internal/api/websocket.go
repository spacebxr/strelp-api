package api

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/spacebxr/strelp-api/internal/models"
)

func applyPresenceMeta(presence *models.Presence) {
	if len(presence.Activities) > 5 {
		presence.Activities = presence.Activities[len(presence.Activities)-5:]
	}

	now := time.Now().Unix()
	for i := range presence.Activities {
		start := presence.Activities[i].Timestamps.Start
		if start != 0 {
			presence.Activities[i].Duration = now - start
		}
	}
}

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

	presence, err := s.DB.GetPresence(r.Context(), userID)
	if err == nil {
		applyPresenceMeta(presence)
		if err := conn.WriteJSON(presence); err != nil {
			log.Printf("[API] Error sending initial state: %v", err)
			return
		}
	}

	dbConn, err := s.DB.AcquireConn(r.Context())
	if err != nil {
		log.Printf("[API] Error acquiring conn for LISTEN: %v", err)
		return
	}
	defer dbConn.Release()

	_, err = dbConn.Exec(r.Context(), "LISTEN presence_updates")
	if err != nil {
		log.Printf("[API] Error executing LISTEN: %v", err)
		return
	}

	for {
		notification, err := dbConn.Conn().WaitForNotification(r.Context())
		if err != nil {
			log.Printf("[API] Notification error: %v", err)
			return
		}

		if notification.Payload == userID {
			presence, err := s.DB.GetPresence(r.Context(), userID)
			if err == nil {
				applyPresenceMeta(presence)
				if err := conn.WriteJSON(presence); err != nil {
					log.Printf("[API] Error streaming update: %v", err)
					return
				}
			}
		}
	}
}
