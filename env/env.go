package env

import (
	"os"
)

var (
	//DBPath where the DB file is
	DBPath string
	//IsItSkaEndpoint is it ska api endpoint
	IsItSkaEndpoint string
	//CmdPrefix used to prefix all discord commands
	CmdPrefix string
	//DiscordToken discord token
	DiscordToken string
	//VideosPath path to saved videos
	VideosPath string
	//YoutubeAPIKey youtube API
	YoutubeAPIKey string
)

func init() {
	DBPath = os.Getenv("DB_PATH")
	IsItSkaEndpoint = os.Getenv("IS_IT_SKA_ENDPOINT")
	CmdPrefix = os.Getenv("DISCORD_COMMAND_PREFIX")
	if CmdPrefix == "" {
		CmdPrefix = "pub\\$"
	}
	DiscordToken = os.Getenv("DISCORD_AUTH")
	VideosPath = os.Getenv("VIDEOS_PATH")
	YoutubeAPIKey = os.Getenv("YOUTUBE_API_KEY")
}
