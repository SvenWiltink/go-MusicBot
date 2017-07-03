package bot

import (
	"bufio"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/svenwiltink/go-musicbot/config"
	"github.com/svenwiltink/go-musicbot/player"
	"github.com/svenwiltink/go-musicbot/util"
	"github.com/svenwiltink/volumecontrol"
	"github.com/thoj/go-ircevent"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Command struct {
	Name     string
	Function func(bot *MusicBot, event *irc.Event, parameters []string)
}

func (c *Command) execute(bot *MusicBot, event *irc.Event, parameters []string) {
	c.Function(bot, event, parameters)
}

var HelpCommand = Command{
	Name: "help",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		var names []string
		for commandName := range bot.commands {
			names = append(names, boldText(commandName))
		}
		sort.Strings(names)
		event.Connection.Privmsgf(target, "Available commands: %s", strings.Join(names, ", "))
	},
}

var WhitelistCommand = Command{
	Name: "whitelist",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		realname := event.User
		if len(parameters) < 1 {
			event.Connection.Privmsg(target, "Usage: !music whitelist <show|add|remove> [user]")
			return
		}

		subcommand := parameters[0]
		switch subcommand {
		case "show":
			message := "Current whitelist: "
			for _, name := range bot.whitelist {
				message += " " + underlineText(name)
			}
			event.Connection.Privmsg(target, message)
		case "add":
			if len(parameters) < 2 {
				event.Connection.Privmsg(target, boldText("Usage: !music whitelist add [user]"))
				return
			}
			user := parameters[1]
			if realname == bot.config.IRC.Master {
				if isWhitelisted, _ := bot.isUserWhitelisted(user); !isWhitelisted {
					bot.whitelist = append(bot.whitelist, user)

					err := config.WriteWhitelist(bot.config.IRC.WhiteListPath, bot.whitelist)
					if err != nil {
						event.Connection.Privmsg(target, err.Error())
						return
					}
					logrus.Infof("Whitelist: User %s added to whitelist by %s", user, event.Nick)
					event.Connection.Privmsgf(target, boldText("User %s added to whitelist by %s"), user, event.Nick)
				}
			}
		case "remove":
			if len(parameters) < 2 {
				event.Connection.Privmsg(target, boldText("Usage: !music whitelist remove [user]"))
				return
			}
			user := parameters[1]
			if realname == bot.config.IRC.Master {
				if isWhitelisted, index := bot.isUserWhitelisted(user); isWhitelisted {
					bot.whitelist = append(bot.whitelist[:index], bot.whitelist[index+1:]...)

					err := config.WriteWhitelist(bot.config.IRC.WhiteListPath, bot.whitelist)
					if err != nil {
						event.Connection.Privmsg(target, err.Error())
						return
					}
					logrus.Infof("Whitelist: User %s removed from whitelist by %s", user, event.Nick)
					event.Connection.Privmsgf(target, boldText("User %s removed from whitelist by %s"), user, event.Nick)
				}
			}
		}
	},
}

var NextCommand = Command{
	Name: "next",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		_, err := bot.player.Next()
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}
		bot.announceMessage(true, event, boldText(event.Nick)+" skipped to the next song")
	},
}

var PreviousCommand = Command{
	Name: "previous",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		_, err := bot.player.Previous()
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}
		bot.announceMessage(true, event, boldText(event.Nick)+" skipped to the previous song")
	},
}

var JumpCommand = Command{
	Name: "jump",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		if len(parameters) < 1 {
			event.Connection.Privmsg(target, boldText("Usage: !music jump <deltaIndex> (positive index for forwards, negative index for backwards)"))
			return
		}

		index, err := strconv.ParseInt(parameters[0], 10, 32)
		if err != nil {
			event.Connection.Privmsg(target, inverseText("Error parsing jump index"))
			return
		}
		_, err = bot.player.Jump(int(index))
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}
		bot.announceMessagef(true, event, "%s jumped to song at index %d", boldText(event.Nick), index)
	},
}

