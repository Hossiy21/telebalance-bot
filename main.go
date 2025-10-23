package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	openai "github.com/openai/openai-go"
)

var usersFile = "users.json"
var userSet = make(map[int64]string)

// Set your Telegram user ID as admin
const adminID int64 = 413906777

// ================= USER MANAGEMENT =================
func saveUsers() {
	data, _ := json.MarshalIndent(userSet, "", "  ")
	_ = ioutil.WriteFile(usersFile, data, 0644)
}

func loadUsers() {
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

// ================= AI SUMMARIZER =================
func summarizeWithAI(message string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("missing OPENAI_API_KEY")
	}

	client := openai.NewClient(apiKey)

	resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: openai.F(openai.ChatModelGPT4oMini),
		Messages: openai.F([]openai.ChatCompletionMessageParam{
			{
				Role:    openai.F(openai.ChatCompletionMessageRoleSystem),
				Content: openai.F("You are an assistant that summarizes Ethio Telecom SMS into a short structured report with key data (original minutes, remaining minutes, GB, SMS, etc.)."),
			},
			{
				Role:    openai.F(openai.ChatCompletionMessageRoleUser),
				Content: openai.F(fmt.Sprintf("Summarize clearly and neatly for Telegram:\n\n%s", message)),
			},
		}),
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == nil {
		return "", fmt.Errorf("empty AI response")
	}

	summary := *resp.Choices[0].Message.Content
	return fmt.Sprintf("🧠 <b>Summary</b>\n%s\n\n👉 <a href=\"https://t.me/Hossiy_DevDiary\">Join our channel for more resources</a>", summary), nil
}

// ================= REGEX FALLBACK =================
func summarizeWithRegex(msg string) string {
	origRe := regexp.MustCompile(`Monthly voice (\d+) Min,([\d.]+)GB and (\d+) from telebirr SMS`)
	origMatch := origRe.FindStringSubmatch(msg)
	if len(origMatch) < 4 {
		return `🤔 <b>Sorry, I couldn’t understand that message.</b>
<i>Please send a valid Ethio Telecom SMS like:</i> 
"Dear Customer, your remaining Monthly voice is 219 Min..."

👉 <a href="https://t.me/Hossiy_DevDiary">Join our channel for more powerful resources</a> 👈`
	}

	origMinutes, origData, origSMS := origMatch[1], origMatch[2], origMatch[3]

	remRe := regexp.MustCompile(`is (\d+) minute(?:s)? and (\d+) second`)
	remMatch := remRe.FindStringSubmatch(msg)
	if len(remMatch) < 3 {
		return `❌ I couldn’t find remaining balance details.
Please include something like "is 183 minute and 10 second" in your message.

👉 <a href="https://t.me/Hossiy_DevDiary">Join our channel for more powerful resources</a> 👈`
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

👉 <a href="https://t.me/Hossiy_DevDiary">Join our channel for more powerful resources</a> 👈`,
		origMinutes, origData, origSMS,
		remainingMinutes, remainingData, remainingSMS)

	return shortMsg
}

// ================= MAIN =================
func main() {
	// Load .env locally, ignore error in Render (env vars already set)
	_ = godotenv.Load()

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("❌ TELEGRAM_BOT_TOKEN is missing")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("✅ Logged in as %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	loadUsers()

	// Simple HTTP keep-alive server for Render
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
		chatID := update.Message.Chat.ID
		userMsg := update.Message.Text

		// Save unique users
		if _, exists := userSet[userID]; !exists {
			userSet[userID] = userName
			saveUsers()
			log.Printf("New user added: %s (%d). Total users: %d", userName, userID, len(userSet))
		}

		// Handle /start
		if strings.HasPrefix(userMsg, "/start") {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("👋 Hi %s! Welcome to Ethio Tele Package Shortener Bot.\nSend your Ethio Telecom package SMS and I’ll summarize it neatly using AI. ⚡", userName))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// Handle /stats (admin only)
		if strings.HasPrefix(userMsg, "/stats") {
			if userID == adminID {
				count := len(userSet)
				msgText := fmt.Sprintf("📊 Total unique users: %d\n\n👥 User list (click to chat):\n", count)
				for id, name := range userSet {
					msgText += fmt.Sprintf("- <a href=\"tg://user?id=%d\">%s</a>\n", id, name)
				}
				msg := tgbotapi.NewMessage(chatID, msgText)
				msg.ParseMode = "HTML"
				bot.Send(msg)
			} else {
				bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Sorry, this command is only for the bot owner."))
			}
			continue
		}

		// ================= MESSAGE HANDLING =================
		var summary string
		summary, err := summarizeWithAI(userMsg)
		if err != nil {
			log.Println("AI summarization failed, falling back to regex:", err)
			summary = summarizeWithRegex(userMsg)
		}

		msg := tgbotapi.NewMessage(chatID, summary)
		msg.ParseMode = "HTML"
		msg.DisableWebPagePreview = true
		bot.Send(msg)
	}
}
