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

// 👇 Change this to your Telegram user ID (you can get it using @userinfobot)
const adminID int64 = 413906777 // Replace this with your own Telegram user ID

func saveUsers() {
	data, _ := json.MarshalIndent(userSet, "", "  ")
	_ = ioutil.WriteFile(usersFile, data, 0644)
}

func loadUsers() {
	data, err := ioutil.ReadFile(usersFile)
	if err == nil {
		_ = json.Unmarshal(data, &userSet)
	}
}

func parseMessage(msg string) string {
	origRe := regexp.MustCompile(`Monthly voice (\d+) Min,([\d.]+)GB and (\d+) from telebirr SMS`)
	origMatch := origRe.FindStringSubmatch(msg)
	if len(origMatch) < 4 {
		return `🤔 <b>Sorry, I couldn’t understand that message.</b>
<i>Please send a valid package text like:</i> "Dear Customer, your remaining Monthly voice is 219 Min..."

👉 <a href="https://t.me/Hossiy_DevDiary"> Join our channel for more powerful resources</a> 👈`
	}

	origMinutes, origData, origSMS := origMatch[1], origMatch[2], origMatch[3]

	remRe := regexp.MustCompile(`is (\d+) minute(?:s)? and (\d+) second`)
	remMatch := remRe.FindStringSubmatch(msg)
	if len(remMatch) < 3 {
		return `❌ I couldn’t find remaining balance details.
Please include something like "is 183 minute and 10 second" in your message.

👉 <a href="https://t.me/Hossiy_DevDiary"> Join our channel for more powerful resources</a> 👈`
	}

	remainingMinutes := remMatch[1]
	remainingData := "0"
	remainingSMS := origSMS

	shortMsg := fmt.Sprintf(`📝 <b>Original Package</b>
Minutes: %s
Data: %s GB
SMS: %s

💬 <b>Remaining Balance</b>
Minutes: %s
Data: %s MB
SMS: %s

👉 <a href="https://t.me/Hossiy_DevDiary"> Join our channel for more powerful resources</a> 👈`,
		origMinutes, origData, origSMS,
		remainingMinutes, remainingData, remainingSMS)

	return shortMsg
}

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

	// Load previous users
	loadUsers()

	// ✅ Start minimal HTTP server (Render requirement)
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
			log.Printf("New user added: %s (%d)", userName, userID)
		}

		// Handle /start
		if update.Message.Text == "/start" {
			welcomeMsg := fmt.Sprintf("👋 Hi %s! Welcome to Ethio Tele Package Shortener Bot.\nSend your Ethio Telecom package SMS, and I’ll summarize it neatly. ⚡", userName)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMsg)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// Handle /stats — only admin can view
		if update.Message.Text == "/stats" {
			if userID == adminID {
				count := len(userSet)
				msgText := fmt.Sprintf("📊 Total unique users: %d", count)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
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
