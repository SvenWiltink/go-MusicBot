package api

import (
	"github.com/SvenWiltink/go-MusicBot/player"
	"github.com/SvenWiltink/go-MusicBot/songplayer"
	"github.com/SvenWiltink/go-MusicBot/util"
	"time"
)

type Song struct {
	Title            string
	Seconds          int
	SecondsRemaining int
	FormattedTime    string
	URL              string
	ImageURL         string
}

type Status struct {
	Status  player.Status
	Current *Song
	List    []Song
}

type Event struct {
	Event     string
	Arguments []interface{}
}

type Command struct {
	Command   string
	Arguments []string
}

type CommandResponse struct {
	Command string
	Success bool
	Error   string
	Status  *Status `json:",omitempty"`
}

func getAPISong(song songplayer.Playable, remaining time.Duration) (apiSong *Song) {
	if song != nil {
		duration := song.GetDuration()

		apiSong = &Song{
			Title:            song.GetTitle(),
			URL:              song.GetURL(),
			Seconds:          int(duration.Seconds()),
			SecondsRemaining: int(remaining.Seconds()),
			FormattedTime:    util.FormatSongLength(duration),
			ImageURL:         song.GetImageURL(),
		}
	}
	return
}

func getAPISongs(songs []songplayer.Playable) (apiSongs []Song) {
	for _, song := range songs {
		if song == nil {
			continue
		}
		apiSongs = append(apiSongs, *getAPISong(song, song.GetDuration()))
	}
	return
}

func getCommandResponse(cmd *Command, err error) (resp CommandResponse) {
	resp.Command = cmd.Command
	resp.Success = err == nil
	resp.Error = err.Error()
	return
}
