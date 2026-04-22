package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/spacebxr/strelp-api/internal/crypto"
	"github.com/spacebxr/strelp-api/internal/database"
	"github.com/spacebxr/strelp-api/internal/discord"
	"github.com/spacebxr/strelp-api/internal/github"
	"github.com/spacebxr/strelp-api/internal/models"
)

type Bot struct {
	Session        *discordgo.Session
	DB             *database.Database
	EncryptionKey  string
	AllowedGuildID string
	SyncRoles      []string
}

func NewBot(token string, db *database.Database, encryptionKey string, allowedGuildID string, syncRoles []string) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		Session:        dg,
		DB:             db,
		EncryptionKey:  encryptionKey,
		AllowedGuildID: allowedGuildID,
		SyncRoles:      syncRoles,
	}

	dg.Identify.Intents = discordgo.IntentGuildPresences | discordgo.IntentGuildMembers | discordgo.IntentGuilds

	dg.AddHandler(b.onPresenceUpdate)
	dg.AddHandler(b.onMemberRemove)
	dg.AddHandler(b.onInteractionCreate)

	return b, nil
}

func (b *Bot) Start() error {
	return b.Session.Open()
}

func resolveAssetURL(appID, imageKey string) string {
	if imageKey == "" {
		return ""
	}
	if strings.HasPrefix(imageKey, "spotify:") {
		return fmt.Sprintf("https://i.scdn.co/image/%s", imageKey[8:])
	}
	if strings.HasPrefix(imageKey, "mp:") {
		return fmt.Sprintf("https://media.discordapp.net/%s", imageKey[3:])
	}
	if appID == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/app-assets/%s/%s.png", appID, imageKey)
}

func buildActivities(discordActivities []*discordgo.Activity) ([]models.Activity, *models.Spotify) {
	activities := make([]models.Activity, len(discordActivities))
	var spotify *models.Spotify

	for i, a := range discordActivities {
		startTime := a.Timestamps.StartTimestamp / 1000
		if startTime == 0 && !a.CreatedAt.IsZero() {
			startTime = a.CreatedAt.Unix()
		}

		activities[i] = models.Activity{
			Name:      a.Name,
			Type:      int(a.Type),
			State:     a.State,
			Details:   a.Details,
			CreatedAt: startTime,
		}
		activities[i].Timestamps.Start = startTime
		activities[i].LargeImage = resolveAssetURL(a.ApplicationID, a.Assets.LargeImageID)
		activities[i].SmallImage = resolveAssetURL(a.ApplicationID, a.Assets.SmallImageID)
		activities[i].LargeText = a.Assets.LargeText
		activities[i].SmallText = a.Assets.SmallText

		if a.Name == "Spotify" {
			sStart := a.Timestamps.StartTimestamp / 1000
			sEnd := a.Timestamps.EndTimestamp / 1000

			albumArt := ""
			var albumName string
			albumName = a.Assets.LargeText
			if len(a.Assets.LargeImageID) > 8 {
				albumArt = fmt.Sprintf("https://i.scdn.co/image/%s", a.Assets.LargeImageID[8:])
			}

			spotify = &models.Spotify{
				Track:    a.Details,
				Artist:   a.State,
				Album:    albumName,
				AlbumArt: albumArt,
				Start:    sStart,
				End:      sEnd,
			}
		}
	}

	return activities, spotify
}

