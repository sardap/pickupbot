package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/jonas747/dca"
	"github.com/kkdai/youtube/v2"
	"github.com/sardap/pickupbot/db"
	"golang.org/x/sync/semaphore"
)

type guildVoice struct {
	playing   *semaphore.Weighted
	channel   string
	playQueue *goconcurrentqueue.FIFO
}

type queueEntry struct {
	videoInfo   *youtube.Video
	textChannel string
	track       string
}

var (
	guildVoices map[string]*guildVoice
	gvMutex     *sync.RWMutex
)

func init() {
	guildVoices = make(map[string]*guildVoice)
	gvMutex = &sync.RWMutex{}
}

func getGuildVoiceLock(guildID string) *guildVoice {
	gvMutex.Lock()
	defer gvMutex.Unlock()
	res, ok := guildVoices[guildID]
	if !ok {
		res = &guildVoice{
			playing:   semaphore.NewWeighted(1),
			playQueue: goconcurrentqueue.NewFIFO(),
		}
		guildVoices[guildID] = res
	}
	return res
}

func getVideo(info *youtube.Video) (string, error) {
	var err error
	fileName := db.GetVideoPath(info.ID)

	if fileName == "" {
		fileName, err = db.NewVideo(info)
	}

	return fileName, err
}

func playLoop(
	s *discordgo.Session, v *discordgo.VoiceConnection,
	playing *semaphore.Weighted, queue *goconcurrentqueue.FIFO,
) {
	defer playing.Release(1)
	defer s.ChannelVoiceJoin(v.GuildID, "", false, false)

	var next *queueEntry
	for v.ChannelID != "" {
		var fileName string
		var track string
		var cID string
		if next == nil {
			topTemp, err := queue.Dequeue()
			if err != nil {
				return
			}
			top := topTemp.(queueEntry)
			next = &top
		}
		fileName, err := getVideo(next.videoInfo)
		if err != nil {
			s.ChannelMessageSend(cID, fmt.Sprintf("Unable to play %v %v", track, err))
			return
		}

		next = nil
		go func() {
			topTemp, err := queue.Dequeue()
			if err != nil {
				return
			}
			top := topTemp.(queueEntry)
			getVideo(top.videoInfo)
			next = &top
			log.Printf("got next ready\n")
		}()

		options := dca.StdEncodeOptions
		options.RawOutput = true
		options.Bitrate = 48
		options.Volume = 100
		options.Application = "audio"
		encodingSession, err := dca.EncodeFile(fileName, options)
		if err != nil {
			continue
		}

		go s.ChannelMessageSend(cID, fmt.Sprintf("Now picking up %s", track))

		v.Speaking(true)
		done := make(chan error)
		dca.NewStream(encodingSession, v, done)
		err = <-done
		v.Speaking(false)
		encodingSession.Cleanup()
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func playVideo(
	s *discordgo.Session, v *discordgo.VoiceConnection,
	txtCID string, videoInfo *youtube.Video, plCh chan error,
) {
	gvi := getGuildVoiceLock(v.GuildID)
	gvi.channel = v.ChannelID

	if videoInfo.Duration > time.Duration(10)*time.Minute {
		plCh <- fmt.Errorf(
			"can't play %s is too long it's over 10 mins %d",
			videoInfo.Title, videoInfo.Duration,
		)
		return
	}

	gvi.playQueue.Enqueue(queueEntry{
		videoInfo, txtCID, videoInfo.Title,
	})

	if gvi.playing.TryAcquire(1) {
		go playLoop(s, v, gvi.playing, gvi.playQueue)
	}

	plCh <- nil
}
