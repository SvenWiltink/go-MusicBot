package main

import (
	"fmt"
	"github.com/SvenWiltink/go-MusicBot/api"
	"github.com/SvenWiltink/go-MusicBot/bot"
	"github.com/SvenWiltink/go-MusicBot/config"
	"github.com/SvenWiltink/go-MusicBot/player"
	"github.com/SvenWiltink/go-MusicBot/songplayer"
	"github.com/SvenWiltink/go-MusicBot/util"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	// Set logrus to be the standard logger
	logger := logrus.New()
	logrus.SetOutput(logger.Writer())

	conf, err := config.ReadConfig("conf.json")
	if err != nil {
		logrus.Fatalf("main: Error reading musicbot config: %v", err)
		return
	}

	queueStorage := config.NewQueueStorage(conf.QueuePath)
	playr := player.NewPlayer()

	// Initialize the API
	apiObject := api.NewAPI(&conf.API, playr)
	logrus.Infof("main: Starting HTTP API")
	go apiObject.Start()

	// Initialize the IRC bot
	musicBot, err := bot.NewMusicBot(conf, playr)
	if err != nil {
		logrus.Fatalf("main: Error creating IRC MusicBot: %v", err)
		return
	}
	logrus.Infof("main: Starting IRC MusicBot")
	err = musicBot.Start()
	if err != nil {
		logrus.Fatalf("main: Error starting IRC MusicBot: %v", err)
		return
	}

	if conf.YoutubePlayer.Enabled {
		logrus.Infof("main: Creating YoutubePlayer")

		ytPlayer, err := songplayer.NewYoutubePlayer(conf.YoutubePlayer.YoutubeAPIKey, conf.YoutubePlayer.MpvBinPath, conf.YoutubePlayer.MpvInputPath)
		if err != nil {
			logrus.Errorf("main: Error creating YoutubePlayer: %v", err)
			musicBot.Announce(fmt.Sprintf("[YoutubePlayer] Error creating player: %v", err))
		} else {
			playr.AddSongPlayer(ytPlayer)
		}
	}

	if conf.SpotifyPlayer.Enabled && conf.SpotifyPlayer.UseConnect {
		logrus.Infof("main: Creating SpotifyConnectPlayer")

		spPlayer, authURL, err := songplayer.NewSpotifyConnectPlayer(conf.SpotifyPlayer.ClientID, conf.SpotifyPlayer.ClientSecret, conf.SpotifyPlayer.TokenFilePath, "", 0)
		if err != nil {
			logrus.Errorf("main: Error creating SpotifyConnectPlayer: %v", err)
			musicBot.Announce(fmt.Sprintf("[SpotifyConnectPlayer] Error creating player: %v", err))
		} else if authURL != "" {
			ips, err := util.GetExternalIPs()
			ipStr := "???"
			if err != nil {
				logrus.Warnf("main: Error getting external IPs: %v", err)
			} else {
				ipStr = ""
				for _, ip := range ips {
					ipStr += ip.String() + " "
				}
				ipStr = strings.TrimSpace(ipStr)
			}
			musicBot.Announce(fmt.Sprintf("[SpotifyConnectPlayer] Authorisation: Add the external IP (%s) of the bot to your hosts file under 'musicbot' and visit:", ipStr))
			musicBot.Announce(authURL)
			spPlayer.AddAuthorisationListener(func() {
				playr.AddSongPlayer(spPlayer)
				musicBot.Announce("[SpotifyConnect] The musicbot was successfully authorised!")
			})
		} else {
			playr.AddSongPlayer(spPlayer)
		}
	}

	if conf.SpotifyPlayer.Enabled && !conf.SpotifyPlayer.UseConnect {
		logrus.Infof("main: Creating SpotifyPlayer")

		spPlayer, err := songplayer.NewSpotifyPlayer(conf.SpotifyPlayer.Host)
		if err != nil {
			logrus.Errorf("main: Error creating SpotifyPlayer: %v", err)
			musicBot.Announce(fmt.Sprintf("[SpotifyPlayer] Error creating player: %v", err))
		} else {
			playr.AddSongPlayer(spPlayer)
		}
	}

	urls, err := queueStorage.ReadQueue()
	if err != nil {
		logrus.Warnf("main: Error reading queue file: %v", err)
		musicBot.Announce(fmt.Sprintf("[Queue] Error loading queue: %v", err))
	} else {
		for _, url := range urls {
			_, err = playr.AddSongs(url)
			if err != nil {
				logrus.Errorf("main: Error adding song from queue [%s] %v", url, err)
			}

		}
		logrus.Infof("main: Loaded %d songs from queue file", len(playr.GetQueuedSongs()))
		musicBot.Announce(fmt.Sprintf("%sLoaded %d songs from queue file", bot.UNDERLINE_CHARACTER, len(playr.GetQueuedSongs())))
	}

	playr.AddListener("queue_updated", queueStorage.OnListUpdate)

	// Wait for a terminate signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs

	logrus.Infof("main: Shutting down")
	musicBot.Stop()
}
