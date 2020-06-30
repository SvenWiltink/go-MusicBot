package youtube

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	isoduration "github.com/channelmeter/iso8601duration"
	"github.com/svenwiltink/go-musicbot/pkg/music"
	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
)

const (
	youTubeVideoURL    = "https://www.youtube.com/watch?v=%s"
	youTubePlaylistURL = "https://www.youtube.com/watch?v=%s&list=%s"

	MaxYoutubeItems = 500
)

var youtubeURLRegex = regexp.MustCompile(`^((?:https?:)?\/\/)?((?:www|m)\.)?((?:youtube\.com|youtu.be))(\/(?:[\w\-]+\?v=|embed\/|v\/)?)([\w\-]+)(\S+)?$`)

type DataProvider struct {
	apiKey  string
	service *youtube.Service
}

func NewDataProvider(apiKey string) (*DataProvider, error) {
	instance := &DataProvider{
		apiKey: apiKey,
	}

	err := instance.initAPIClient()
	if err != nil {
		err = fmt.Errorf("could not start youtube api client: %v", err)
		return nil, err
	}

	return instance, nil
}

func (provider *DataProvider) initAPIClient() error {
	client := &http.Client{
		Transport: &transport.APIKey{Key: provider.apiKey},
	}

	service, err := youtube.New(client)
	if err != nil {
		return fmt.Errorf("YoutubeAPI.init: Error creating client: %v", err)
	}

	provider.service = service

	return nil
}

func (provider *DataProvider) CanProvideData(song music.Song) bool {
	return youtubeURLRegex.MatchString(song.Path)
}

func (provider *DataProvider) ProvideData(song *music.Song) error {
	identifier, err := provider.getIdentifierForSong(song)
	if err != nil {
		return err
	}

	return provider.provideDataForIdentifier(identifier, song)
}

func (provider *DataProvider) provideDataForIdentifier(identifier string, song *music.Song) error {
	call := provider.service.Videos.List("snippet,contentDetails").Id(identifier)
	response, err := call.Do()
	if err != nil {
		return fmt.Errorf("could not get data for url: %v", err)
	}

	for _, item := range response.Items {
		if item.Id == identifier && item.Kind == "youtube#video" {
			if item.Snippet == nil {
				return errors.New("snippet not found")
			}

			song.Name = item.Snippet.Title
			song.Artist = item.Snippet.ChannelTitle

			duration, err := isoduration.FromString(item.ContentDetails.Duration)

			if err != nil {
				return err
			}

			song.Duration = duration.ToDuration()

			song.Path = fmt.Sprintf(youTubeVideoURL, identifier)
			return nil
		}
	}

	return fmt.Errorf("playable not found for: %s", identifier)
}

func (provider *DataProvider) getIdentifierForSong(song *music.Song) (string, error) {
	ytURL, err := url.Parse(song.Path)
	if err != nil {
		return "", fmt.Errorf("YoutubeAPI.GetPlayableForURL: Unable to parse URL [%s] %v", song.Path, err)
	}

	identifier := ytURL.Query().Get("v")
	if identifier == "" {
		// Assume format like: https://youtu.be/n1dpZy5Jx4o, in which the path is the identifier
		identifier = ytURL.Path

		if strings.ToLower(identifier) == "watch" || identifier == "" {
			return "", fmt.Errorf("empty identifier for: %s", song.Path)
		}
	}

	return identifier, nil
}

func (provider *DataProvider) Search(searchString string) ([]music.Song, error) {
	searchTypeStr := "video"

	call := provider.service.Search.List("id,snippet").
		Q(searchString).
		Type(searchTypeStr).
		MaxResults(int64(5))

	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("YoutubeApi: error searching %s: %v", searchString, err)
	}

	songs := make([]music.Song, 0)

	for _, item := range response.Items {
		switch item.Id.Kind {
		case "youtube#video":
			song := &music.Song{}
			err = provider.provideDataForIdentifier(item.Id.VideoId, song)
			if err != nil {
				return nil, fmt.Errorf("error finding data: %v", err)
			}

			songs = append(songs, *song)
		}
	}
	return songs, nil
}
