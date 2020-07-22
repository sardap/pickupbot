package db

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/rylio/ytdl"
)

type videoInfo struct {
	Path           string    `json:"path"`
	LastUpdateTime time.Time `json:"last_update_time"`
}

type videosInfo struct {
	Data map[string]videoInfo `json:"data"`
}

var (
	infoLock   *sync.RWMutex
	data       videosInfo
	dbFilePath string
)

func init() {
	infoLock = &sync.RWMutex{}

	dbFilePath = os.Getenv("DB_FILE_PATH")

	reader, err := os.Open(dbFilePath)
	if err == nil {
		err = json.NewDecoder(reader).Decode(&data)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		data = videosInfo{
			Data: make(map[string]videoInfo),
		}
		data.Save()
	}

	go cleanOld()
}

func (v *videosInfo) Save() {
	encoded, err := json.Marshal(*v)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(dbFilePath, encoded, 0644)
}

//GetVideoPath GetVideoPath
func GetVideoPath(id string) string {
	infoLock.Lock()
	defer infoLock.Unlock()
	result, ok := data.Data[id]
	if !ok {
		return ""
	}
	result.LastUpdateTime = time.Now().UTC()
	data.Save()
	return result.Path
}

//NewVideo NewVideo
func NewVideo(ytVideoInfo *ytdl.VideoInfo) (string, error) {
	infoLock.Lock()
	defer infoLock.Unlock()
	path := fmt.Sprintf("videos/%s.mp4", ytVideoInfo.ID)
	data.Data[ytVideoInfo.ID] = videoInfo{path, time.Now().UTC()}

	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	client := ytdl.DefaultClient
	err = client.Download(
		context.TODO(), ytVideoInfo,
		ytVideoInfo.Formats[0], file,
	)
	if err != nil {
		return "", nil
	}

	data.Save()
	return path, nil
}

func cleanOld() {
	for {
		cutOffDate := time.Now().UTC().Add(-time.Duration(48) * time.Hour)

		infoLock.Lock()
		removeStack := make([]string, 0)
		for k, v := range data.Data {
			if v.LastUpdateTime.Before(cutOffDate) {
				removeStack = append(removeStack, k)
			}
		}

		for _, k := range removeStack {
			path := data.Data[k].Path
			log.Printf("Deleting %s", path)
			err := os.Remove(path)
			if err == nil {
				delete(data.Data, k)
			}
		}
		data.Save()
		infoLock.Unlock()

		time.Sleep(time.Duration(60) * time.Second)
	}
}
