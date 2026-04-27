package discord

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var userToken string

func Init(token string) {
	userToken = token
}

type DiscordProfile struct {
	User struct {
		ID     string `json:"id"`
		Banner string `json:"banner"`

		AvatarDecorationData *struct {
			Asset string `json:"asset"`
			SkuID string `json:"sku_id"`
		} `json:"avatar_decoration_data"`

		Clan *struct {
			IdentityGuildID string `json:"identity_guild_id"`
			Tag             string `json:"tag"`
			Badge           string `json:"badge"`
		} `json:"clan"`

		Collectibles *struct {
			Nameplate *struct {
				Asset string `json:"asset"`
				Label string `json:"label"`
			} `json:"nameplate"`
		} `json:"collectibles"`
	} `json:"user"`

	UserProfile struct {
		Bio           string `json:"bio"`
		Pronouns      string `json:"pronouns"`
		ProfileEffect *struct {
			ID string `json:"id"`
		} `json:"profile_effect"`
	} `json:"user_profile"`

	Badges []struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Link        string `json:"link"`
	} `json:"badges"`
}

func (p *DiscordProfile) DecorationURL() string {
	if p.User.AvatarDecorationData == nil || p.User.AvatarDecorationData.Asset == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatar-decoration-presets/%s.png", p.User.AvatarDecorationData.Asset)
}

func (p *DiscordProfile) ClanBadgeURL() string {
	if p.User.Clan == nil || p.User.Clan.Badge == "" || p.User.Clan.IdentityGuildID == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/clan-badges/%s/%s.png", p.User.Clan.IdentityGuildID, p.User.Clan.Badge)
}

func (p *DiscordProfile) BannerURL() string {
	if p.User.ID == "" || p.User.Banner == "" {
		return ""
	}
	ext := "png"
	if strings.HasPrefix(p.User.Banner, "a_") {
		ext = "gif"
	}
	return fmt.Sprintf("https://cdn.discordapp.com/banners/%s/%s.%s?size=480", p.User.ID, p.User.Banner, ext)
}

func (p *DiscordProfile) NameplateURL() string {
	if p.User.Collectibles == nil || p.User.Collectibles.Nameplate == nil {
		return ""
	}
	return p.User.Collectibles.Nameplate.Asset
}

func (p *DiscordProfile) NameplateLabel() string {
	if p.User.Collectibles == nil || p.User.Collectibles.Nameplate == nil {
		return ""
	}
	return p.User.Collectibles.Nameplate.Label
}

func (p *DiscordProfile) ProfileEffectID() string {
	if p.UserProfile.ProfileEffect == nil {
		return ""
	}
	return p.UserProfile.ProfileEffect.ID
}

func (p *DiscordProfile) ProfileEffectURL() string {
	if p.UserProfile.ProfileEffect == nil || p.UserProfile.ProfileEffect.ID == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/effects/%s/flash.apng", p.UserProfile.ProfileEffect.ID)
}

var cache = map[string]DiscordProfile{}
var cacheTime = map[string]time.Time{}

const cacheDuration = 5 * time.Minute

func FetchProfile(userID string) (*DiscordProfile, error) {
	if userToken == "" {
		return nil, fmt.Errorf("discord user token not configured")
	}

	if data, ok := cache[userID]; ok {
		if time.Since(cacheTime[userID]) < cacheDuration {
			return &data, nil
		}
	}

	url := fmt.Sprintf("https://discord.com/api/v10/users/%s/profile?with_mutual_guilds=false&with_mutual_friends_count=false", userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", userToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("discord profile status: %d", resp.StatusCode)
	}

	var data DiscordProfile
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	cache[userID] = data
	cacheTime[userID] = time.Now()

	return &data, nil
}
