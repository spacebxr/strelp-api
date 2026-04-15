package bot

import (
	"context"
	"fmt"
	"log"
	"os"

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

func (b *Bot) onPresenceUpdate(s *discordgo.Session, p *discordgo.PresenceUpdate) {
	if p.GuildID != b.AllowedGuildID {
		return
	}

	ctx := context.Background()

	_, err := b.DB.GetPresence(ctx, p.User.ID)
	if err != nil {
		return
	}

	log.Printf("[Bot] Updating presence for user: %s", p.User.Username)

	activities := make([]models.Activity, len(p.Activities))
	var spotify *models.Spotify

	for i, a := range p.Activities {
		startTime := a.Timestamps.StartTimestamp / 1000

		activities[i] = models.Activity{
			Name:      a.Name,
			Type:      int(a.Type),
			State:     a.State,
			Details:   a.Details,
			CreatedAt: startTime,
		}

		if a.Name == "Spotify" {
			startTime := a.Timestamps.StartTimestamp / 1000
			endTime := a.Timestamps.EndTimestamp / 1000

			albumArt := ""
			if len(a.Assets.LargeImageID) > 8 {
				albumArt = fmt.Sprintf("https://i.scdn.co/image/%s", a.Assets.LargeImageID[8:])
			}

			spotify = &models.Spotify{
				Track:    a.Details,
				Artist:   a.State,
				Album:    a.Assets.LargeText,
				AlbumArt: albumArt,
				Start:    startTime,
				End:      endTime,
			}
		}
	}

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
					Content: "Failed to start tracking. Please try again later.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		apiDomain := os.Getenv("RAILWAY_PUBLIC_DOMAIN")
		if apiDomain == "" {
			apiDomain = "strelp-api-production.up.railway.app"
		}

		embed := &discordgo.MessageEmbed{
			Title:       "Strelp Tracking Started Successfully",
			Description: "Your presence is now being actively tracked and is ready to be fetched.",
			Color:       0x00FF00,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "Your API Endpoint",
					Value: fmt.Sprintf("Make a standard HTTP GET request to:\n`https://%s/v1/presence/%s`", apiDomain, user.ID),
				},
				{
					Name:  "How to use it with Code",
					Value: fmt.Sprintf("```javascript\nfetch('https://%s/v1/presence/%s')\n  .then(res => res.json())\n  .then(data => console.log('Current status:', data.discord_status));\n```", apiDomain, user.ID),
				},
				{
					Name:  "Common Errors and Fixes",
					Value: "**Error: 404 Not Found**\nFix: Ensure you have run the /start command. Data is only stored while the bot is tracking you.\n\n**Error: CORS Blocked**\nFix: The API allows cross-origin requests by default. If you encounter CORS issues, verify that you are not sending custom restricted headers without setting them up properly.",
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
				Content: "Strelp tracking stopped. Your data has been removed from the database.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

	case "ws":
		apiDomain := os.Getenv("RAILWAY_PUBLIC_DOMAIN")
		if apiDomain == "" {
			apiDomain = "strelp-api-production.up.railway.app"
		}

		embed := &discordgo.MessageEmbed{
			Title:       "Using WebSockets for Real-Time Presence",
			Description: "WebSockets allow your website to receive instant presence updates seamlessly without constant API fetching.",
			Color:       0x00B0F4,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "Connection URL",
					Value: fmt.Sprintf("`wss://%s/v1/presence/%s/ws`", apiDomain, user.ID),
				},
				{
					Name:  "Quick Javascript Example",
					Value: fmt.Sprintf("```javascript\nconst socket = new WebSocket('wss://%s/v1/presence/%s/ws');\n\nsocket.onopen = () => {\n  console.log('Connected to Strelp WebSocket');\n};\n\nsocket.onmessage = (event) => {\n  const data = JSON.parse(event.data);\n  console.log('Status instantly updated:', data.discord_status);\n};\n\nsocket.onclose = () => {\n  console.log('Disconnected. Consider adding reconnection logic.');\n};\n```", apiDomain, user.ID),
				},
				{
					Name:  "Common Errors and Fixes",
					Value: "**Error: Connection closed immediately**\nFix: Check if the WebSocket URL is correct and starts with wss:// instead of https://.\n\n**Error: No initial data**\nFix: The WebSocket sends an initial message upon connecting. Make sure your onmessage listener is registered immediately.",
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
					Content: fmt.Sprintf("Could not validate your GitHub token: %v. Make sure you are using a valid Personal Access Token.", err),
					Flags:   discordgo.MessageFlagsEphemeral,
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
					Content: "An internal error occurred. Please try again later.",
					Flags:   discordgo.MessageFlagsEphemeral,
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
					Content: "Failed to save your GitHub settings. Please try again. ",
					Flags:   discordgo.MessageFlagsEphemeral,
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
						Description: fmt.Sprintf("Your GitHub account **%s** has been linked. Your latest commits will appear in your presence within 5 minutes.", ghUsername),
						Color:       0x24292e,
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:  "Visibility",
								Value: fmt.Sprintf("Showing: %s repos", visibility),
							},
							{
								Name:  "Required Token Permissions",
								Value: "**Classic PATs**:\n`repo` — read access (required for private repos)\n`read:user` — read public profile info\n*(For public repos only, `public_repo` is sufficient)*\n\n**Fine-Grained PATs**:\nRequire **Read-only** access to **Contents** and **Metadata** for the selected repositories.",
							},
							{
								Name:  "Security",
								Value: "Your token has been encrypted with AES-256-GCM before being stored. It is never logged or exposed in any API response.",
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
				Content: "GitHub disconnected. Your commit data has been removed from your presence and your data has been cleared from our database, any loss of data now will not be our responsibility.",
				Flags:   discordgo.MessageFlagsEphemeral,
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
					Content: "You do not have permission to use this command.",
					Flags:   discordgo.MessageFlagsEphemeral,
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
				}

				if err := b.DB.SetPresence(ctx, userID, presence); err != nil {
					log.Printf("[Bot] Error updating presence for %s during sync: %v", userID, err)
					continue
				}
				count++
			}

			successMsg := fmt.Sprintf("Successfully synced %d users to the latest version.", count)
			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &successMsg,
			})
		}()
	}
}
