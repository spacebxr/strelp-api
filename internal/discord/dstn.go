package discord

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DstnProfile struct {
	User struct {
		Bio string `json:"bio"`

		Clan struct {
			Tag string `json:"tag"`
		} `json:"clan"`

		Collectibles struct {
			Nameplate struct {
				Asset string `json:"asset"`
			} `json:"nameplate"`
		} `json:"collectibles"`
	} `json:"user"`

	Badges []struct {
		ID string `json:"id"`
	} `json:"badges"`
}

var cache = map[string]DstnProfile{}
var cacheTime = map[string]time.Time{}

const cacheDuration = 5 * time.Minute

func FetchProfile(userID string) (*DstnProfile, error) {
	if data, ok := cache[userID]; ok {
		if time.Since(cacheTime[userID]) < cacheDuration {
			return &data, nil
		}
	}

	url := fmt.Sprintf("https://dcdn.dstn.to/profile/%s", userID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("dstn status: %d", resp.StatusCode)
	}

	var data DstnProfile
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	cache[userID] = data
	cacheTime[userID] = time.Now()

	return &data, nil
}
