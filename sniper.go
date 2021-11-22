package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	strconv "strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	discordgo "github.com/courtier/kolizey"
	"github.com/gookit/color"
	"github.com/valyala/fasthttp"
)

//ConfigStruct holds config values
type ConfigStruct struct {
	MainToken          string
	ClaimOnMain        bool
	SnipeOnMain        bool
	AltTokens          []string
	UseAlts            bool
	CustomWebhookNitro bool
	CustomWebhookLink  string
	RemoveDuplicates   bool
	ShowDuplicates     bool
	SmartSnipe         bool
}

type responseStruct struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

var (
	//holds config options
	config ConfigStruct
	//payment source id of each token
	paymentSourceIDs = make(map[string]string)
	//cached codes
	cacheCodes = make(map[string]bool)
	//regex to extract payment source id
	rePaymentSourceID = regexp.MustCompile(`("id": ")([0-9]+)"`)
	//regex to extract nitro type
	reNitroType = regexp.MustCompile(` "name": "([ a-zA-Z]+)", "features"`)
	//regex to extract nitro gift code
	reGiftLink = regexp.MustCompile("(ifts/|gift/)([a-zA-Z0-9]+)")
	//mutex to use when removing broken tokens
	tokenMutex = &sync.Mutex{}
	//mutex for the nitro codes file
	nitroCodeFileMutex = &sync.Mutex{}
	//map of tokens and pointers to their respective sessions
	allSessions = make(map[string]*discordgo.Session)
	//main tokens username
	mainUsername string
	//main tokens avatar url
	mainAvatar string
	//alts amount
	altsAmount int
	//time started
	timeStarted = time.Now()
	//nitros sniped total
	nitrosSniped int
)

func main() {
	startSniper()
}

func printTitle() {
	clearConsole()

	//http://www.network-science.de/ascii/
	//https://www.askapache.com/online-tools/figlet-ascii/
	color.Println(`<blue>
	 █████╗ ██████╗ ██╗███████╗ █████╗ ███╗  ██╗ █████╗ 
	██╔══██╗██╔══██╗██║╚════██║██╔══██╗████╗ ██║██╔══██╗
	███████║██████╔╝██║  ███╔═╝██║  ██║██╔██╗██║███████║
	██╔══██║██╔══██╗██║██╔══╝  ██║  ██║██║╚████║██╔══██║
	██║  ██║██║  ██║██║███████╗╚█████╔╝██║ ╚███║██║  ██║
	╚═╝  ╚═╝╚═╝  ╚═╝╚═╝╚══════╝ ╚════╝ ╚═╝  ╚══╝╚═╝  ╚═╝</>
	<yellow>		by courtier</>
	`)
	setTitle("Arizona Sniper")

	fmt.Println()
	fmt.Println()
}

func startSniper() {
	printTitle()
	logWithTime("<cyan>Loading config & tokens...</>")

	config = loadViper()
	loadPastCodes()

	if config.MainToken == "" {
		fatalWithTime("<red>Main token is missing!</>")
	}

	if config.ClaimOnMain || config.SnipeOnMain {
		getPaymentSourceID(config.MainToken)
		logWithTime("<cyan>Claiming Nitro on main token!</>")
	}

	var authedAlts chan string
	if config.UseAlts {
		authedAlts = make(chan string, len(config.AltTokens))
		config.AltTokens = deleteEmpty(config.AltTokens)

		logWithTime("<cyan>Loaded </><yellow>" + strconv.Itoa(len(config.AltTokens)) + "</><cyan> alts!</>")

		if len(config.AltTokens) != 0 {
			for _, token := range config.AltTokens {
				go connectAltToken(token, authedAlts)
			}
		}
	}

	var dg *discordgo.Session
	var err error

	if config.SnipeOnMain {
		logWithTime("<cyan>Sniping Nitro on main token!</>")
		dg, err = discordgo.New(config.MainToken)
		dg.LogLevel = -1
		if err != nil {
			fatalWithTime("<red>Error creating session for main token!</>")
		}
		dg.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.159 Safari/537.36"
		err = dg.Open()
		if err != nil {
			fatalWithTime("<red>Error opening connection for main token!</>")
		}
		dg.AddHandler(messageCreate)
		appendSession(config.MainToken, dg)
		selfUser, err := dg.User("@me")
		if err == nil {
			mainUsername = selfUser.Username + "#" + selfUser.Discriminator
			mainAvatar = selfUser.AvatarURL("")
			logWithTime("<cyan>Sniping on </><yellow>" + strconv.Itoa(len(dg.State.Guilds)) + "</><cyan> guilds on main account </><yellow>" + userIntoUsername(selfUser) + "</><cyan>!</>")
		} else {
			logWithTime("<cyan>Sniping on </><yellow>" + strconv.Itoa(len(dg.State.Guilds)) + "</><cyan> guilds on main token!</>")
		}
		altsAmount++
	} else if config.ClaimOnMain {
		dg, err = discordgo.New(config.MainToken)
		dg.LogLevel = -1
		if err != nil {
			fatalWithTime("<red>Error creating session for main token!</>")
		}
		err = dg.Open()
		if err != nil {
			fatalWithTime("<red>Error opening connection for main token!</>")
		}
		selfUser, err := dg.User("@me")
		if err == nil {
			mainUsername = selfUser.Username + "#" + selfUser.Discriminator
			mainAvatar = selfUser.AvatarURL("")
		}
		dg.Close()
	}

	if mainUsername == "" {
		mainUsername = "Velocity"
		mainAvatar = "https://files.catbox.moe/d2s7og.jpeg"
	}

	if config.UseAlts && len(config.AltTokens) != 0 {
		processed, maxProcessed := 0, len(config.AltTokens)
		for token := range authedAlts {
			if strings.HasPrefix(token, "?") {
				removeAltToken(token)
			}
			processed++
			altsAmount++
			if processed == maxProcessed {
				close(authedAlts)
				break
			}
		}
	}

	go updateTitleRegular()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	if len(allSessions) > 0 {
		for _, session := range allSessions {
			session.Close()
		}
	}
}

