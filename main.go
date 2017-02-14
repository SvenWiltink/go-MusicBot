package main

import (
	"encoding/json"
	"fmt"
	"gitlab.transip.us/swiltink/go-MusicBot/bot"
	"gitlab.transip.us/swiltink/go-MusicBot/player"
	"os"
)

func main() {
	file, err := os.Open("conf.json")
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(2)
	}

	decoder := json.NewDecoder(file)
	configuration := bot.Configuration{}
	err = decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(2)
	}
	playerObject := player.NewMusicPlayer()
	botObject := bot.NewMusicBot(configuration, playerObject)
	botObject.Start()
}
