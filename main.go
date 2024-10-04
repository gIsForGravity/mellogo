package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Score struct {
	Minutes int
	Seconds int
}

func (s *Score) String() string {
	return fmt.Sprintf("%d:%d", s.Minutes, s.Seconds)
}

var (
	Token string // Discord bot token
	AppId string // Discord app id

	SubmitCommand = discordgo.ApplicationCommand{
		Name:        "submit",
		Description: "submit a time for the uniform speedrun",
		Type:        discordgo.ChatApplicationCommand,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "today",
				Description: "submit a time for today",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "time",
						Description: "the time to submit",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
						MinLength:   create(4),
					},
				},
			},
			{
				Name:        "for-date",
				Description: "submit a time for a certain date",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "date",
						Description: "(MM/DD/YYYY) the date to submit a time for",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
						MinLength:   create(10),
						MaxLength:   10,
					},
					{
						Name:        "time",
						Description: "(Minutes:Seconds) the time to submit",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
						MinLength:   create(4),
					},
				},
			},
		},
	}
	LeaderboardCommand = discordgo.ApplicationCommand{
		Name:        "leaderboard",
		Description: "shows the current leaderboard",
		Type:        discordgo.ChatApplicationCommand,
	}
)

func create(a int) *int {
	return &a
}

func init() {
	// Parse command line flags (bot token)
	flag.StringVar(&Token, "token", "", "Discord bot token")
	flag.StringVar(&AppId, "id", "", "Discord application id")
	flag.Parse()
}

func main() {
	// Create a bot session using the token provided in flag
	s, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Panicln("bot creation error:", err)
		return
	}

	// dictionary of command names to ids
	commandId := make(map[string]string)

	// Register handlers
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// name, ok := commandId[i.AppID]
		// if !ok {
		// 	fmt.Println("unknown command error: ", errors.New("unknown command with ID: "+i.AppID))
		// 	return
		// }
		name := i.ApplicationCommandData().Name
		fmt.Printf("%#v\n", i.ApplicationCommandData())

		switch name {
		case "submit":
			submitHandler(s, i)
		case "leaderboard":
			leaderboardHandler(s, i)
		default:
			// handle default case
			response := discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Sorry, I was unable to process your message.",
				},
			}
			// send response
			s.InteractionRespond(i.Interaction, &response)
		}
	})

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})

	// Specify intents
	s.Identify.Intents = discordgo.IntentGuildMessages | discordgo.IntentDirectMessages

	// Open channel
	err = s.Open()
	if err != nil {
		log.Panicln("bot connection error:", err)
	}

	defer s.Close()

	// Register commands
	registerCommand(s, &commandId, "submit", &SubmitCommand)
	registerCommand(s, &commandId, "leaderboard", &LeaderboardCommand)

	// Block
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop
}

func registerCommand(s *discordgo.Session, commandId *map[string]string, name string, command *discordgo.ApplicationCommand) {
	result, err := s.ApplicationCommandCreate(AppId, "", command)
	if err != nil {
		log.Panicln("command registration error: ", err)
	}

	// store resulting Id
	(*commandId)[result.ID] = name
}

func sendUserError(s *discordgo.Session, i *discordgo.InteractionCreate, update bool, ephemeral bool, msg string) {
	// Create embed
	embed := discordgo.MessageEmbed{
		Color:       0xff7081, // red rgb
		Description: msg,
	}

	// determine response type
	var responseType discordgo.InteractionResponseType
	if update {
		responseType = discordgo.InteractionResponseDeferredMessageUpdate
	} else {
		responseType = discordgo.InteractionResponseChannelMessageWithSource
	}
	// determine flags
	var messageFlags discordgo.MessageFlags
	if ephemeral {
		messageFlags = discordgo.MessageFlagsEphemeral
	} else {
		messageFlags = 0
	}

	if update {
		_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Embeds: &[]*discordgo.MessageEmbed{&embed},
		})
		if err != nil {
			fmt.Println("error sending user an error:", err)
		}
	} else {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: responseType,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{&embed},
				Flags:  messageFlags,
			},
		})
		if err != nil {
			fmt.Println("error sending user an error:", err)
		}
	}
}

func submitHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// acknowledge command is received and tell discord we will respond later
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		fmt.Println("error sending interaction response:", err)
	}
	// variables
	subOption := i.ApplicationCommandData().Options[0]
	options := subOption.Options
	userId := i.Member.User.ID

	var scoreArg int
	var date time.Time

	// handle date and score or just score
	switch subOption.Name {
	case "today":
		date = time.Now()
		scoreArg = 0 // options[0] should be time/score
	case "for-date":
		// options[0] should be date
		if options[0].Name != "date" {
			sendUserError(s, i, true, true, "There was an issue processing the command! Sorry ):")
			fmt.Println("error:", errors.New("second argument of /submit for-date is not 'date'")) // this should be reported as it is sTrAnGe (sppooookkyyy)
			return
		}
		// parse date
		var err error
		date, err = time.Parse("01/02/2006", options[0].StringValue())
		if err != nil {
			sendUserError(s, i, true, true, "I could not understand the date you sent. Please write it in the form MM/DD/YYYY.")
			return
		}
		// options[1] should be time/score
		scoreArg = 1
	default:
		sendUserError(s, i, true, true, "There was an issue processing the command! Sorry ):")
		fmt.Println("error:", errors.New("invalid command "+options[0].Name)) // also spooky
		return
	}

	if options[scoreArg].Name != "time" {
		fmt.Println("error:", errors.New("second argument of /submit today is not 'time'"))
		return
	}
	// parse score
	score, err := parseScore(options[scoreArg].StringValue())
	if err != nil {
		sendUserError(s, i, true, true, "I couldn't understand the time you submitted. Please make sure it's a real amount of time and it looks like Minutes:Seconds.")
		return
	}
	submitDate(s, date, userId, *score)
	// Tell user their time was submitted
	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &[]*discordgo.MessageEmbed{
			{
				Title:       "Submitted successfully!",
				Description: fmt.Sprintf("Your time of `%s` on `%s` has been submitted successfully.", score.String(), date.Format("01/02/2006")),
			},
		},
	})
	if err != nil {
		fmt.Println("error sending interaction response:", err)
	}
}

func submitDate(_ *discordgo.Session, d time.Time, userId string, score Score) {
	// TODO: implement date submission
	fmt.Printf("Score submitted for %s: %s - %s\n", userId, d.Format("01/02/2006"), score.String())
}

func parseScore(score string) (*Score, error) {
	segments := strings.Split(score, ":")

	if len(segments) != 2 {
		return nil, errors.New("too many ':' in score string")
	}

	minutes, errM := strconv.Atoi(segments[0])
	seconds, errS := strconv.Atoi(segments[1])

	fmt.Printf("time: %d:%d\n", minutes, seconds)

	if errM != nil || errS != nil {
		return nil, errors.New("unable to parse numbers in score string")
	}

	if minutes < 0 || 60 <= minutes {
		return nil, errors.New("minutes outside range")
	}

	if seconds < 0 || 60 <= seconds {
		return nil, errors.New("seconds outside range")
	}

	return &Score{minutes, seconds}, nil
}

func leaderboardHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// acknowledge command is received and tell discord we will respond later
	// s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
	// 	Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	// })
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "yea oops haven't implemented this yet lol", // TODO
		},
	})
}