func (b *Bot) onPresenceUpdate(s *discordgo.Session, p *discordgo.PresenceUpdate) {
	if p.GuildID != b.AllowedGuildID {
		return
	}

	ctx := context.Background()

	existing, err := b.DB.GetPresence(ctx, p.User.ID)
	if err != nil {
		return
	}

	log.Printf("[Bot] Updating presence for user: %s", p.User.Username)

	activities, spotify := buildActivities(p.Activities)

	userObj := p.User
	if cachedMember, err := s.State.Member(p.GuildID, p.User.ID); err == nil {
		userObj = cachedMember.User
	} else if cachedUser, err := s.User(p.User.ID); err == nil {
		userObj = cachedUser
	}

	var badges []models.Badge
	var nameplate string
	var clanTag string

	dstnProf, _ := discord.FetchProfile(userObj.ID)
	if dstnProf != nil {
		nameplate = dstnProf.User.Collectibles.Nameplate.Asset
		clanTag = dstnProf.User.Clan.Tag
		for _, bg := range dstnProf.Badges {
			iconKey := bg.Icon
			if iconKey == "" {
				iconKey = bg.ID
			}
			badges = append(badges, models.Badge{
				ID:      bg.ID,
				IconURL: fmt.Sprintf("https://cdn.discordapp.com/badge-icons/%s.png", iconKey),
			})
		}
	}

	now := time.Now().Unix()
	history := existing.History
	for _, old := range existing.Activities {
		if old.Name == "Spotify" {
			continue
		}
		stillActive := false
		for _, newAct := range p.Activities {
			if newAct.Name == old.Name {
				stillActive = true
				break
			}
		}
		if !stillActive {
			if old.Timestamps.Start != 0 {
				old.Duration = now - old.Timestamps.Start
			}
			var newHistory []models.Activity
			for _, h := range history {
				if h.Name != old.Name {
					newHistory = append(newHistory, h)
				}
			}
			history = append([]models.Activity{old}, newHistory...)
		}
	}
	if len(history) > 5 {
		history = history[:5]
	}

	presence := &models.Presence{
		User: models.User{
			ID:         userObj.ID,
			Username:   userObj.Username,
			GlobalName: userObj.GlobalName,
			Avatar:     userObj.AvatarURL("1024"),
		},
		DiscordStatus: string(p.Status),
		Activities:    activities,
		Spotify:       spotify,
		Badges:        badges,
		Nameplate:     nameplate,
		ClanTag:       clanTag,
		GitHub:        existing.GitHub,
		History:       history,
	}

	presence.Devices.Desktop = p.ClientStatus.Desktop != ""
	presence.Devices.Mobile = p.ClientStatus.Mobile != ""
	presence.Devices.Web = p.ClientStatus.Web != ""

	if err := b.DB.SetPresence(ctx, p.User.ID, presence); err != nil {
		log.Printf("[Bot] Error saving presence for %s: %v", p.User.ID, err)
	}
}

func (b *Bot) onMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.GuildID != b.AllowedGuildID {
		return
	}

	ctx := context.Background()
	log.Printf("[Bot] User %s left, deleting data...", m.User.ID)
	b.DB.DeletePresence(ctx, m.User.ID)
}

