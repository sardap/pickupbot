package db

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/kkdai/youtube/v2"
)

var (
	rClient *redis.Client
)

func init() {
	go cleanOld()

	dbNum, err := strconv.Atoi(os.Getenv("REDIS_DB_NUMBER"))
	if err != nil {
		panic(err)
	}

	rClient = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDRESS"),
		Password: os.Getenv("REDIS_PASSWORD"), // no password set
		DB:       dbNum,                       // use default DB
	})
}

//GetClient returns redis client
func GetClient() *redis.Client {
	return rClient
}

//GetVideoPath GetVideoPath
func GetVideoPath(id string) string {
	return rClient.Get(id).Val()
}

//NewVideo NewVideo
func NewVideo(video *youtube.Video) (string, error) {
	path := fmt.Sprintf("videos/%s.mp4", video.ID)

	client := youtube.Client{}
	resp, err := client.GetStream(video, &video.Formats[0])
	if err != nil {
		rClient.Del()
		return "", err
	}
	defer resp.Body.Close()

	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		panic(err)
	}

	rClient.SetNX(video.ID, path, time.Duration(1)*time.Hour)
	return path, nil
}

func cleanOld() {
	for {
		err := filepath.Walk(os.Getenv("VIDEOS_PATH"), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			_, id := filepath.Split(path)
			id = strings.TrimSuffix(id, filepath.Ext(path))
			if rClient.Get(id).Val() == "" {
				os.Remove(path)
			}
			return nil
		})
		if err != nil {
			panic(err)
		}

		time.Sleep(time.Duration(60) * time.Second)
	}
}
