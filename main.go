package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/rylio/ytdl"
	"github.com/sardap/discgov"
	"github.com/sardap/pickupbot/isitska"
	"github.com/sardap/pickupbot/translator"
	"github.com/sardap/voterigging"
)

const messageCommandPattern = "pub\\$"
const helpPattern = "help"
const isItSkaPattern = "is this ska\\?"
const playSomeSkaPattern = "play some ska!"
const playThisSkaPattern = "play this ska"
const isSkaParsePattern = "((artist\\$ (?P<artist>.*?))?title\\$ (?P<title>.*)|url\\$ (?P<url>(https:\\/\\/open\\.spotify\\.com/track/([a-zA-Z0-9]+)))(.*))"

type commandHandler func(s *discordgo.Session, m *discordgo.MessageCreate)

type commandInfo struct {
	re      *regexp.Regexp
	handler commandHandler
	desc    string
}

var (
	commands   []commandInfo
	skaInvoker isitska.Invoker
)

func init() {
	skaInvoker = isitska.Invoker{
		Endpoint: os.Getenv("IS_IT_SKA_ENDPOINT"),
	}

	commands = []commandInfo{
		commandInfo{
			re:      regexp.MustCompile(isItSkaPattern),
			handler: isItSkaCommand,
			desc: "is this ska? artist(optional)$ {Artist name here} title$ {track title here} |or| is this ska? url$ {URL HERE}\n" +
				"What it does? will return if found track is ska or not.\n" +
				"Example: pub$ is this ska? title$ call me maybe\n" +
				"Example: pub$ is this ska? url$ https://open.spotify.com/track/7g96GMqMFfkrzEvDwSIWzQ?si=W4sESgdbSUuCyYncWM4tUA",
		},
		commandInfo{
			re:      regexp.MustCompile(helpPattern),
			handler: printHelp,
			desc:    "help command prints this message",
		},
		commandInfo{
			re:      regexp.MustCompile(playSomeSkaPattern),
			handler: playSomeSka,
			desc: "play some ska!\n" +
				"Will join your chat channel and play ska.",
		},
		commandInfo{
			re:      regexp.MustCompile(playThisSkaPattern),
			handler: playThisSka,
			desc: "play this ska! artist(optional)$ {Artist name here} title$ {track title here} |or| is this ska? url$ {URL HERE}\n" +
				"What it does? Will play the given song if it's ska.\n" +
				"Example: pub$ play this ska! title$ call me maybe\n" +
				"Example: pub$ play this ska! url$ https://open.spotify.com/track/7g96GMqMFfkrzEvDwSIWzQ?si=W4sESgdbSUuCyYncWM4tUA",
		},
	}
}

// Returns title, artist, url
func parseSongInfo(str string) (string, string, string) {
	var (
		title  string
		artist string
		url    string
	)

	re := regexp.MustCompile(isSkaParsePattern)
	groupNames := re.SubexpNames()
	for _, match := range re.FindAllStringSubmatchIndex(string(str), -1) {
		for i := 0; i < len(match); i += 2 {
			var name string
			if i > 0 {
				name = groupNames[i/2]
			} else {
				name = groupNames[0]
			}
			if name != "" {
				switch name {
				case "artist":
					if match[i] > -1 {
						artist = str[match[i]:match[i+1]]
					}
				case "title":
					if match[i] > -1 {
						title = str[match[i]:match[i+1]]
					}
				case "url":
					if match[i] > -1 {
						url = str[match[i]:match[i+1]]
					}
				}
			}
		}
	}

	return title, artist, url
}

func isItSka(msg string) (*isitska.TrackInfo, error) {
	title, artist, url := parseSongInfo(msg)

	var trackInfo isitska.TrackInfo
	var err error
	if title != "" {
		trackInfo, err = skaInvoker.ByName(title, artist)
	} else if url != "" {
		trackInfo, err = skaInvoker.ByID(url)
	} else {
		return nil, errors.New("Must set title or url")
	}
	return &trackInfo, err
}

func isItSkaCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	re := regexp.MustCompile(isSkaParsePattern)
	toCheck := []byte(strings.ToLower(m.Content))
	if !re.Match(toCheck) {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> Invalid parameters must match this regex \"%s\"",
				m.Author.ID, isSkaParsePattern,
			),
		)
		return
	}

	trackInfo, err := isItSka(m.Content)
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> Error: %s",
				m.Author.ID, err.Error(),
			),
		)
		return
	}

	var skaStr string
	if trackInfo.IsSka() {
		skaStr = "Yeah it is PICK IT UP!"
	} else {
		skaStr = "Drop this garbage (it's not ska)."
	}

	s.ChannelMessageSend(
		m.ChannelID,
		fmt.Sprintf(
			"<@%s> is %s by %s ska? %s %s",
			m.Author.ID, trackInfo.Title,
			trackInfo.ArtistsStr(), skaStr, trackInfo.TrackLink,
		),
	)
}

