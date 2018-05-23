package player

import (
	"fmt"
	"log"

	"github.com/svenwiltink/go-musicbot/music"
	"github.com/vansante/go-event-emitter"
)


// MusicPlayer is responsible for playing music
type MusicPlayer struct {
	*eventemitter.Emitter
	Queue          *Queue
	Status         music.PlayerStatus
	dataProviders  []music.DataProvider
	musicProviders []music.Provider
	activeProvider music.Provider
	currentSong    *music.Song
	shouldStop     bool
}

func (player *MusicPlayer) GetStatus() music.PlayerStatus {
	return player.Status
}

func (player *MusicPlayer) GetCurrentSong() *music.Song {
	return player.currentSong
}

func (player *MusicPlayer) SetVolume(percentage int) {
	for _, provider := range player.musicProviders {
		provider.SetVolume(percentage)
	}
}

func (player *MusicPlayer) addMusicProvider(provider music.Provider) {
	player.musicProviders = append(player.musicProviders, provider)
}

func (player *MusicPlayer) Search(searchString string) ([]*music.Song, error) {
	songs := make([]*music.Song, 0)

	for _, provider := range player.dataProviders {
		results, _ := provider.Search(searchString)

		if results != nil {
			songs = append(songs, results...)
		}
	}

	return songs, nil
}

// AddSong tries to add the song to the Queue
func (player *MusicPlayer) AddSong(song *music.Song) error {
	// assume it is a song unless the dataprovider changes it to a stream
	song.SongType = music.SongTypeSong

	dataProvider := player.getSuitableDataProvider(song)

	if dataProvider == nil {
		return fmt.Errorf("no dataprovider found for %+v", song)
	}

	err := dataProvider.ProvideData(song)

	log.Printf("provided song data: %+v", song)

	if err != nil {
		return fmt.Errorf("could not get data for song: %v", err)
	}

	suitablePlayer := player.getSuitablePlayer(song)

	if suitablePlayer == nil {
		return fmt.Errorf("no suitable player found for %+v", song)
	}

	player.Queue.append(song)
	return nil
}

func (player *MusicPlayer) getSuitableDataProvider(song *music.Song) music.DataProvider {
	for _, provider := range player.dataProviders {
		if provider.CanProvideData(song) {
			return provider
		}
	}

	return nil
}

func (player *MusicPlayer) getSuitablePlayer(song *music.Song) music.Provider {
	for _, provider := range player.musicProviders {
		if provider.CanPlay(song) {
			return provider
		}
	}

	return nil
}

// Start the MusicPlayer
func (player *MusicPlayer) Start() {
	log.Println("Starting music player")
	go player.playLoop()
}

func (player *MusicPlayer) playLoop() {
	for !player.shouldStop {
		player.Status = music.PlayerStatusWaiting
		log.Println("Waiting for song")
		song := player.Queue.WaitForNext()
		player.currentSong = song

		provider := player.getSuitablePlayer(song)
		player.activeProvider = provider

		player.Status = music.PlayerStatusLoading
		err := provider.PlaySong(song)

		if err != nil {
			log.Println(err)
			continue
		}

		player.EmitEvent(music.EventSongStarted, song)
		player.Status = music.PlayerStatusPlaying
		provider.Wait()

		log.Println("Song ended")
	}
}

func (player *MusicPlayer) Next() error {
	fmt.Printf("current player status: %v", player.Status)

	if player.Status.CanBeSkipped() {
		return player.activeProvider.Skip()
	}

	return fmt.Errorf("nothing is playing")
}

func (player *MusicPlayer) Stop() {
	player.shouldStop = true
	for _, provider := range player.musicProviders {
		provider.Stop()
	}
}

// NewMusicPlayer creates a new MusicPlayer instance
func NewMusicPlayer(providers []music.Provider, dataProviders []music.DataProvider) *MusicPlayer {
	instance := &MusicPlayer{
		Emitter:        eventemitter.NewEmitter(false),
		Queue:          NewQueue(),
		musicProviders: providers,
		dataProviders:  dataProviders,
		shouldStop:     false,
	}

	return instance
}
