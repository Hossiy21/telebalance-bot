package main

import (
	"fmt"
	"log"
	"os"
	"regexp"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func parseMessage(msg string) string {
	// Extract original minutes, data, and SMS
	origRe := regexp.MustCompile(`Monthly voice (\d+) Min,([\d.]+)GB and (\d+) from telebirr SMS`)
	origMatch := origRe.FindStringSubmatch(msg)
	if len(origMatch) < 4 {
		return `🤔 <b>Sorry, I couldn’t understand that message. </b><i>Please send a valid package text like: </i> "Dear Customer, your remaining Monthly voice is 219 Min..."

👉 <a href="https://t.me/Hossiy_DevDiary"> Join our channel for more powerful resources</a> 👈`
	}

	origMinutes, origData, origSMS := origMatch[1], origMatch[2], origMatch[3]

	// Extract remaining minutes
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
	// Load .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get token from env
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set in .env")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			shortMsg := parseMessage(update.Message.Text)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, shortMsg)
			msg.ParseMode = "HTML"
			msg.DisableWebPagePreview = true
			bot.Send(msg)
		}
	}
}
