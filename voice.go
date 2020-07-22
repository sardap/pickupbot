package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/enriquebris/goconcurrentqueue"
	"github.com/jonas747/dca"
	"github.com/rylio/ytdl"
	"github.com/sardap/pickupbot/db"
	"golang.org/x/sync/semaphore"
)

type guildVoice struct {
	playing   *semaphore.Weighted
	channel   string
	playQueue *goconcurrentqueue.FIFO
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

func playLoop(
	s *discordgo.Session, v *discordgo.VoiceConnection,
	playing *semaphore.Weighted, queue *goconcurrentqueue.FIFO,
) {

	for queue.GetLen() > 0 {
		nextFileTmp, _ := queue.Dequeue()

		fileName := nextFileTmp.(string)

		options := dca.StdEncodeOptions
		options.RawOutput = true
		options.Bitrate = 48
		options.Volume = 100
		options.Application = "audio"
		encodingSession, err := dca.EncodeFile(fileName, options)
		if err != nil {
			continue
		}

		v.Speaking(true)
		done := make(chan error)
		dca.NewStream(encodingSession, v, done)
		err = <-done
		if err != nil && err != io.EOF {
		}
		v.Speaking(false)
		encodingSession.Cleanup()
	}

	playing.Release(1)
	s.ChannelVoiceJoin(v.GuildID, "", false, false)
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
	videoInfo *ytdl.VideoInfo, plCh chan error,
) {
	gvi := getGuildVoiceLock(v.GuildID)

	if videoInfo.Duration > time.Duration(10)*time.Minute {
		plCh <- fmt.Errorf(
			"can't play %s is too long it's over 10 mins %d",
			videoInfo.Title, videoInfo.Duration,
		)
		return
	}
	fileName := db.GetVideoPath(videoInfo.ID)

	if fileName == "" {
		var err error
		fileName, err = db.NewVideo(videoInfo)
		if err != nil {
			plCh <- err
			return
		}
	}

	gvi.playQueue.Enqueue(fileName)

	if gvi.playing.TryAcquire(1) {
		go playLoop(s, v, gvi.playing, gvi.playQueue)
	}

	plCh <- nil
}