func getUserChannel(guildID, userID string, channels []*discordgo.Channel) (string, error) {
	for _, channel := range channels {
		users := discgov.GetUsers(guildID, channel.ID)
		for _, userID := range users {
			if userID == userID {
				return channel.ID, nil
			}
		}
	}

	return "", errors.New("Could not find user")
}

func joinCaller(
	s *discordgo.Session, m *discordgo.MessageCreate,
) (voice *discordgo.VoiceConnection, err error) {
	guild, err := s.State.Guild(m.GuildID)
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> could not get your discord server info are you in a server?",
				m.Author.ID,
			),
		)
		return
	}

	targetChannel, err := getUserChannel(m.GuildID, m.Author.ID, guild.Channels)
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> you must be in a channel on this server to pick it up!",
				m.Author.ID,
			),
		)
		return
	}

	return s.ChannelVoiceJoin(m.GuildID, targetChannel, false, true)
}

func playVideoOuter(videoInfo *ytdl.VideoInfo, s *discordgo.Session, m *discordgo.MessageCreate) {
	connection, err := joinCaller(s, m)
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> Unable to join voice channel %v!",
				m.Author.ID, err,
			),
		)
		return
	}
	plCh := make(chan error)
	go playVideo(s, connection, m.ChannelID, videoInfo, plCh)

	err = <-plCh
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> Error %v!",
				m.Author.ID, err,
			),
		)
	} else {
		s.MessageReactionAdd(
			m.ChannelID, m.ID, "💦",
		)
	}
}

func playThisSka(s *discordgo.Session, m *discordgo.MessageCreate) {
	trackInfo, err := isItSka(m.Content)
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> Error: %s",
				m.Author.ID, err.Error(),
			),
		)
		return
	}

	if !trackInfo.IsSka() {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> How can you ask me to play %s by %s it's not ska drop it the fuck off.",
				m.Author.ID, trackInfo.Title, trackInfo.ArtistsStr(),
			),
		)
		return
	}

	videoInfo, err := translator.ToYTURL(trackInfo.Title, trackInfo.Artists)
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> Error: %s",
				m.Author.ID, err.Error(),
			),
		)
		return
	}
	playVideoOuter(videoInfo, s, m)
}

func playSomeSka(s *discordgo.Session, m *discordgo.MessageCreate) {
	tracks, err := skaInvoker.GetNSka(5)
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> Error: %s",
				m.Author.ID, err.Error(),
			),
		)
		return
	}

	count := 0
	for _, track := range tracks {
		videoInfo, err := translator.ToYTURL(track.Title, track.Artists)
		if err != nil {
			log.Printf("Error pulling video info %v\n", err)
			continue
		}

		playVideoOuter(videoInfo, s, m)
		count++
	}

	s.ChannelMessageSend(
		m.ChannelID,
		fmt.Sprintf(
			"<@%s> Enqueued %d ska songs",
			m.Author.ID, count,
		),
	)
}

func printHelp(s *discordgo.Session, m *discordgo.MessageCreate) {
	message := fmt.Sprintf(
		"<@%s> this is a program for picking it up. Commands are\n",
		m.Author.ID,
	)

	for _, c := range commands {
		message += c.desc + "\n"
		message += "🏁\n"
	}

	s.ChannelMessageSend(
		m.ChannelID,
		message,
	)
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	re := regexp.MustCompile(messageCommandPattern)
	if re.Match([]byte(strings.ToLower(m.Content))) {
		triggered := false
		for _, c := range commands {
			if c.re.Match([]byte(strings.ToLower(m.Content))) {
				go c.handler(s, m)
				triggered = true
				break
			}
		}

		if !triggered {
			s.ChannelMessageSend(
				m.ChannelID,
				fmt.Sprintf(
					"<@%s> I don't recognize that command type \"%s help\" for info on commands",
					m.Author.ID, messageCommandPattern,
				),
			)
		}
	}
}

func voiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	discgov.UserVoiceTrackerHandler(s, v)
	gvi := getGuildVoiceLock(v.GuildID)

	if v.UserID == s.State.User.ID {
		if gvi.channel != v.ChannelID {
			gvi.channel = v.ChannelID
		}
		return
	}

	if len(discgov.GetUsers(v.GuildID, gvi.channel)) == 0 {
		s.ChannelVoiceJoin(v.GuildID, "", true, true)
	}
}

func main() {
	token := strings.Replace(os.Getenv("DISCORD_AUTH"), "\"", "", -1)
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Printf("unable to create new discord instance")
		log.Fatal(err)
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	discord.AddHandler(voiceStateUpdate)
	discord.AddHandler(messageCreate)
	discord.AddHandler(voterigging.VoteReactCreateMessage)
	discord.AddHandler(voterigging.VoteReactUpdateMessage)

	// Open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	discord.UpdateStatus(-1, "🏁pub$ for help!🏁")

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	discord.Close()

}