func (b *Bot) RegisterCommands(guildID string) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "start",
			Description: "Start tracking your presence and enable your Strelp API",
		},
		{
			Name:        "stop",
			Description: "Stop tracking your presence and delete your Strelp data",
		},
		{
			Name:        "ws",
			Description: "Learn how to use WebSockets for real-time data",
		},
		{
			Name:        "git",
			Description: "Connect your GitHub account to show your latest commits in your presence",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "token",
					Description: "Your GitHub Personal Access Token (keep this private)",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "visibility",
					Description: "Which repos to show in your presence",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Public repos only", Value: "public"},
						{Name: "Private repos only", Value: "private"},
						{Name: "Both public and private", Value: "both"},
					},
				},
			},
		},
		{
			Name:        "gitstop",
			Description: "Disconnect your GitHub account and stop showing commit data",
		},
		{
			Name:        "sync",
			Description: "Sync all tracked users presence to the latest version (Staff Only)",
		},
		{
			Name:        "help",
			Description: "Learn everything about Strelp — setup, endpoints, WebSocket, GitHub integration, and more",
		},
	}

	_, err := b.Session.ApplicationCommandBulkOverwrite(b.Session.State.User.ID, guildID, commands)
	if err == nil && guildID != "" {
		b.Session.ApplicationCommandBulkOverwrite(b.Session.State.User.ID, "", nil)
	}
	return err
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID != b.AllowedGuildID {
		return
	}

	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	ctx := context.Background()
	data := i.ApplicationCommandData()

	user := i.Member.User
	if user == nil && i.User != nil {
		user = i.User
	}

	switch data.Name {
	case "start":
		var badges []models.Badge
		var nameplate string
		var clanTag string

		dstnProf, _ := discord.FetchProfile(user.ID)
		if dstnProf != nil {
			nameplate = dstnProf.User.Collectibles.Nameplate.Asset
			clanTag = dstnProf.User.Clan.Tag
			for _, bg := range dstnProf.Badges {
				iconKey := bg.Icon
				if iconKey == "" {
					iconKey = bg.ID
				}
				badges = append(badges, models.Badge{
					ID:      bg.ID,
					IconURL: fmt.Sprintf("https://cdn.discordapp.com/badge-icons/%s.png", iconKey),
				})
			}
		}

		presence := &models.Presence{
			User: models.User{
				ID:         user.ID,
				Username:   user.Username,
				GlobalName: user.GlobalName,
				Avatar:     user.Avatar,
			},
			DiscordStatus: "online",
			Activities:    []models.Activity{},
			Badges:        badges,
			Nameplate:     nameplate,
			ClanTag:       clanTag,
		}

		if p, err := s.State.Presence(i.GuildID, user.ID); err == nil && p != nil {
			presence.DiscordStatus = string(p.Status)
			presence.Devices.Desktop = p.ClientStatus.Desktop != ""
			presence.Devices.Mobile = p.ClientStatus.Mobile != ""
			presence.Devices.Web = p.ClientStatus.Web != ""
		}

		if err := b.DB.SetPresence(ctx, user.ID, presence); err != nil {
			log.Printf("[Bot] Error enabling tracking for %s: %v", user.ID, err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "Something went wrong",
							Description: "Failed to start tracking your presence. Please try again in a moment. If this keeps happening, reach out to a developer or a contributor.",
							Color:       0xED4245,
						},
					},
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		apiDomain := os.Getenv("RAILWAY_PUBLIC_DOMAIN")
		if apiDomain == "" {
			apiDomain = "strelp-api-production.up.railway.app"
		}

		embed := &discordgo.MessageEmbed{
			Title:       "Tracking Started",
			Description: fmt.Sprintf("Your presence is now live. Use the endpoint below to fetch your real-time Discord status from anywhere.\n\n**Your endpoint:**\n`https://%s/v1/presence/%s`", apiDomain, user.ID),
			Color:       0x57F287,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "Fetch — JavaScript",
					Value: fmt.Sprintf("```js\nfetch('https://%s/v1/presence/%s')\n  .then(res => res.json())\n  .then(data => {\n    console.log(data.discord_status);\n    console.log(data.user.global_name);\n    console.log(data.activities);\n  });\n```", apiDomain, user.ID),
				},
				{
					Name:  "WebSocket — Real-time Updates",
					Value: fmt.Sprintf("For instant updates without polling, connect to the WebSocket endpoint:\n`wss://%s/v1/presence/%s/ws`\nRun `/ws` for a full example.", apiDomain, user.ID),
				},
				{
					Name:  "Response Fields",
					Value: "`discord_status` — online / idle / dnd / offline\n`user` — id, username, global_name, avatar URL\n`activities` — current games or activities\n`spotify` — track, artist, album, album art, timestamps\n`github` — latest commit, repo, URL\n`badges` — id and icon_url for each badge\n`devices` — desktop, mobile, web (boolean)",
				},
				{
					Name:  "Troubleshooting",
					Value: "**404 Not Found** — Run `/start` first. Your data only exists while tracking is active.\n**Stale data** — Presence updates are pushed by Discord in real-time. If your status looks wrong, change it on Discord and it will refresh automatically.",
				},
			},
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})

	case "stop":
		b.DB.DeletePresence(ctx, user.ID)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "Tracking Stopped",
						Description: "Your presence data has been removed from the database. Your API endpoint will return 404 until you run `/start` again.\n\nYour GitHub connection, if any, has not been removed — use `/gitstop` to disconnect that separately.",
						Color:       0xFEE75C,
					},
				},
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})

	case "ws":
		apiDomain := os.Getenv("RAILWAY_PUBLIC_DOMAIN")
		if apiDomain == "" {
			apiDomain = "strelp-api-production.up.railway.app"
		}

		embed := &discordgo.MessageEmbed{
			Title:       "Real-Time Presence via WebSocket",
			Description: fmt.Sprintf("WebSockets push updates to your app the instant your Discord status changes — no polling required.\n\n**Your WebSocket URL:**\n`wss://%s/v1/presence/%s/ws`", apiDomain, user.ID),
			Color:       0x5865F2,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "JavaScript Example",
					Value: fmt.Sprintf("```js\nconst ws = new WebSocket('wss://%s/v1/presence/%s/ws');\n\nws.onopen = () => {\n  console.log('Connected');\n};\n\nws.onmessage = (event) => {\n  const data = JSON.parse(event.data);\n  // data is the full presence object\n  console.log(data.discord_status);\n  console.log(data.spotify?.track);\n};\n\nws.onclose = () => {\n  // reconnect after a delay\n  setTimeout(() => location.reload(), 3000);\n};\n```", apiDomain, user.ID),
				},
				{
					Name:  "Behaviour",
					Value: "The server sends the full presence object immediately on connect, then pushes a new payload every time your status or activity changes. There is no need to send any messages to the server.",
				},
				{
					Name:  "Troubleshooting",
					Value: "**Connection closes instantly** — Make sure the URL uses `wss://` not `https://`. Confirm `/start` has been run.\n**No initial message** — Register your `onmessage` handler before the connection opens.\n**Reconnection** — The server will close idle or errored connections. Add a reconnect loop in your client.",
				},
			},
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})

	case "git":
		opts := data.Options
		var rawToken, visibility string
		for _, o := range opts {
			switch o.Name {
			case "token":
				rawToken = o.StringValue()
			case "visibility":
				visibility = o.StringValue()
			}
		}

		ghUsername, err := github.ValidateToken(rawToken)
		if err != nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "Invalid GitHub Token",
							Description: fmt.Sprintf("Could not authenticate with the token you provided.\n\n**Error:** %v", err),
							Color:       0xED4245,
							Fields: []*discordgo.MessageEmbedField{
								{
									Name:  "How to create a token",
									Value: "Go to **GitHub → Settings → Developer settings → Personal access tokens** and generate a new token. For Classic PATs, enable the `repo` and `read:user` scopes. For Fine-Grained PATs, grant Read-only access to Contents and Metadata.",
								},
							},
						},
					},
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		encryptedToken, err := crypto.Encrypt(rawToken, b.EncryptionKey)
		if err != nil {
			log.Printf("[Bot] Failed to encrypt GitHub token for %s: %v", user.ID, err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "Something went wrong",
							Description: "An internal error occurred while securing your token. Please try again in a moment.",
							Color:       0xED4245,
						},
					},
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		showPrivate := visibility == "private" || visibility == "both"
		showPublic := visibility == "public" || visibility == "both"

		settings := &database.GitHubSettings{
			UserID:      user.ID,
			AccessToken: encryptedToken,
			Username:    ghUsername,
			ShowPrivate: showPrivate,
			ShowPublic:  showPublic,
		}

		if err := b.DB.SaveGitHubSettings(ctx, settings); err != nil {
			log.Printf("[Bot] Failed to save GitHub settings for %s: %v", user.ID, err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "Failed to save settings",
							Description: "Could not save your GitHub settings to the database. Please try again. If the issue persists, contact a server admin.",
							Color:       0xED4245,
						},
					},
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "GitHub Connected",
						Description: fmt.Sprintf("Your GitHub account **%s** is now linked. Commit data will appear in your API presence within 5 minutes and will update every 5 minutes after that.", ghUsername),
						Color:       0x238636,
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:  "Visibility Setting",
								Value: fmt.Sprintf("Currently set to show **%s** repositories. You can change this at any time by running `/git` again with a different visibility option.", visibility),
							},
							{
								Name:  "What appears in the API",
								Value: "The `github` field in your presence will contain:\n`username` — your GitHub username\n`last_commit` — the message of your most recent commit\n`repo` — the repository it was pushed to\n`url` — a direct link to the commit\n`private` — whether the repo is private\n`updated_at` — Unix timestamp of the commit",
							},
							{
								Name:  "Security",
								Value: "Your token is encrypted with AES-256-GCM before being stored. It is never logged, cached in plaintext, or returned by the API. Run `/gitstop` at any time to revoke access and purge your data.",
							},
						},
					},
				},
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})

	case "gitstop":
		b.DB.DeleteGitHubSettings(ctx, user.ID)

		presence, err := b.DB.GetPresence(ctx, user.ID)
		if err == nil {
			presence.GitHub = nil
			b.DB.SetPresence(ctx, user.ID, presence)
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       "GitHub Disconnected",
						Description: "Your GitHub account has been unlinked. Your encrypted token and all commit data have been permanently deleted from the database.\n\nThe `github` field will no longer appear in your API response. Your presence tracking via `/start` is still active.",
						Color:       0xFEE75C,
					},
				},
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})

	case "sync":
		isAllowed := false
		if i.Member != nil {
			for _, userRole := range i.Member.Roles {
				for _, allowedRole := range b.SyncRoles {
					if userRole == allowedRole {
						isAllowed = true
						break
					}
				}
				if isAllowed {
					break
				}
			}
		}

		if !isAllowed {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "Access Denied",
							Description: "You do not have a role that is permitted to run `/sync`. Contact a server admin if you believe this is a mistake.",
							Color:       0xED4245,
						},
					},
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})

		userIDs, err := b.DB.GetAllTrackedUserIDs(ctx)
		if err != nil {
			log.Printf("[Bot] Failed to get tracked users for sync: %v", err)
			msg := "Internal error while fetching users."
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
			})
			return
		}

		go func() {
			count := 0
			for _, userID := range userIDs {
				userObj, err := s.User(userID)
				if err != nil {
					log.Printf("[Bot] Failed to fetch user %s during sync: %v", userID, err)
					continue
				}

				var badges []models.Badge
				var nameplate string
				var clanTag string

				dstnProf, _ := discord.FetchProfile(userID)
				if dstnProf != nil {
					nameplate = dstnProf.User.Collectibles.Nameplate.Asset
					clanTag = dstnProf.User.Clan.Tag
					for _, bg := range dstnProf.Badges {
						iconKey := bg.Icon
						if iconKey == "" {
							iconKey = bg.ID
						}
						badges = append(badges, models.Badge{
							ID:      bg.ID,
							IconURL: fmt.Sprintf("https://cdn.discordapp.com/badge-icons/%s.png", iconKey),
						})
					}
				}

				presence, err := b.DB.GetPresence(ctx, userID)
				if err != nil {
					presence = &models.Presence{
						User: models.User{
							ID:         userID,
							Username:   userObj.Username,
							GlobalName: userObj.GlobalName,
							Avatar:     userObj.AvatarURL("1024"),
						},
					}
				} else {
					presence.User.Username = userObj.Username
					presence.User.GlobalName = userObj.GlobalName
					presence.User.Avatar = userObj.AvatarURL("1024")
				}

				presence.Badges = badges
				presence.Nameplate = nameplate
				presence.ClanTag = clanTag

				if p, err := s.State.Presence(i.GuildID, userID); err == nil && p != nil {
					presence.DiscordStatus = string(p.Status)
					presence.Devices.Desktop = p.ClientStatus.Desktop != ""
					presence.Devices.Mobile = p.ClientStatus.Mobile != ""
					presence.Devices.Web = p.ClientStatus.Web != ""

					acts, spot := buildActivities(p.Activities)
					presence.Activities = acts
					presence.Spotify = spot
				}

				if err := b.DB.SetPresence(ctx, userID, presence); err != nil {
					log.Printf("[Bot] Error updating presence for %s during sync: %v", userID, err)
					continue
				}
				count++
			}

			emptyStr := ""
			successEmbed := &discordgo.MessageEmbed{
				Title:       "Sync Complete",
				Description: fmt.Sprintf("Successfully synced **%d** tracked users to the latest version.\n\nProfile data, badges, nameplates, clan tags, activities, and status have all been refreshed.", count),
				Color:       0x57F287,
			}
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &emptyStr,
				Embeds:  &[]*discordgo.MessageEmbed{successEmbed},
			})
		}()
	case "help":
		apiDomain := os.Getenv("RAILWAY_PUBLIC_DOMAIN")
		if apiDomain == "" {
			apiDomain = "strelp-api-production.up.railway.app"
		}

		helpEmbed := &discordgo.MessageEmbed{
			Title:       "Strelp API — Complete Guide",
			Description: fmt.Sprintf("Strelp exposes your Discord presence as a live JSON API and WebSocket stream that anyone can consume from a website, app, or tool.\n\nBase URL: `https://%s`", apiDomain),
			Color:       0x5865F2,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "Step 1 — Start Tracking",
					Value: "Run `/start` to register yourself. This creates your presence record and generates your personal API endpoint. Nothing is exposed until you run this command.",
				},
				{
					Name:  "Getting Your Access Key",
					Value: fmt.Sprintf("No tokens or sign-ups needed. Your Discord User ID is your key. After running `/start`, your endpoint is:\n`https://%s/v1/presence/{your_discord_id}`\n\nTo find your User ID: enable Developer Mode in Discord settings, right-click your name, then select Copy User ID.", apiDomain),
				},
				{
					Name:  "REST Endpoint — Fetch Presence",
					Value: fmt.Sprintf("```js\nfetch('https://%s/v1/presence/{your_discord_id}')\n  .then(res => res.json())\n  .then(data => {\n    console.log(data.discord_status);\n    console.log(data.user.global_name);\n    console.log(data.activities);\n    console.log(data.spotify?.track);\n  });\n```", apiDomain),
				},
				{
					Name:  "WebSocket — Real-Time Stream",
					Value: fmt.Sprintf("Connect to `wss://%s/v1/presence/{your_discord_id}/ws` for instant push updates.\n\nThe server sends the full presence object immediately on connect, then pushes a new payload every time your status or activity changes. No messages need to be sent to the server.", apiDomain),
				},
				{
					Name:  "WebSocket Reconnection Pattern",
					Value: fmt.Sprintf("```js\nfunction connect() {\n  const ws = new WebSocket('wss://%s/v1/presence/{id}/ws');\n  ws.onmessage = e => update(JSON.parse(e.data));\n  ws.onclose = () => setTimeout(connect, 3000);\n}\nconnect();\n```\nRun `/ws` for a full annotated example.", apiDomain),
				},
				{
					Name:  "Response Fields Reference",
					Value: "`discord_status` — online / idle / dnd / offline\n`user` — id, username, global_name, avatar URL\n`activities` — current games or apps with images and timestamps\n`spotify` — track, artist, album, album art, start/end timestamps\n`github` — last commit message, repo, URL, privacy flag, timestamp\n`badges` — id and icon_url for each Discord badge\n`devices` — desktop, mobile, web (booleans)\n`nameplate` — active Discord nameplate asset\n`clan_tag` — Discord clan tag if set\n`history` — last 5 ended activities with durations",
				},
				{
					Name:  "GitHub Integration",
					Value: "Run `/git token:<PAT> visibility:<choice>` to link your GitHub account. Your latest commit will appear in the `github` field, refreshed every 5 minutes.\n\n**Creating a PAT:** GitHub → Settings → Developer settings → Personal access tokens. For `Fine-Grained PATs` grant `Read-only` access to `Contents and Metadata`. For `Classic PATs` enable `repo` and `read:user` scopes.\n\nYour token is encrypted with AES-256-GCM before storage and is never returned by the API. Run `/gitstop` to unlink at any time.",
				},
				{
					Name:  "Available Commands",
					Value: "`/start` — Begin tracking your presence\n`/stop` — Delete your presence data\n`/ws` — Full WebSocket code example for your user ID\n`/git` — Link your GitHub account\n`/gitstop` — Unlink GitHub and remove your token\n`/sync` — Staff only: refresh all tracked users\n`/help` — This guide",
				},
				{
					Name:  "Common Issues",
					Value: "**404 Not Found** — Run `/start` first. Your endpoint only exists while tracking is active.\n**Stale presence** — Change your Discord status or activity; updates are pushed by Discord in real time.\n**WebSocket closes instantly** — Confirm the URL uses `wss://` and that you have run `/start`.\n**No Spotify data** — Spotify must be active and linked to Discord via User Settings → Connections.\n**No GitHub data** — Allow up to 5 minutes after running `/git` for the first poll to complete.",
				},
			},
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{helpEmbed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
	}
}
