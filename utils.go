package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	discordgo "github.com/courtier/kolizey"
	"github.com/gookit/color"
	"github.com/spf13/viper"
)

var (
	allSessionsMutex = &sync.Mutex{}
)

func appendSession(token string, s *discordgo.Session) {
	allSessionsMutex.Lock()
	defer allSessionsMutex.Unlock()
	allSessions[token] = s
}

func logWithTime(msg string) {
	timeStr := time.Now().Format("Jan 2, 2006 at 15:04:05")
	color.Println("<magenta>" + timeStr + " | </>" + msg)
}

func fatalWithTime(msg string) {
	timeStr := time.Now().Format("Jan 2, 2006 at 15:04:05")
	color.Println("<magenta>" + timeStr + " | </>" + msg)

	os.Exit(1)
}

func deleteEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func loadViper() ConfigStruct {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		fatalWithTime("<red>Error loading config! " + err.Error() + "</>")
	}
	return ConfigStruct{
		MainToken:          viper.GetString("mainToken"),
		ClaimOnMain:        viper.GetBool("claimOnMain"),
		SnipeOnMain:        viper.GetBool("snipeOnMain"),
		AltTokens:          viper.GetStringSlice("altTokens"),
		UseAlts:            viper.GetBool("useAlts"),
		CustomWebhookNitro: viper.GetBool("customWebhookNitro"),
		CustomWebhookLink:  viper.GetString("customWebhookLink"),
		RemoveDuplicates:   viper.GetBool("removeDuplicates"),
		ShowDuplicates:     viper.GetBool("showDuplicates"),
		SmartSnipe:         viper.GetBool("smartSnipe"),
	}
}

func loadPastCodes() {
	path := "nitrocodes.txt"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_, err = os.Create(path)
		if err != nil {
			logWithTime("<red>Error creating nitro code file!</>")
		}
	} else {
		file, err := os.Open(path)
		if err != nil {
			return
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			code := scanner.Text()
			code = strings.ReplaceAll(code, " ", "")
			code = strings.ReplaceAll(code, "\n", "")
			cacheCodes[code] = true
		}

		if err := scanner.Err(); err != nil {
			logWithTime("<red>Error reading nitro codes file!</>")
		}
	}
}

func removeAltToken(token string) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	for in, el := range config.AltTokens {
		if el == token {
			//remove token from slice
			config.AltTokens[in] = config.AltTokens[0]
			config.AltTokens = config.AltTokens[1:]
		}
	}
}

func getGuildName(session *discordgo.Session, guildID string) string {
	guild, err := session.State.Guild(guildID)
	if err != nil || guild == nil {
		guild, err = session.Guild(guildID)
		if err != nil {
			return "DM"
		}
	}
	return guild.Name
}

func userIntoUsername(user *discordgo.User) string {
	if user == nil || user.Username == "" {
		return ""
	}
	return user.Username + "#" + user.Discriminator
}

func setTitle(title string) {
	fmt.Print("\033]0;" + title + "\007")
}

func diffToString(diff time.Duration) string {
	seconds := float64(diff) / float64(time.Second)
	return fmt.Sprintf("%f", seconds)
}

func clearConsole() {
	var c = &exec.Cmd{}
	switch runtime.GOOS {
	case "linux":
		c = exec.Command("clear")
	case "darwin":
		c = exec.Command("clear")
	default:
		c = exec.Command("cmd", "/c", "cls")
	}
	c.Stdout = os.Stdout
	c.Run()
}