var PlayCommand = Command{
	Name: "play",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		_, err := bot.player.Play()
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}
		bot.announceMessage(true, event, boldText(event.Nick)+" started the player")
	},
}

var SeekCommand = Command{
	Name: "seek",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		if len(parameters) < 1 {
			event.Connection.Privmsg(target, boldText("Usage: !music seek <secondsInSong> Or: !music seek <percentage>%"))
			return
		}
		seekStr := parameters[0]
		var seekSeconds int64
		if strings.HasSuffix(seekStr, "%") {
			percentage, err := strconv.ParseInt(seekStr[:len(seekStr)-1], 10, 32)
			if err != nil {
				event.Connection.Privmsg(target, inverseText("Error parsing seek percentage"))
				return
			}
			song, _ := bot.player.GetCurrent()
			if song == nil {
				event.Connection.Privmsg(target, inverseText("Nothing playing!"))
				return
			}
			duration := song.GetDuration().Nanoseconds() / 100 * percentage
			seekSeconds = duration / int64(time.Second)
		} else {
			var err error
			seekSeconds, err = strconv.ParseInt(seekStr, 10, 32)
			if err != nil {
				event.Connection.Privmsg(target, inverseText("Error parsing seek seconds"))
				return
			}
		}
		err := bot.player.Seek(int(seekSeconds))
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}
		song, remaining := bot.player.GetCurrent()
		if song != nil {
			bot.announceMessagef(false, event, "Progress: %s", boldText(progressString(song.GetDuration(), remaining)))
		}
	},
}

var PauseCommand = Command{
	Name: "pause",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		err := bot.player.Pause()
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}
		song, remaining := bot.player.GetCurrent()
		state := bot.player.GetStatus()
		switch state {
		case player.PAUSED:
			bot.announceMessage(false, event, boldText(event.Nick)+" paused the player")
		case player.PLAYING:
			bot.announceMessage(false, event, boldText(event.Nick)+" unpaused the player")
		}
		if song != nil {
			bot.announceMessagef(false, event, "Progress: %s", boldText(progressString(song.GetDuration(), remaining)))
		}
	},
}

var StopCommand = Command{
	Name: "stop",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		err := bot.player.Stop()
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
		}
		bot.announceMessage(true, event, boldText(event.Nick)+" stopped the player")
	},
}

var CurrentCommand = Command{
	Name: "current",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		song, remaining := bot.player.GetCurrent()
		if song != nil {
			event.Connection.Privmsgf(target, "Current song: %s%s%s "+italicText("(%s remaining)"), BOLD_CHARACTER, formatSong(song), BOLD_CHARACTER, util.FormatDuration(remaining))
			event.Connection.Privmsgf(target, "Progress: %s", boldText(progressString(song.GetDuration(), remaining)))
		} else {
			event.Connection.Privmsg(target, italicText("Nothing currently playing"))
		}
	},
}

var AddCommand = Command{
	Name: "add",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		if len(parameters) < 1 {
			event.Connection.Privmsg(target, boldText("!music add <music link>"))
			return
		}
		url := parameters[0]

		_, err := bot.player.Add(url, event.User)
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}

		if bot.player.GetStatus() != player.PLAYING {
			bot.player.Play()
		}
	},
}

var OpenCommand = Command{
	Name: "open",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		if len(parameters) < 1 {
			event.Connection.Privmsg(target, boldText("Usage: !music open <music link>"))
			return
		}
		url := parameters[0]

		_, err := bot.player.Insert(url, 0, event.User)
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}

		if bot.player.GetStatus() != player.PLAYING {
			bot.player.Play()
		}
	},
}

var ShuffleCommand = Command{
	Name: "shuffle",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		bot.player.ShuffleQueue()
		bot.announceMessage(true, event, boldText(event.Nick)+" shuffled the playlist")
	},
}

