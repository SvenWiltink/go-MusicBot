package main

import (
	"fmt"
	"gitlab.transip.us/swiltink/go-MusicBot/api"
	"gitlab.transip.us/swiltink/go-MusicBot/bot"
	"gitlab.transip.us/swiltink/go-MusicBot/config"
	"gitlab.transip.us/swiltink/go-MusicBot/player"
	"gitlab.transip.us/swiltink/go-MusicBot/playlist"
	"log"
)

func main() {
	conf, err := config.ReadConfig("conf.json")
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	play := playlist.NewPlaylist()

	ytPlayer, err := player.NewYoutubePlayer()
	if err != nil {
		fmt.Printf("Error creating Youtube player: %v\n", err)
	} else {
		play.AddMusicPlayer(ytPlayer)
		fmt.Println("Added Youtube player")
	}

	spPlayer, err := player.NewSpotifyPlayer()
	if err != nil {
		fmt.Printf("Error creating Spotify player: %v\n", err)
	} else {
		play.AddMusicPlayer(spPlayer)
		fmt.Println("Added Spotify player")
	}

	// Initialize the API
	apiObject := api.NewAPI(&conf.API, play)
	go apiObject.Start()

	// Initialize the IRC bot
	botObject, err := bot.NewMusicBot(&conf.IRC, play)
	if err != nil {
		fmt.Printf("Error creating IRC bot: %v\n", err)
		return
	}
	err = botObject.Start()
	if err != nil {
		fmt.Printf("Error starting IRC bot: %v\n", err)
		return
	}
}
