package translator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/agnivade/levenshtein"
	"github.com/go-redis/redis"
	"github.com/rylio/ytdl"
	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
)

var (
	developerKey string = os.Getenv("YOUTUBE_API_KEY")
	rClient      *redis.Client
)

func init() {
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

//ToYTURL ToYTURL
func ToYTURL(title string, artists []string) (*ytdl.VideoInfo, error) {
	query := fmt.Sprintf("%s - %s", title, artists[0])
	queryB := fmt.Sprintf("%s - %s", artists[0], title)

	var videoID string
	cache := rClient.Get(query)
	if cache.Val() != "" {
		videoID = cache.Val()
	} else {
		client := &http.Client{
			Transport: &transport.APIKey{Key: developerKey},
		}

		service, err := youtube.New(client)
		if err != nil {
			return nil, err
		}

		call := service.Search.List([]string{"id", "snippet"}).Q(query).MaxResults(50)
		response, err := call.Do()
		if err != nil {
			return nil, errors.New("Could not find match on youtube")
		}

		// Group video, channel, and playlist results in separate lists.
		bestDist := 25555555.0
		bestID := ""

		// Iterate through each item and add it to the correct list.
		for _, item := range response.Items {
			switch item.Id.Kind {
			case "youtube#video":
				reg := regexp.MustCompile("[^a-zA-Z0-9]+")
				title := reg.ReplaceAllString(item.Snippet.Title, "")
				aDistance := float64(levenshtein.ComputeDistance(reg.ReplaceAllString(query, ""), title))
				bDistance := float64(levenshtein.ComputeDistance(reg.ReplaceAllString(queryB, ""), title))
				distance := math.Min(aDistance, bDistance)
				if distance < bestDist {
					bestID = item.Id.VideoId
					bestDist = distance
				}
			}
		}

		if bestID == "" {
			return nil, errors.New("Could not find match on youtube")
		}

		rClient.SetNX(query, bestID, time.Duration(120)*time.Hour)
		videoID = bestID
	}

	return ytdl.DefaultClient.GetVideoInfoFromID(context.TODO(), videoID)
}
