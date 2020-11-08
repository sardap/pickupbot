package translator

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/agnivade/levenshtein"
	ytdl "github.com/kkdai/youtube/v2"
	"github.com/sardap/pickupbot/db"
	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
)

var (
	developerKey string = os.Getenv("YOUTUBE_API_KEY")
)

//ToYTURL ToYTURL
func ToYTURL(title string, artists []string) (*ytdl.Video, error) {
	query := fmt.Sprintf("%s - %s", title, artists[0])
	queryB := fmt.Sprintf("%s - %s", artists[0], title)

	var videoID string
	cache := db.GetClient().Get(query)
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
			return nil, err
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

		db.GetClient().SetNX(query, bestID, time.Duration(24*365)*time.Hour)
		videoID = bestID
	}

	client := ytdl.Client{}
	return client.GetVideo(videoID)
}