func getPaymentSourceID(token string) {
	var strRequestURI = []byte("https://discord.com/api/v9/users/@me/billing/payment-sources")
	req := fasthttp.AcquireRequest()
	req.Header.Set("Authorization", token)
	req.Header.SetMethodBytes([]byte("GET"))
	req.SetRequestURIBytes(strRequestURI)
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
	}

	fasthttp.ReleaseRequest(req)

	body := res.Body()

	id := rePaymentSourceID.FindStringSubmatch(string(body))

	if id == nil {
		paymentSourceIDs[token] = "null"
	}
	if len(id) > 1 {
		paymentSourceIDs[token] = id[2]
	}
}

func connectAltToken(token string, authedAlts chan string) {
	dg, err := discordgo.New(token)
	//dg.LogLevel = -1
	if err != nil {
		logWithTime("<red>Error creating session for " + token + "! " + err.Error() + "</>")
		authedAlts <- "?" + token
	} else {
		dg.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.159 Safari/537.36"
		err = dg.Open()
		if err != nil {
			logWithTime("<red>Error opening connection for " + token + "! " + err.Error() + "</>")
			authedAlts <- "?" + token
		} else {
			if len(dg.State.Guilds) == 0 {
				logWithTime("<red>No servers found for " + token + "! Disconnecting.</>")
				authedAlts <- "?" + token
				return
			}
			selfUser, err := dg.User("@me")
			if err == nil {
				logWithTime("<cyan>Sniping on </><yellow>" + strconv.Itoa(len(dg.State.Guilds)) + "</><cyan> guilds on alt account </><yellow> " + userIntoUsername(selfUser) + "</><cyan>!</>")
			} else {
				logWithTime("<cyan>Sniping on </><yellow>" + strconv.Itoa(len(dg.State.Guilds)) + "</><cyan> guilds on token</><yellow> " + token + "</><cyan>!</>")
			}
			dg.AddHandler(messageCreate)
			appendSession(token, dg)
			getPaymentSourceID(token)
			authedAlts <- token
		}
	}
}

func checkGiftLink(s *discordgo.Session, m *discordgo.MessageCreate, code string, start time.Time) {
	if cacheCodes[code] {
		end := time.Now()
		diff := end.Sub(start)
		if !config.RemoveDuplicates {
			guildName := getGuildName(s, m.GuildID)
			authorName := m.Author.Username + "#" + m.Author.Discriminator
			if config.ShowDuplicates {
				logWithTime("<yellow>Detected duplicate Nitro! | Server: " + guildName + " | Delay: " + diffToString(diff) + "s | Code: " + code + " | From: " + authorName + "</>")
			}
		}
		return
	}

	var token string
	var channelID string
	if config.ClaimOnMain {
		token = config.MainToken
		channelID = "null"
	} else {
		token = s.Identify.Token
		channelID = m.ChannelID
	}
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.SetContentType("application/json")
	req.Header.Set("Authorization", token)
	req.SetBodyString(`{"channel_id":` + channelID + `,"payment_source_id":` + paymentSourceIDs[token] + `}`)
	req.Header.SetMethod("POST")
	req.SetRequestURI("https://discordapp.com/api/v9/entitlements/gift-codes/" + code + "/redeem")
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)
	if err := fasthttp.Do(req, resp); err != nil {
		return
	}

	end := time.Now()
	diff := end.Sub(start)

	body := resp.Body()
	bodyString := string(body)
	guildName := getGuildName(s, m.GuildID)

	authorName := m.Author.Username + "#" + m.Author.Discriminator
	fmt.Println(m.Content)
	checkRedeemResponse(bodyString, resp.StatusCode(), code, s.Identify.Token, guildName, authorName, diff)
}

