package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

var usersFile = "users.json"
var userSet = make(map[int64]string)

// Set your Telegram user ID as admin
const adminID int64 = 413906777

func saveUsers() {
	data, _ := json.MarshalIndent(userSet, "", "  ")
	_ = ioutil.WriteFile(usersFile, data, 0644)
}

func loadUsers() {
	// Auto-create file if it doesn't exist
	if _, err := os.Stat(usersFile); os.IsNotExist(err) {
		userSet = make(map[int64]string)
		saveUsers()
		return
	}

	data, err := ioutil.ReadFile(usersFile)
	if err != nil {
		log.Println("Error reading users file:", err)
		return
	}

	_ = json.Unmarshal(data, &userSet)
}

// 🚀 CORRECTED AND ENHANCED parseMessage function to handle complex, concatenated SMS
func parseMessage(msg string) string {
	// Initialize default values
	origMinutes := "N/A"
	origSMS := "N/A"
	origData := "N/A"

	remainingMinutes := "0"
	remainingDataMB := "0"
	remainingSMS := "0"

	// --- 1. Extract Original VOICE & SMS Package ---
	// Matches: 'Monthly student pack 234Min + 120SMS plus 234Min night bonus from telebirr'
	// Captures: Type (Monthly/Daily/etc), Voice (234), SMS (120)
	origVoiceSMSRe := regexp.MustCompile(`(Monthly|Daily|Weekly|Holiday)\s+.*?(\d+)Min\s*\+\s*(\d+)SMS.*?\s+from telebirr`)
	origVoiceSMSMatch := origVoiceSMSRe.FindStringSubmatch(msg)

	if len(origVoiceSMSMatch) >= 4 {
		origMinutes = origVoiceSMSMatch[2]
		origSMS = origVoiceSMSMatch[3]
	}

	// --- 2. Extract Original Data Package (Optional) ---
	// Matches: 'Daily 1.5 GB Telegram + WhatsApp'
	// Captures: Type (Daily/Weekly/etc), Data (1.5)
	origDataRe := regexp.MustCompile(`(Daily|Weekly|Monthly|Holiday)\s+([\d.]+)\s+GB`)
	origDataMatch := origDataRe.FindStringSubmatch(msg)

	if len(origDataMatch) >= 3 {
		origData = origDataMatch[2]
	}

	// Fail if no core package info is found
	if origMinutes == "N/A" && origData == "N/A" {
		return `🤔 <b>Sorry, I couldn’t understand that message.</b>
<i>The message format is too complex or different from expected.</i>
<br>Expected to find: <b>[Type] [Min] + [SMS]</b> OR <b>[Type] [GB]</b>
👉 <a href="https://t.me/Hossiy_DevDiary"> Join our channel for more powerful resources</a> 👈`
	}

	// --- 3. Extract Remaining DATA (in MB) ---
	// Matches: 'is 1536.000 MB with expiry date'
	// Captures: Remaining Data (1536.000)
	remDataRe := regexp.MustCompile(`is ([\d\.]+) MB with expiry date`)
	remDataMatch := remDataRe.FindStringSubmatch(msg)
	if len(remDataMatch) >= 2 {
		remainingDataMB = remDataMatch[1]
	}

	// --- 4. Extract Remaining MINUTES ---
	// Matches: 'is 90 minute and 38 second'
	// Captures: Remaining Minutes (90)
	remMinRe := regexp.MustCompile(`is (\d+) minute(?:s)? and (\d+) second`)
	remMinMatch := remMinRe.FindStringSubmatch(msg)
	if len(remMinMatch) >= 3 {
		remainingMinutes = remMinMatch[1]
	}

	// --- 5. Extract Remaining SMS ---
	// Matches: 'is 111 SMS with expiry date'
	// Captures: Remaining SMS (111)
	remSMSRe := regexp.MustCompile(`is (\d+) SMS with expiry date`)
	remSMSMatch := remSMSRe.FindStringSubmatch(msg)
	if len(remSMSMatch) >= 2 {
		remainingSMS = remSMSMatch[1]
	}

	// --- 6. Construct the Summary Message ---
	shortMsg := fmt.Sprintf(
		`📝 <b>Original Package(s)</b>
Minutes: %s Min
Data: %s GB
SMS: %s

💬 <b>Remaining Balance</b>
Minutes: %s Min
Data: %s MB
SMS: %s

👉 <a href="https://t.me/Hossiy_DevDiary"> Join our channel for more powerful resources</a> 👈`,
		origMinutes, origData, origSMS, remainingMinutes, remainingDataMB, remainingSMS,
	)

	return shortMsg
}

func main() {
	if _, exists := os.LookupEnv("TELEGRAM_BOT_TOKEN"); !exists {
		// Attempt to load .env file if the token isn't set
		_ = godotenv.Load()
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set. Please set the environment variable or create a .env file.")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Load previous users or create file
	loadUsers()

	// Start minimal HTTP server for Render/deployment
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "Telegram bot is running ✅")
		})
		log.Printf("Listening on port %s\n", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID
		userName := update.Message.From.FirstName

		// Save unique users
		if _, exists := userSet[userID]; !exists {
			userSet[userID] = userName
			saveUsers()
			log.Printf("New user added: %s (%d). Total users: %d", userName, userID, len(userSet))
		}

		// Handle /start
		if update.Message.Text == "/start" {
			welcomeMsg := fmt.Sprintf(
				"👋 Hi %s! Welcome to Ethio Tele Package Shortener Bot.\nSend your Ethio Telecom package SMS (Monthly, Daily, Weekly, Holiday), and I’ll summarize it neatly. ⚡",
				userName,
			)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMsg)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// Handle /stats — admin only
		if update.Message.Text == "/stats" {
			if userID == adminID {
				count := len(userSet)
				msgText := fmt.Sprintf("📊 Total unique users: %d\n\n", count)
				msgText += "👥 User list (click to chat):\n"
				for id, name := range userSet {
					msgText += fmt.Sprintf("- <a href=\"tg://user?id=%d\">%s</a>\n", id, name)
				}
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
				msg.ParseMode = "HTML"
				bot.Send(msg)
			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ Sorry, this command is only for the bot owner.")
				bot.Send(msg)
			}
			continue
		}

		// Handle other messages
		shortMsg := parseMessage(update.Message.Text)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, shortMsg)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		bot.Send(msg)
	}
}
