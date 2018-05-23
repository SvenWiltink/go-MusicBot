package music

import "github.com/vansante/go-event-emitter"

const (
	EventSongStarted = "song-started"
)

// Player is the wrapper around MusicProviders. This should keep track of the queue and control
// the MusicProviders
type Player interface {
	eventemitter.Observable
	Start()
	Search(string) ([]*Song, error)
	SetVolume(percentage int)
	AddSong(song *Song) error
	Next() error
	Stop()
	GetStatus() PlayerStatus
	GetCurrentSong() *Song
}

type PlayerStatus string

const (
	PlayerStatusStarting PlayerStatus = "starting"
	PlayerStatusWaiting  PlayerStatus = "waiting"
	PlayerStatusLoading  PlayerStatus = "loading"
	PlayerStatusPlaying  PlayerStatus = "playing"
	PlayerStatusPaused   PlayerStatus = "paused"
)

func (s PlayerStatus) CanBeSkipped() bool {
	if s == PlayerStatusPlaying || s == PlayerStatusPaused {
		return true
	}

	return false
}