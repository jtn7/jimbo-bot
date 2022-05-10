package main

import (
	"bimbot/jimlib"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/bwmarrin/discordgo"
)

var botToken string

// Initialize DB and prepared statements
func init() {
	botToken = os.Getenv("TOKEN")

	// Initialize DB driver and open sqlite DB
	db, err := sql.Open("sqlite3", "./jimbo-bot.db")
	if err != nil {
		log.Fatal("could not open db file: ", err)
	}
	jimlib.AddPreparedStatements(db)
}

var est, _ = time.LoadLocation("America/New_York")

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
	// Confirming event creation
	case "confirm":
		s.InteractionRespond(i.Interaction, willUpdate)

		eventDate, _ := time.ParseInLocation("Mon 01/02/06 3:04 PM", i.Message.Embeds[0].Fields[0].Value, est)
		// Insert event into db
		_, err := jimlib.InsertEvent.Exec(i.Message.ID, i.Message.Embeds[0].Title, fmt.Sprintf("%d", eventDate.Unix()), i.Message.Embeds[0].Description, "", "")
		if err != nil {
			log.Println("ERROR: Could not insert into db:", err)
		}

		// Edit event message
		_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:      i.Message.ID,
			Channel: i.Message.ChannelID,
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
	// Delete event interaction as opposed to confirming and committing it
	case "unconfirm":
		s.InteractionRespond(i.Interaction, willUpdate)
		s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
	case "going":
		s.InteractionRespond(i.Interaction, willUpdate)

		// Get people going to the event
		var result string
		err := jimlib.GetGoing.QueryRow(i.Message.ID).Scan(&result)
		if err != nil {
			log.Printf("ERROR: Could not query getGoing with %s: %v", i.Message.ID, err)
			return
		}

		// Set going slice and protect against empty going list
		var going []string
		if len(result) > 0 {
			going = strings.Split(result, ",;")
		}

		// Check if person is already going
		attending := make(map[string]bool)
		for _, person := range going {
			attending[person] = true
		}
		if attending[i.Member.User.Username] {
			return
		}

		// Get users in the flaking list
		err = jimlib.GetFlaking.QueryRow(i.Message.ID).Scan(&result)
		if err != nil {
			log.Printf("ERROR: Could not query getFlaking with %s: %v", i.Message.ID, err)
			return
		}
		var flaking []string
		if len(result) > 0 {
			flaking = strings.Split(result, ",;")
		}

		// Check if the person is in the flaking list
		for ind, person := range flaking {
			// If user is in the flaking list remove them from it
			if i.Member.User.Username == person {
				flaking[ind] = flaking[len(flaking)-1]
				flaking = flaking[:len(flaking)-1]

				_, err = jimlib.UpdateEventFlaking.Exec(strings.Join(flaking, ",;"), i.Message.ID)
				if err != nil {
					log.Printf("ERROR: Could not remove user from flaking list: updateEventFlaking with %s: %v", i.Message.ID, err)
				}
				break
			}
		}

		going = append(going, i.Member.User.Username)

		if len(flaking) > 0 {
			_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:      i.Message.ID,
				Channel: i.ChannelID,
				Embeds: []*discordgo.MessageEmbed{
					i.Message.Embeds[0],
					{
						Title: "Attendees",
						Fields: []*discordgo.MessageEmbedField{
							{Name: "Going", Value: strings.Join(going, ", ")},
							{Name: "Flaking", Value: strings.Join(flaking, ", ")},
						},
					},
				},
			})
			if err != nil {
				log.Printf("ERROR: Could not edit event message %s: %v", i.Message.ID, err)
				return
			}
		} else {
			_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:      i.Message.ID,
				Channel: i.ChannelID,
				Embeds: []*discordgo.MessageEmbed{
					i.Message.Embeds[0],
					{
						Title: "Attendees",
						Fields: []*discordgo.MessageEmbedField{
							{Name: "Going", Value: strings.Join(going, ", ")},
						},
					},
				},
			})
			if err != nil {
				log.Printf("ERROR: Could not edit event message %s: %v", i.Message.ID, err)
				return
			}
		}

		// Insert user into going for event
		_, err = jimlib.UpdateEventGoing.Exec(strings.Join(going, ",;"), i.Message.ID)
		if err != nil {
			log.Printf("ERROR: Could not updateEventGoing with %s: %v", i.Message.ID, err)
		}
	case "flaking":
		s.InteractionRespond(i.Interaction, willUpdate)

		// Get flaking list for event
		var result string
		err := jimlib.GetFlaking.QueryRow(i.Message.ID).Scan(&result)
		if err != nil {
			log.Printf("ERROR: Could not query getFlaking with %s: %v", i.Message.ID, err)
			return
		}
		// Put flaking list into slice
		var flaking []string
		if len(result) > 0 {
			flaking = strings.Split(result, ",;")
		}

		// Check if person is already flaking
		flakes := make(map[string]bool)
		for _, person := range flaking {
			flakes[person] = true
		}
		if flakes[i.Member.User.Username] {
			return
		}

		// Get going list for event
		err = jimlib.GetGoing.QueryRow(i.Message.ID).Scan(&result)
		if err != nil {
			log.Printf("ERROR: Could not query getGoing with %s: %v", i.Message.ID, err)
			return
		}
		// Put going list into slice
		var going []string
		if len(result) > 0 {
			going = strings.Split(result, ",;")
		}

		// Check if the person is in the going list
		for ind, person := range going {
			// If user is in the going list remove them from it
			if i.Member.User.Username == person {
				going[ind] = going[len(going)-1]
				going = going[:len(going)-1]

				_, err = jimlib.UpdateEventGoing.Exec(strings.Join(going, ",;"), i.Message.ID)
				if err != nil {
					log.Printf("ERROR: Could not remove user from going list: updateEventGoing with %s: %v", i.Message.ID, err)
				}
				break
			}
		}

		flaking = append(flaking, i.Member.User.Username)

		if len(going) > 0 {
			_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:      i.Message.ID,
				Channel: i.ChannelID,
				Embeds: []*discordgo.MessageEmbed{
					i.Message.Embeds[0],
					{
						Title: "Attendees",
						Fields: []*discordgo.MessageEmbedField{
							{Name: "Going", Value: strings.Join(going, ", ")},
							{Name: "Flaking", Value: strings.Join(flaking, ", ")},
						},
					},
				},
			})
			if err != nil {
				log.Printf("ERROR: Could not edit event message %s: %v", i.Message.ID, err)
				return
			}
		} else {
			_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:      i.Message.ID,
				Channel: i.ChannelID,
				Embeds: []*discordgo.MessageEmbed{
					i.Message.Embeds[0],
					{
						Title: "Attendees",
						Fields: []*discordgo.MessageEmbedField{
							{Name: "Flaking", Value: strings.Join(flaking, ", ")},
						},
					},
				},
			})
			if err != nil {
				log.Printf("ERROR: Could not edit event message %s: %v", i.Message.ID, err)
				return
			}
		}

		// Insert user into flaking for event
		_, err = jimlib.UpdateEventFlaking.Exec(strings.Join(flaking, ",;"), i.Message.ID)
		if err != nil {
			log.Printf("ERROR: Could not updateEventFlaking with %s: %v", i.Message.ID, err)
		}
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

	r = &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: ":star: **NEW EVENT** :star:",
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
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       title,
					Description: description,
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:  "Time",
							Value: date.Format("Mon 01/02/06 3:04 PM"),
						},
					},
				},
			},
		},
	}

	return
}

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
