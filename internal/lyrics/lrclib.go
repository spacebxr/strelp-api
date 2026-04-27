package lyrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Line struct {
	TimeMs int64  `json:"time_ms"`
	Text   string `json:"text"`
}

type Result struct {
	Synced bool   `json:"synced"`
	Lines  []Line `json:"lines,omitempty"`
	Plain  string `json:"plain,omitempty"`
}

var lrcRegex = regexp.MustCompile(`^\[(\d{2}):(\d{2})\.(\d{2,3})\](.*)$`)

var cache = map[string]*Result{}
var cacheTime = map[string]time.Time{}

const cacheDuration = 15 * time.Minute

func cacheKey(track, artist string) string {
	return track + "||" + artist
}

func Fetch(track, artist string) (*Result, error) {
	key := cacheKey(track, artist)
	if r, ok := cache[key]; ok {
		if time.Since(cacheTime[key]) < cacheDuration {
			return r, nil
		}
	}

	u := fmt.Sprintf("https://lrclib.net/api/get?track_name=%s&artist_name=%s",
		url.QueryEscape(track), url.QueryEscape(artist))

	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("lrclib status: %d", resp.StatusCode)
	}

	var body struct {
		SyncedLyrics string `json:"syncedLyrics"`
		PlainLyrics  string `json:"plainLyrics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	result := &Result{}

	if body.SyncedLyrics != "" {
		result.Synced = true
		result.Lines = parseLRC(body.SyncedLyrics)
	} else if body.PlainLyrics != "" {
		result.Synced = false
		result.Plain = body.PlainLyrics
	}

	cache[key] = result
	cacheTime[key] = time.Now()

	return result, nil
}

func parseLRC(raw string) []Line {
	var lines []Line
	for _, l := range strings.Split(raw, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		m := lrcRegex.FindStringSubmatch(l)
		if m == nil {
			continue
		}

		min, _ := strconv.ParseInt(m[1], 10, 64)
		sec, _ := strconv.ParseInt(m[2], 10, 64)
		frac, _ := strconv.ParseInt(m[3], 10, 64)

		var fracMs int64
		if len(m[3]) == 2 {
			fracMs = frac * 10
		} else {
			fracMs = frac
		}

		timeMs := (min*60+sec)*1000 + fracMs
		text := strings.TrimSpace(m[4])

		lines = append(lines, Line{TimeMs: timeMs, Text: text})
	}
	return lines
}

func CurrentLine(lines []Line, elapsedMs int64) (int, *Line) {
	idx := -1
	for i, l := range lines {
		if l.TimeMs <= elapsedMs {
			idx = i
		} else {
			break
		}
	}
	if idx < 0 {
		return -1, nil
	}
	return idx, &lines[idx]
}
