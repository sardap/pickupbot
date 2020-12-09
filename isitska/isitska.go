package isitska

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const skaConfidenceThreshold = 0.75

//Invoker used to invoke the api endpoints
type Invoker struct {
	Endpoint string
}

//TrackInfo contatins track info from api
type TrackInfo struct {
	Prob      float64  `json:"prob"`
	Title     string   `json:"title"`
	Album     string   `json:"album"`
	Artists   []string `json:"artists"`
	TrackLink string   `json:"track_link"`
}

type someSkaResponse struct {
	Tracks []TrackInfo `json:"tracks"`
}

//ArtistsStr returns artists as a string
func (t *TrackInfo) ArtistsStr() string {
	var artistStr strings.Builder
	artistStr.Grow(len(t.Artists) * 4)
	for i, artist := range t.Artists {
		fmt.Fprintf(&artistStr, "%s", artist)
		if i > 0 && i < len(t.Artists)-1 {
			fmt.Fprintf(&artistStr, ", ")
		}
	}
	return artistStr.String()
}

//IsSka returns if track is ska
func (t *TrackInfo) IsSka() bool {
	return t.Prob > skaConfidenceThreshold
}

func (i *Invoker) fetchSkaProb(name, artist, trackID string) (TrackInfo, error) {
	url := fmt.Sprintf("%s/api/ska_prob", i.Endpoint)
	req, err := http.NewRequest("GET", url, nil)
	q := req.URL.Query()
	if name != "" {
		q.Add("track_name", name)
	}
	if artist != "" {
		q.Add("artist_name", artist)
	}
	if trackID != "" {
		q.Add("track_id", trackID)
	}
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return TrackInfo{}, errors.Wrap(
			err,
			fmt.Sprintf("unable to fetch %s", url),
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)
		log.Printf("%v\n", bodyString)
		return TrackInfo{}, nil
	}

	var data TrackInfo
	err = json.NewDecoder(resp.Body).Decode(&data)

	return data, err
}

//ByName fetches if named song is ska
func (i *Invoker) ByName(name, artist string) (TrackInfo, error) {
	res, err := i.fetchSkaProb(name, artist, "")
	if err != nil {
		return TrackInfo{}, err
	}
	return res, nil
}

//ByID fetches if named song is ska
func (i *Invoker) ByID(trackID string) (TrackInfo, error) {
	res, err := i.fetchSkaProb("", "", trackID)
	if err != nil {
		return TrackInfo{}, err
	}
	return res, nil
}

//GetNSka returns n ska songs
func (i *Invoker) GetNSka(n int) ([]TrackInfo, error) {
	if n <= 0 {
		return nil, errors.New("Must set n to greater than 0")
	}

	url := fmt.Sprintf("%s/api/some_ska", i.Endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("n", strconv.Itoa(n))

	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(
			err,
			fmt.Sprintf("unable to fetch %s", url),
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)
		log.Printf("%v\n", bodyString)
		return nil, nil
	}

	var data someSkaResponse
	err = json.NewDecoder(resp.Body).Decode(&data)

	return data.Tracks, err
}
