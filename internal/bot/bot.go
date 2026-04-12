package bot

import (
	"context"
	"fmt"
	"log"

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

	presence := &models.Presence{
		User: models.User{
			ID:         p.User.ID,
			Username:   p.User.Username,
			GlobalName: p.User.GlobalName,
			Avatar:     p.User.Avatar,
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
	}

	for _, cmd := range commands {
		_, err := b.Session.ApplicationCommandCreate(b.Session.State.User.ID, guildID, cmd)
		if err != nil {
			return err
		}
	}
	return nil
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

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "🚀 **Strelp tracking started!** Your presence is now being tracked and your API is live (Powered by PostgreSQL).",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

	case "stop":
		b.DB.DeletePresence(ctx, user.ID)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "🛑 **Strelp tracking stopped.** Your data has been deleted from the database.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}
}
