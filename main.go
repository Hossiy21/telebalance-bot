package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// Function to parse the Ethio Telecom SMS message
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
	// Load .env file locally if environment variables are not set
	if _, exists := os.LookupEnv("TELEGRAM_BOT_TOKEN"); !exists {
		_ = godotenv.Load()
	}

	// Get Telegram bot token
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set")
	}

	// Initialize bot
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// ✅ Start small HTTP server for Render detection
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
			log.Fatal(err)
		}
	}()

	// ✅ Handle Telegram updates
	for update := range updates {
		if update.Message == nil {
			continue
		}

		userName := update.Message.From.FirstName

		// Handle /start command
		if update.Message.Text == "/start" {
			welcomeMsg := fmt.Sprintf("👋 Hello %s! Welcome to Ethio Tele Package Shortener Bot.\nPlease send your Ethio Telecom package SMS to get a quick summary. ⚡", userName)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMsg)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// Handle normal messages
		shortMsg := parseMessage(update.Message.Text)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, shortMsg)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		bot.Send(msg)
	}
}
