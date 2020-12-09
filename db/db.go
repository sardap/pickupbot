package db

import (
	"fmt"
	"io"
	"os"

	"github.com/kkdai/youtube/v2"
	"github.com/sardap/pickupbot/env"
	bolt "go.etcd.io/bbolt"
)

var (
	dbClient   *bolt.DB
	bucketName = []byte("SKA_SONGS")
)

//Connect init connection to DB
func Connect() {
	var err error
	dbClient, err = bolt.Open(env.DBPath, 0666, nil)
	if err != nil {
		panic(err)
	}

	err = dbClient.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		panic(err)
	}
}

//GetVideoPath GetVideoPath
func GetVideoPath(id string) string {
	var result string
	dbClient.View(func(tx *bolt.Tx) error {
		val := tx.Bucket(bucketName).Get([]byte(id))
		if val != nil {
			result = string(val)
		}
		return nil
	})
	return result
}

//NewVideo adds a new video to the DB
func NewVideo(video *youtube.Video) (string, error) {
	path := fmt.Sprintf("videos/%s.mp4", video.ID)

	client := youtube.Client{}
	resp, err := client.GetStream(video, &video.Formats[0])
	if err != nil {
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

	dbClient.Update(func(tx *bolt.Tx) error {
		tx.Bucket(bucketName).Put(
			[]byte(video.ID), []byte(path),
		)
		return nil
	})
	return path, nil
}

//SaveYTSearch saves a query with result in the DB
func SaveYTSearch(query, bestID string) {
	dbClient.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put(
			[]byte(query), []byte(bestID),
		)
	})
}

//GetYTSearch gets a query with result in the DB
func GetYTSearch(query string) string {
	var result string
	dbClient.View(func(tx *bolt.Tx) error {
		val := tx.Bucket(bucketName).Get(
			[]byte(query),
		)
		if val != nil {
			result = string(val)
		}
		return nil
	})
	return result
}