var QueueCommand = Command{
	Name: "queue",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		items := bot.player.GetQueue()
		if len(items) == 0 {
			event.Connection.Privmsg(target, italicText("The queue is empty"))
		}

		dur := time.Duration(0)
		for i, song := range items {
			if i < 10 {
				event.Connection.Privmsgf(target, "%d. %s", i+1, formatSong(song))
			}
			dur += song.GetDuration()
		}

		if len(items) > 10 {
			event.Connection.Privmsgf(target, italicText("And %d more.."), len(items)-10)
		}

		event.Connection.Privmsgf(target, "Total duration: %s%s", BOLD_CHARACTER, util.FormatDuration(dur))
	},
}

var HistoryCommand = Command{
	Name: "history",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		items := bot.player.GetHistory()
		if len(items) == 0 {
			event.Connection.Privmsg(target, italicText("The history is empty"))
		}

		dur := time.Duration(0)
		for i, song := range items {
			if i < 10 {
				event.Connection.Privmsgf(target, "%d. %s", i+1, formatSong(song))
			}
			dur += song.GetDuration()
		}

		if len(items) > 10 {
			event.Connection.Privmsgf(target, italicText("And %d more.."), len(items)-10)
		}

		event.Connection.Privmsgf(target, "Total duration: %s%s", BOLD_CHARACTER, util.FormatDuration(dur))
	},
}

var FlushCommand = Command{
	Name: "flush",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		bot.player.EmptyQueue()
		bot.announceMessage(true, event, boldText(event.Nick)+" emptied the playlist")
	},
}

var SearchCommand = Command{
	Name: "search",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		if len(parameters) < 1 {
			event.Connection.Privmsgf(target, boldText("Usage: !music search [<track|album|playlist>] [<%s>] <search term>"), strings.ToLower(strings.Join(getPlayerNames(bot.player), "|")))
			return
		}

		results, err := searchSongs(bot.player, parameters, 5)
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}
		if len(results) == 0 {
			event.Connection.Privmsg(target, italicText("Nothing found!"))
			return
		}
		baseNameWidth := 80
		for plyr, res := range results {
			for i, item := range res {
				resultName := formatSong(item)
				paddingLength := baseNameWidth - utf8.RuneCountInString(resultName)
				if paddingLength > 0 {
					resultName += strings.Repeat(" ", paddingLength)
				}

				event.Connection.Privmsgf(target, "[%s #%d] %s | %s", plyr, i+1, boldText(resultName), item.GetURL())
			}
		}
	},
}

var SearchAddCommand = Command{
	Name: "search-add",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		if len(parameters) < 1 {
			event.Connection.Privmsgf(target, boldText("Usage: !music search-add [<track|album|playlist>] [<%s>] <search term>"), strings.ToLower(strings.Join(getPlayerNames(bot.player), "|")))
			return
		}

		results, err := searchSongs(bot.player, parameters, 1)
		if err != nil {
			event.Connection.Privmsg(target, inverseText(err.Error()))
			return
		}
		if len(results) == 0 {
			event.Connection.Privmsg(target, italicText("Nothing found!"))
			return
		}
		for _, res := range results {
			for _, item := range res {
				_, err := bot.player.Add(item.GetURL(), event.User)
				if err != nil {
					event.Connection.Privmsg(target, inverseText(err.Error()))
					return
				}
				if bot.player.GetStatus() != player.PLAYING {
					bot.player.Play()
				}
				return
			}
		}
	},
}

