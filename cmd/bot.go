package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/bwmarrin/discordgo"
)

var botToken string
var insertEvent *sql.Stmt
var updateEvent *sql.Stmt

func init() {
	botToken = os.Getenv("TOKEN")

	// Initialize DB driver and open sqlite DB
	db, err := sql.Open("sqlite3", "./jimbo-bot.db")
	if err != nil {
		log.Fatal("could not open db file: ", err)
	}

	insertEvent, err = db.Prepare(`insert into Events(MessageID, Name, Date, Details, Going, Flaking)
										values(?,?,?,?,?,?)`)
	if err != nil {
		log.Fatal("failed to create insertEvent prepared statement: ", err)
	}

	updateEvent, err = db.Prepare(`update Events set Going = ? where MessageID = ?`)
	if err != nil {
		log.Fatal("failed to create updateEvent prepared statement: ", err)
	}
}

func main() {
	if botToken == "" {
		fmt.Fprintln(os.Stderr, "Your bot token is empty\nSet \"TOKEN\" and try again.")
		os.Exit(1)
	}

	// Create a new Discord session using the provided bot token.
	// Session is just a struct; no connecting happens here.
	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(InteractionHandler)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}
	defer dg.Close()

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

// InteractionHandler responds to Interactions
func InteractionHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Data.Type() {
	// Maps slash commands to handler functions
	case discordgo.InteractionApplicationCommand:
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	// Handles MessageComponent events (Buttons)
	case discordgo.InteractionMessageComponent:
		MessageComponentHandler(s, i)
	}
}

var willUpdate = &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredMessageUpdate}

// MessageComponentHandler responds to MessageComponent interactions.
// MessageComponent interactions are button presses.
func MessageComponentHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.MessageComponentData().CustomID {
	case "confirm":
		s.InteractionRespond(i.Interaction, willUpdate)
		_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:      i.Message.ID,
			Channel: i.Message.ChannelID,
			Content: &i.Message.Content,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "I'm going",
							Style:    discordgo.SuccessButton,
							CustomID: "going",
						},
						discordgo.Button{
							Label:    "I'm FLAKING",
							Style:    discordgo.DangerButton,
							CustomID: "flaking",
						},
					},
				},
			},
		})
		if err != nil {
			log.Println("error editing interaction: ", err)
		}
	// This is for not committing an event
	case "unconfirm":
		s.InteractionRespond(i.Interaction, willUpdate)
		s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
	case "going":
		// TODO
		// need to abstract getting the attendee list and appending another
		// Add user to 'going' list
		// _, err := updateEvent.Exec(i.Member.User.ID, i.Message.ID)
		// if err != nil {
		// 	log.Fatal("could not insert into Events table: ", err)
		// }

		response := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "See you there  :sunglasses:",
				// Only the user who pressed the button will see this message
				Flags: 1 << 6,
			},
		}
		s.InteractionRespond(i.Interaction, response)

	case "flaking":
		response := &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "what the...  :rage:",
				// Only the user who pressed the button will see this message
				Flags: 1 << 6,
			},
		}
		s.InteractionRespond(i.Interaction, response)
	}
}

// interactionEventHandler is an event handler for InteractionCreate events.
type interactionEventHandler func(*discordgo.Session, *discordgo.InteractionCreate)

var commandHandlers = map[string]interactionEventHandler{
	"event":     SlashEventHandler,
	"vibecheck": SlashVibeCheck,
}

// Handles /event create
func eventCreate(c *discordgo.ApplicationCommandInteractionDataOption) (r *discordgo.InteractionResponse) {
	title := c.Options[0].StringValue()
	description := c.Options[1].StringValue()
	date, err := time.ParseInLocation("01/02/06 15:04", c.Options[2].StringValue(), est)
	if err != nil || len(c.Options[2].StringValue()) != 14 {
		log.Println("Could not parse date string: ",
			c.Options[1].StringValue(),
			err,
		)
		r = &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You formatted the date wrong. The format is \"MM/DD/YY HH:MM\" Try again.",
				Flags:   1 << 6,
			},
		}
		return
	}

	message := fmt.Sprintf(
		":star: **NEW EVENT** :star:\n\n**%s**\n%s\n\n:calendar: %s",
		title, description, date.Format("Mon 01/02/06 15:04"),
	)

	r = &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Confirm Event",
							Style:    discordgo.SuccessButton,
							CustomID: "confirm",
						},
						discordgo.Button{
							Label:    "Delete",
							Style:    discordgo.DangerButton,
							CustomID: "unconfirm",
						},
					},
				},
			},
		},
	}

	// _, err = insertEventStmt.Exec(title, date.Format("Mon 01/02/06 15:04"), description, "", "")
	// if err != nil {
	// 	r = &discordgo.InteractionResponse{
	// 		Type: discordgo.InteractionResponseChannelMessageWithSource,
	// 		Data: &discordgo.InteractionResponseData{
	// 			Content: "Could not register event. Contact developer.",
	// 			Flags:   1 << 6,
	// 		},
	// 	}
	// 	log.Println("could not insert record into Events table: ", err)
	// }

	return
}

var est, _ = time.LoadLocation("America/New_York")

// Handles /event
func SlashEventHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// This is how to get a user ID to @
	// target := i.ApplicationCommandData().Options[0].UserValue(s).ID

	var response *discordgo.InteractionResponse
	command := i.ApplicationCommandData().Options[0]
	switch command.Name {
	// case `get`:
	// 	switch options[0].Options[0].Name {
	// 	case `all`:
	// 	case `specific`:
	// 	}
	case `create`:
		response = eventCreate(command)
	}

	if response != nil {
		s.InteractionRespond(i.Interaction, response)
	}
}

// SlashVibeCheck handles /vibecheck
func SlashVibeCheck(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options

	r, err := rand.Int(rand.Reader, big.NewInt(6))
	if err != nil {
		log.Println("could not get crypto/rand int:", err)
		return
	}

	var message string
	if len(options) > 0 {
		user := options[0].UserValue(s)
		if r.Int64() < 3 {
			message = fmt.Sprintf("**%s** has passed the vibe check  :sunglasses:", user.Username)
		} else {
			message = fmt.Sprintf("**%s** has failed the vibe check  :thumbsdown:", user.Username)
		}
	} else {
		atID := i.Member.User.ID
		if r.Int64() < 3 {
			message = fmt.Sprintf("<@%s> has passed the vibe check  :sunglasses:", atID)
		} else {
			message = fmt.Sprintf("<@%s> has failed the vibe check  :thumbsdown:", atID)
		}
	}

	sendInteractionResponse(s, i, message)
}

func sendInteractionResponse(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	}
	s.InteractionRespond(i.Interaction, response)
}
