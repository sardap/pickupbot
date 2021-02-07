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
	"github.com/kkdai/youtube/v2"
	"github.com/sardap/discgov"
	"github.com/sardap/discom"
	"github.com/sardap/pickupbot/db"
	"github.com/sardap/pickupbot/env"
	"github.com/sardap/pickupbot/isitska"
	"github.com/sardap/pickupbot/translator"
)

const isItSkaPattern = "is this ska\\?"
const playSomeSkaPattern = "play some ska!"
const playThisSkaPattern = "play this ska"
const isSkaParsePattern = "((artist\\$ (?P<artist>.*?))?title\\$ (?P<title>.*)|url\\$ (?P<url>(https:\\/\\/open\\.spotify\\.com/track/([a-zA-Z0-9]+)))(.*))"

var (
	commandSet *discom.CommandSet
	skaInvoker isitska.Invoker
)

func init() {
	db.Connect()

	skaInvoker = isitska.Invoker{
		Endpoint: env.IsItSkaEndpoint,
	}

	commandSet = discom.CreateCommandSet(regexp.MustCompile(env.CmdPrefix))

	err := commandSet.AddCommand(discom.Command{
		Re:      regexp.MustCompile(isItSkaPattern),
		Handler: isItSkaCommand, CaseInsensitive: true,
		Example: "is this ska? title$ call me maybe",
		Description: "is this ska? artist(optional)$ {Artist name here} title$ {track title here} |or| is this ska? url$ {URL HERE}\n" +
			"What it does? will return if found track is ska or not.",
	})
	if err != nil {
		panic(err)
	}

	err = commandSet.AddCommand(discom.Command{
		Re:      regexp.MustCompile(playSomeSkaPattern),
		Handler: playSomeSka, CaseInsensitive: true,
		Description: "Will join your chat channel and play ska.",
	})
	if err != nil {
		panic(err)
	}

	err = commandSet.AddCommand(discom.Command{
		Re:      regexp.MustCompile(playThisSkaPattern),
		Handler: playThisSka, CaseInsensitive: true,
		Example: "play this ska! title$ call me maybe",
		Description: "play this ska! artist(optional)$ {Artist name here} title$ {track title here} |or| is this ska? url$ {URL HERE}\n" +
			"What it does? Will play the given song if it's ska.",
	})
	if err != nil {
		panic(err)
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
		return nil, fmt.Errorf("could not find your discord server")
	}

	targetChannel, err := getUserChannel(m.GuildID, m.Author.ID, guild.Channels)
	if err != nil {
		return nil, fmt.Errorf("Must be in a channel on the target server to pick it up")
	}

	return s.ChannelVoiceJoin(m.GuildID, targetChannel, false, true)
}

func connectAndPlay(videoInfo *youtube.Video, s *discordgo.Session, m *discordgo.MessageCreate) error {
	connection, err := joinCaller(s, m)
	if err != nil {
		return err
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

	return nil
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
	err = connectAndPlay(videoInfo, s, m)
	if err != nil {
		s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"<@%s> Unable to join voice channel %v!",
				m.Author.ID, err,
			),
		)
	}
}

func playSomeSka(s *discordgo.Session, m *discordgo.MessageCreate) {
	tracks, err := skaInvoker.GetNSka(9)
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

	for _, track := range tracks {
		videoInfo, err := translator.ToYTURL(track.Title, track.Artists)
		if err != nil {
			log.Printf("Error pulling video info %v\n", err)
			continue
		}

		err = connectAndPlay(videoInfo, s, m)
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
	token := strings.Replace(env.DiscordToken, "\"", "", -1)
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Printf("unable to create new discord instance")
		log.Fatal(err)
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	discord.AddHandler(voiceStateUpdate)
	discord.AddHandler(commandSet.Handler)

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
