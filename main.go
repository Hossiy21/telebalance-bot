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

// 🚀 CORRECTED AND ENHANCED parseMessage function
func parseMessage(msg string) string {
	// 1. Regex to extract Original Package (Minutes, GB, SMS)
	// It now accepts Monthly, Daily, Weekly, or Holiday package types.
	// (?:...) is a non-capturing group for the package type.
	origRe := regexp.MustCompile(`(?:Monthly|Daily|Weekly|Holiday) voice (\d+) Min,([\d.]+)GB and (\d+) from telebirr SMS`)
	origMatch := origRe.FindStringSubmatch(msg)

	if len(origMatch) < 4 {
		return `🤔 <b>Sorry, I couldn’t understand that message.</b>
<i>Please send a valid package text that includes:</i>
<ul>
<li>A package type (Monthly, Daily, Weekly, or Holiday)</li>
<li>Minutes, GB, and SMS count</li>
</ul>
👉 <a href="https://t.me/Hossiy_DevDiary"> Join our channel for more powerful resources</a> 👈`
	}

	// Captured groups for original package (indices 1, 2, 3)
	origMinutes, origData, origSMS := origMatch[1], origMatch[2], origMatch[3]

	// Initialize remaining values
	remainingMinutes := "Unknown ❓"
	remainingDataMB := "0"
	remainingSMS := origSMS // Default to original SMS, as remaining SMS is often not in the remaining balance SMS

	// 2. Regex to extract Remaining Minutes
	remRe := regexp.MustCompile(`is (\d+) minute(?:s)? and (\d+) second`)
	remMatch := remRe.FindStringSubmatch(msg)
	if len(remMatch) >= 3 {
		remainingMinutes = remMatch[1]
	}

	// 3. Regex to extract Remaining Data in MB
	dataRemRe := regexp.MustCompile(`remaining data balance is ([\d.]+)MB`)
	dataRemMatch := dataRemRe.FindStringSubmatch(msg)
	if len(dataRemMatch) >= 2 {
		remainingDataMB = dataRemMatch[1]
	}

	// 4. Construct the summary message
	shortMsg := fmt.Sprintf(
		`📝 <b>Original Package</b>
Minutes: %s
Data: %s GB
SMS: %s

💬 <b>Remaining Balance</b>
Minutes: %s
Data: %s MB
SMS: %s

👉 <a href="https://t.me/Hossiy_DevDiary"> Join our channel for more powerful resources</a> 👈`,
		origMinutes, origData, origSMS, remainingMinutes, remainingDataMB, remainingSMS,
	)

	return shortMsg
}
// ----------------------------------------------------------------------
// main function remains the same

func main() {
	if _, exists := os.LookupEnv("TELEGRAM_BOT_TOKEN"); !exists {
		_ = godotenv.Load()
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set")
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

	// Start minimal HTTP server for Render
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "Telegram bot is running ✅")
		})
		log.Printf("Listening on port %s\n", port)
		log.Fatal(http.ListenAndServe(":"+port, nil))
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