func checkRedeemResponse(bodyString string, statusCode int, code, token string, guild string, author string, diff time.Duration) {
	var response responseStruct
	err := json.Unmarshal([]byte(bodyString), &response)
	if err != nil {
		return
	}
	fmt.Println(response.Message)
	if strings.Contains(bodyString, "redeemed") && statusCode >= 400 {
		logWithTime("<red>Nitro already redeemed! | Server: " + guild + " | Delay: " + diffToString(diff) + "s | Code: " + code + " | From: " + author + "</>")
	} else if strings.Contains(bodyString, "nitro") || statusCode < 300 {
		nitroType := ""
		if reNitroType.Match([]byte(bodyString)) {
			nitroType = reNitroType.FindStringSubmatch(bodyString)[1]
		}
		logWithTime("<green>Sniped Nitro! | Type: " + nitroType + " | Server: " + guild + " | Delay: " + diffToString(diff) + "s | Code: " + code + " | From: " + author + " | Token (Last 5): " + token[len(token)-5:] + "</>")
		if config.ClaimOnMain {
			webhookNitro(diffToString(diff), nitroType, mainUsername, mainAvatar)
		} else {
			self, err := allSessions[token].User("@me")
			if err == nil {
				webhookNitro(diffToString(diff), nitroType, userIntoUsername(self), self.AvatarURL(""))
				webhookNitro(diffToString(diff), nitroType, userIntoUsername(self), self.AvatarURL(""))
			} else {
				webhookNitro(diffToString(diff), nitroType, mainUsername, mainAvatar)
				webhookNitro(diffToString(diff), nitroType, mainUsername, mainAvatar)
			}
		}
	} else if statusCode == 404 {
		logWithTime("<yellow>Detected unknown Nitro! | Server: " + guild + " | Delay: " + diffToString(diff) + "s | Code: " + code + " | From: " + author + "</>")
	} else if statusCode == 403 || statusCode == 401 {
		logWithTime("<red>Token unauthorized, can't claim code! | Server: " + guild + " | Delay: " + diffToString(diff) + "s | Code: " + code + " | Token: " + token + " | From: " + author + "</>")
	} else {
		logWithTime("<red>" + response.Message + " | Server: " + guild + " | Delay: " + diffToString(diff) + "s | Code: " + code + " | From: " + author + "</>")
	}
	cacheCodes[code] = true
	if config.RemoveDuplicates {
		saveNitroCode(code)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	go func() {
		/*code, success := extractNitroCode(m.Content)
		if success {
			checkGiftLink(s, m, code, time.Now())
		}*/
		codes := extractNitroCodeRegex(m.Content)
		for _, code := range codes {
			go checkGiftLink(s, m, code, time.Now())
		}
	}()
}

//function that extracts code from message
//more complicated but about 20 times faster than regex
func extractNitroCode(input string) (string, bool) {
	tempInput := strings.ToLower(input)
	code := ""
	start := strings.Index(tempInput, "/gifts/")
	if start != -1 {
		code = input[start+7:]
		for spaceIndex, space := range code {
			if space == ' ' || space == '/' || spaceIndex == len(tempInput)-1 {
				code = code[:spaceIndex]
				break
			}
		}
	} else {
		start := strings.Index(tempInput, ".gift/")
		if start != -1 {
			code = input[start+6:]
			for spaceIndex, space := range code {
				if space == ' ' || space == '/' || spaceIndex == len(tempInput)-1 {
					code = code[:spaceIndex]
					break
				}
			}
		}
	}
	if len(code) != 16 || len(code) != 24 {
		return code, false
	}
	return code, true
}

/*func newExtractor(input string) string {
	l := len(input)
	for i, r := range input {
		if r == 'g' && i+4 < l && input[i+3] == 't' {

		}
	}
	return ""
}*/

func extraNitroSplit(input string) string {
	return strings.Split(strings.Split(input, ".gift/")[1], "/")[0]
}

//alternative regex solution
func extractNitroCodeRegex(input string) []string {
	codes := reGiftLink.FindStringSubmatch(input)
	results := []string{}
	for _, code := range codes {
		//16+5 and 24+5
		if len(code) >= 21 && len(code) <= 29 {
			results = append(results, code[5:])
		}
	}
	return results
}

func saveNitroCode(nitroCode string) {
	nitroCodeFileMutex.Lock()
	defer nitroCodeFileMutex.Unlock()
	file, err := os.OpenFile("nitrocodes.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logWithTime("<red>Error opening Nitro codes file! " + err.Error() + "</>")
		return
	}
	defer file.Close()
	_, err = file.WriteString(nitroCode + "\n")
	if err != nil {
		logWithTime("<red>Error saving Nitro code! " + err.Error() + "</>")
	}
}

func updateTitleRegular() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		updateTitle()
	}
}

func updateTitle() {
	diff := time.Since(timeStarted)
	timeRunning := formatDuration(diff)
	title := fmt.Sprintf("Arizona Sniper | Alts: %d | Time Running: %s | Nitros Sniped: %d ", altsAmount, timeRunning, nitrosSniped)
	cmd := exec.Command("cmd", "/C", "title", title)
	cmd.Run()
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
