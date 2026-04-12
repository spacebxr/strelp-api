package models

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	GlobalName  string `json:"global_name"`
	Avatar      string `json:"avatar"`
	Decoration  string `json:"decoration,omitempty"`
}

type Spotify struct {
	Track    string `json:"track"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	AlbumArt string `json:"album_art"`
	Start    int64  `json:"start"`
	End      int64  `json:"end"`
}

type GitHub struct {
	Username   string `json:"username"`
	LastCommit string `json:"last_commit"`
	Repo       string `json:"repo"`
	URL        string `json:"url"`
	UpdatedAt  int64  `json:"updated_at"`
}

type Activity struct {
	Name      string `json:"name"`
	Type      int    `json:"type"`
	State     string `json:"state,omitempty"`
	Details   string `json:"details,omitempty"`
	Emoji     string `json:"emoji,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

type Presence struct {
	User          User       `json:"user"`
	DiscordStatus string     `json:"discord_status"`
	Activities    []Activity `json:"activities"`
	Spotify       *Spotify   `json:"spotify,omitempty"`
	GitHub        *GitHub    `json:"github,omitempty"`
}
