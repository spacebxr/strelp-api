package api

import (
	"log"
	"net/http"

	"encoding/json"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func fetchLyrics(song, artist string) string {
	url := fmt.Sprintf("https://api.lyrics.ovh/v1/%s/%s", artist, song)

	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var data struct {
		Lyrics string `json:"lyrics"`
	}

	json.NewDecoder(resp.Body).Decode(&data)
	return data.Lyrics
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
		if err := conn.WriteJSON(presence); err != nil {
			log.Printf("[API] Error sending initial state: %v", err)
			return
		}
		for _, activity := range presence.Activities {
			if activity.Name == "Spotify" {
				lyrics := fetchLyrics(activity.Details, activity.State)

				conn.WriteJSON(map[string]interface{}{
					"type":   "lyrics",
					"song":   activity.Details,
					"artist": activity.State,
					"lyrics": lyrics,
				})
			}
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

				if err := conn.WriteJSON(presence); err != nil {
					log.Printf("[API] Error streaming update: %v", err)
					return
				}

				// 🔥 ADD THIS
				for _, activity := range presence.Activities {
					if activity.Name == "Spotify" {
						lyrics := fetchLyrics(activity.Details, activity.State)

						conn.WriteJSON(map[string]interface{}{
							"type":   "lyrics",
							"song":   activity.Details,
							"artist": activity.State,
							"lyrics": lyrics,
						})
					}
				}
			}
		}
	}
}
