package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spacebxr/strelp-api/internal/lyrics"
)

type lyricsPayload struct {
	Type         string        `json:"type"`
	Song         string        `json:"song"`
	Artist       string        `json:"artist"`
	Synced       bool          `json:"synced"`
	Lines        []lyrics.Line `json:"lines,omitempty"`
	Plain        string        `json:"plain,omitempty"`
	CurrentIndex int           `json:"current_index"`
	CurrentText  string        `json:"current_text"`
}

type linePayload struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Text  string `json:"text"`
	TimeMs int64 `json:"time_ms"`
}

func buildLyricsPayload(track, artist string, startMs int64) (*lyricsPayload, error) {
	result, err := lyrics.Fetch(track, artist)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	p := &lyricsPayload{
		Type:         "lyrics",
		Song:         track,
		Artist:       artist,
		Synced:       result.Synced,
		Lines:        result.Lines,
		Plain:        result.Plain,
		CurrentIndex: -1,
	}

	if result.Synced && len(result.Lines) > 0 {
		elapsedMs := time.Now().UnixMilli() - startMs
		idx, line := lyrics.CurrentLine(result.Lines, elapsedMs)
		p.CurrentIndex = idx
		if line != nil {
			p.CurrentText = line.Text
		}
	}

	return p, nil
}

func (s *Server) handleGetLyrics(w http.ResponseWriter, r *http.Request) {
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

	if presence.Spotify == nil {
		http.Error(w, "No Spotify activity", http.StatusNotFound)
		return
	}

	sp := presence.Spotify
	startMs := sp.Start * 1000

	payload, err := buildLyricsPayload(sp.Track, sp.Artist, startMs)
	if err != nil {
		http.Error(w, "Failed to fetch lyrics", http.StatusServiceUnavailable)
		return
	}
	if payload == nil {
		http.Error(w, "Lyrics not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleStreamLyrics(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[lyrics-ws] upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[lyrics-ws] client connected: %s", userID)

	presence, err := s.DB.GetPresence(r.Context(), userID)
	if err != nil || presence.Spotify == nil {
		conn.WriteJSON(map[string]string{"type": "error", "message": "no spotify activity"})
		return
	}

	currentTrack := presence.Spotify.Track
	currentArtist := presence.Spotify.Artist
	currentStartMs := presence.Spotify.Start * 1000

	payload, _ := buildLyricsPayload(currentTrack, currentArtist, currentStartMs)
	if payload != nil {
		conn.WriteJSON(payload)
	}

	dbConn, err := s.DB.AcquireConn(r.Context())
	if err != nil {
		log.Printf("[lyrics-ws] db conn error: %v", err)
		return
	}
	defer dbConn.Release()

	_, err = dbConn.Exec(r.Context(), "LISTEN presence_updates")
	if err != nil {
		log.Printf("[lyrics-ws] LISTEN error: %v", err)
		return
	}

	notifyCh := make(chan struct{}, 1)
	go func() {
		for {
			_, err := dbConn.Conn().WaitForNotification(r.Context())
			if err != nil {
				return
			}
			select {
			case notifyCh <- struct{}{}:
			default:
			}
		}
	}()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	lastIdx := -2

	var currentLines []lyrics.Line
	if payload != nil {
		currentLines = payload.Lines
	}

	for {
		select {
		case <-ticker.C:
			if len(currentLines) == 0 {
				continue
			}
			elapsedMs := time.Now().UnixMilli() - currentStartMs
			idx, line := lyrics.CurrentLine(currentLines, elapsedMs)
			if idx == lastIdx {
				continue
			}
			lastIdx = idx
			lp := linePayload{
				Type:  "line",
				Index: idx,
			}
			if line != nil {
				lp.Text = line.Text
				lp.TimeMs = line.TimeMs
			}
			if err := conn.WriteJSON(lp); err != nil {
				return
			}

		case <-notifyCh:
			presence, err := s.DB.GetPresence(r.Context(), userID)
			if err != nil {
				continue
			}
			if presence.Spotify == nil {
				conn.WriteJSON(map[string]string{"type": "error", "message": "no spotify activity"})
				return
			}
			sp := presence.Spotify
			if sp.Track == currentTrack && sp.Artist == currentArtist {
				continue
			}
			currentTrack = sp.Track
			currentArtist = sp.Artist
			currentStartMs = sp.Start * 1000
			lastIdx = -2

			newPayload, _ := buildLyricsPayload(currentTrack, currentArtist, currentStartMs)
			if newPayload != nil {
				currentLines = newPayload.Lines
				conn.WriteJSON(newPayload)
			}

		case <-r.Context().Done():
			return
		}
	}
}