var StatsCommand = Command{
	Name: "stats",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		bot.ircConn.Privmsgf(target, "%s Statistics!", GetMusicBotStringFormatted())

		stats := bot.player.GetStatistics()

		var timeByPlayer []string
		for player, time := range stats.TimeByPlayer {
			timeByPlayer = append(timeByPlayer, fmt.Sprintf("%s: %s%s%s", player, BOLD_CHARACTER, util.FormatDuration(time), BOLD_CHARACTER))
		}
		bot.ircConn.Privmsgf(target, "Total play time: %s%v%s (%s)", BOLD_CHARACTER, util.FormatDuration(stats.TotalTimePlayed), BOLD_CHARACTER, strings.Join(timeByPlayer, " | "))

		var playedByPlayer []string
		for player, count := range stats.SongsPlayedByPlayer {
			playedByPlayer = append(playedByPlayer, fmt.Sprintf("%s: %s%d%s", player, BOLD_CHARACTER, count, BOLD_CHARACTER))
		}
		bot.ircConn.Privmsgf(target, "Total songs played: %s%d%s (%s)", BOLD_CHARACTER, stats.TotalSongsPlayed, BOLD_CHARACTER, strings.Join(playedByPlayer, " | "))
		bot.ircConn.Privmsgf(target, "Total songs queued: %s%d", BOLD_CHARACTER, stats.TotalSongsQueued)
		bot.ircConn.Privmsgf(target, "Total times skipped: %s%d", BOLD_CHARACTER, stats.TotalTimesNext+stats.TotalTimesPrevious+stats.TotalTimesJump)
		bot.ircConn.Privmsgf(target, "Total times paused: %s%d", BOLD_CHARACTER, stats.TotalTimesPaused)

		var songQueuers []string
		pairs := util.GetSortedStringIntMap(stats.SongsAddedByUser)
		pos := 1
		for i := len(pairs) - 1; i >= 0; i-- {
			songQueuers = append(songQueuers, fmt.Sprintf("%d. %s: %s%d%s", pos, pairs[i].Key, BOLD_CHARACTER, pairs[i].Val, BOLD_CHARACTER))
			pos++
			if pos > 10 {
				break
			}
		}
		bot.ircConn.Privmsgf(target, "Top 10 song queuers: %s", strings.Join(songQueuers, " | "))
	},
}

var VolUpCommand = Command{
	Name: "vol++",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		err := volumecontrol.IncreaseVolume(10)
		if err != nil {
			target, _, _ := bot.getTarget(event)
			event.Connection.Privmsgf(target, "%s%v", INVERSE_CHARACTER, err)
		}
	},
}

var VolDownCommand = Command{
	Name: "vol--",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		err := volumecontrol.DecreaseVolume(10)
		if err != nil {
			target, _, _ := bot.getTarget(event)
			event.Connection.Privmsgf(target, "%s%v", INVERSE_CHARACTER, err)
		}
	},
}

var VolCommand = Command{
	Name: "vol",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)
		if len(parameters) < 1 {
			event.Connection.Privmsg(target, "!music vol <volume>")
			return
		}

		vol, err := strconv.Atoi(parameters[0])
		if err != nil {
			event.Connection.Privmsgf(target, "%s%v", INVERSE_CHARACTER, err)
			return
		}

		err = volumecontrol.SetVolume(vol)
		if err != nil {
			event.Connection.Privmsgf(target, "%s%v", INVERSE_CHARACTER, err)
			return
		}
	},
}

var VersionCommand = Command{
	Name: "version",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)

		bot.ircConn.Privmsgf(target, "%s %s (%s)", GetMusicBotStringFormatted(), util.VersionTag, util.GitCommit)
		bot.ircConn.Privmsgf(target, "Built: %s", util.BuildDate)
		bot.ircConn.Privmsgf(target, "Built on: %s", util.BuildHost)
		bot.ircConn.Privmsgf(target, "GO version: %s", util.GoVersion)
	},
}

var LogCommand = Command{
	Name: "log",
	Function: func(bot *MusicBot, event *irc.Event, parameters []string) {
		target, _, _ := bot.getTarget(event)

		if bot.config.LogFile == "" {
			event.Connection.Privmsgf(target, "%sCannot show log, no logfile configured", ITALIC_CHARACTER)
			return
		}

		file, err := os.Open(bot.config.LogFile)
		if err != nil {
			logrus.Errorf("bot.LogCommand: Error opening file: [%s] %v", bot.config.LogFile, err)
			return
		}
		defer file.Close()

		var lines []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		err = scanner.Err()
		if err != nil {
			logrus.Errorf("bot.LogCommand: Error scanning file: [%s] %v", bot.config.LogFile, err)
			return
		}

		for i := len(lines) - 11; i < len(lines); i++ {
			if i < 0 {
				continue
			}
			event.Connection.Privmsgf(target, "#%03d: %s", i+1, lines[i])
		}
	},
}
