package bot

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/spacebxr/strelp/internal/database"
	"github.com/spacebxr/strelp/internal/models"
)

type Bot struct {
	Session *discordgo.Session
	DB      *database.Database
}

func NewBot(token string, db *database.Database) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		Session: dg,
		DB:      db,
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
	ctx := context.Background()

	// Check if user is opted-in
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

	// Retrieve full User object since PresenceUpdate often omits fields
	userObj := p.User
	if cachedMember, err := s.State.Member(p.GuildID, p.User.ID); err == nil {
		userObj = cachedMember.User
	} else if cachedUser, err := s.User(p.User.ID); err == nil {
		userObj = cachedUser
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
	}

	if err := b.DB.SetPresence(ctx, p.User.ID, presence); err != nil {
		log.Printf("[Bot] Error saving presence for %s: %v", p.User.ID, err)
	}
}

func (b *Bot) onMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
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
	}

	_, err := b.Session.ApplicationCommandBulkOverwrite(b.Session.State.User.ID, guildID, commands)
	if err == nil && guildID != "" {
		b.Session.ApplicationCommandBulkOverwrite(b.Session.State.User.ID, "", nil)
	}
	return err
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
		presence := &models.Presence{
			User: models.User{
				ID:         user.ID,
				Username:   user.Username,
				GlobalName: user.GlobalName,
				Avatar:     user.Avatar,
			},
			DiscordStatus: "online",
			Activities:    []models.Activity{},
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
	}
}
