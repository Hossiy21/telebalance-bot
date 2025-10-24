package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

var usersFile = "users.json"
var userSet = make(map[int64]string)

// Set your Telegram user ID as admin
const adminID int64 = 413906777

// -------------------- USER MANAGEMENT --------------------

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

// -------------------- BALANCE PARSER --------------------

type BalanceInfo struct {
	OriginalMinutes  string
	OriginalData     string
	OriginalSMS      string
	RemainingMinutes string
	RemainingData    string
	RemainingSMS     string
}

func parseBalanceMessage(text string) *BalanceInfo {
	// Check if message contains Ethio Telecom keywords (more flexible)
	lowerText := strings.ToLower(text)
	if !strings.Contains(lowerText, "remaining") &&
		!strings.Contains(lowerText, "package") &&
		!strings.Contains(lowerText, "pack") &&
		!strings.Contains(lowerText, "balance") &&
		!strings.Contains(lowerText, "ethio") {
		return nil
	}

	balance := &BalanceInfo{
		OriginalMinutes:  "0",
		OriginalData:     "0 MB",
		OriginalSMS:      "0",
		RemainingMinutes: "0",
		RemainingData:    "0 MB",
		RemainingSMS:     "0",
	}

	// Extract original package info - try multiple patterns

	// Pattern 1: Student pack with bonus "XMin + YSMS plus ZMin night bonus"
	studentPackRegex := regexp.MustCompile(`(?i)(\d+)\s*Min\s*\+\s*(\d+)\s*SMS(?:\s+plus\s+(\d+)\s*Min)?`)
	if match := studentPackRegex.FindStringSubmatch(text); len(match) > 2 {
		baseMinutes := 0
		bonusMinutes := 0
		fmt.Sscanf(match[1], "%d", &baseMinutes)
		if len(match) > 3 && match[3] != "" {
			fmt.Sscanf(match[3], "%d", &bonusMinutes)
		}
		totalMinutes := baseMinutes + bonusMinutes
		balance.OriginalMinutes = fmt.Sprintf("%d", totalMinutes)
		balance.OriginalSMS = match[2]
	}

	// Pattern 2: Combo pack "X Min, Y MB/GB and Z SMS"
	if balance.OriginalMinutes == "0" {
		comboRegex := regexp.MustCompile(`(?i)(\d+)\s*Min[,\s]+(\d+(?:\.\d+)?)\s*(MB|GB)(?:\s+and|\s*,)\s*(\d+)\s*SMS`)
		if match := comboRegex.FindStringSubmatch(text); len(match) > 4 {
			balance.OriginalMinutes = match[1]
			balance.OriginalSMS = match[4]
			dataValue := match[2]
			dataUnit := strings.ToUpper(match[3])
			if dataUnit == "MB" {
				var mbValue float64
				fmt.Sscanf(dataValue, "%f", &mbValue)
				balance.OriginalData = fmt.Sprintf("%.1f GB", mbValue/1024.0)
			} else {
				balance.OriginalData = dataValue + " " + dataUnit
			}
		}
	}

	// Pattern 3: Voice/SMS only "X Min and Y SMS"
	if balance.OriginalMinutes == "0" {
		voiceSMSRegex := regexp.MustCompile(`(?i)(\d+)\s*Min(?:ute)?s?\s+(?:and\s+)?(\d+)\s*SMS`)
		if match := voiceSMSRegex.FindStringSubmatch(text); len(match) > 2 {
			balance.OriginalMinutes = match[1]
			balance.OriginalSMS = match[2]
		}
	}

	// Pattern 3: Data packages - look for any number followed by GB/MB
	if balance.OriginalData == "0 MB" {
		dataRegex := regexp.MustCompile(`(?i)(?:from|package|pack|bundle).*?(\d+(?:\.\d+)?)\s*(GB|MB)`)
		if match := dataRegex.FindStringSubmatch(text); len(match) > 2 {
			dataValue := match[1]
			dataUnit := strings.ToUpper(match[2])
			if dataUnit == "MB" {
				var mbValue float64
				fmt.Sscanf(dataValue, "%f", &mbValue)
				if mbValue >= 100 { // Only convert if it's a package size (not remaining)
					balance.OriginalData = fmt.Sprintf("%.1f GB", mbValue/1024.0)
				} else {
					balance.OriginalData = dataValue + " " + dataUnit
				}
			} else {
				balance.OriginalData = dataValue + " " + dataUnit
			}
		}
	}

	// Extract remaining balances - look for patterns after "remaining" or "is"

	// Remaining SMS - flexible patterns
	remainingSMSPatterns := []string{
		`(?i)(?:remaining|is)\s+(\d+)\s*SMS`,
		`(?i)SMS.*?(?:remaining|is)\s+(\d+)`,
	}
	for _, pattern := range remainingSMSPatterns {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(text); len(match) > 1 {
			balance.RemainingSMS = match[1]
			break
		}
	}

	// Remaining minutes - aggregate all occurrences (for messages with multiple entries)
	minuteRegex := regexp.MustCompile(`(?i)is\s+(\d+)\s*minute`)
	minuteMatches := minuteRegex.FindAllStringSubmatch(text, -1)
	if len(minuteMatches) > 0 {
		totalMinutes := 0
		for _, match := range minuteMatches {
			if len(match) > 1 {
				var mins int
				fmt.Sscanf(match[1], "%d", &mins)
				totalMinutes += mins
			}
		}
		balance.RemainingMinutes = fmt.Sprintf("%d", totalMinutes)
	}

	// Remaining data - flexible patterns
	remainingDataPatterns := []string{
		`(?i)(?:remaining|is)\s+(\d+(?:\.\d+)?)\s*(MB|GB)`,
		`(?i)(MB|GB).*?(?:remaining|is)\s+(\d+(?:\.\d+)?)`,
	}
	for _, pattern := range remainingDataPatterns {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(text); len(match) > 2 {
			var dataValue, dataUnit string
			if match[2] != "" && (strings.ToUpper(match[2]) == "MB" || strings.ToUpper(match[2]) == "GB") {
				dataValue = match[1]
				dataUnit = strings.ToUpper(match[2])
			} else {
				dataValue = match[2]
				dataUnit = strings.ToUpper(match[1])
			}

			// Convert to GB if >= 1024 MB
			if dataUnit == "MB" {
				var mbValue float64
				fmt.Sscanf(dataValue, "%f", &mbValue)
				if mbValue >= 1024 {
					balance.RemainingData = fmt.Sprintf("%.2f GB", mbValue/1024.0)
				} else {
					balance.RemainingData = fmt.Sprintf("%.0f MB", mbValue)
				}
			} else {
				balance.RemainingData = dataValue + " " + dataUnit
			}
			break
		}
	}

	// If no remaining data found but original had data, assume 0 MB remaining
	if balance.RemainingData == "0 MB" && balance.OriginalData != "0 MB" {
		balance.RemainingData = "0 MB"
	}

	return balance
}

func formatBalanceMessage(balance *BalanceInfo) string {
	msg := fmt.Sprintf(
		"📝 <b>Original Package</b>\n"+
			"Minutes: %s\n"+
			"Data: %s\n"+
			"SMS: %s\n\n"+
			"💬 <b>Remaining Balance</b>\n"+
			"Minutes: %s\n"+
			"Data: %s\n"+
			"SMS: %s\n\n"+
			"👉 <a href=\"https://t.me/Hossiy_DevDiary\">Join our channel for more powerful resources</a> 👈",
		balance.OriginalMinutes,
		balance.OriginalData,
		balance.OriginalSMS,
		balance.RemainingMinutes,
		balance.RemainingData,
		balance.RemainingSMS,
	)

	return msg
}

// -------------------- MAIN FUNCTION --------------------

func main() {
	// Load .env for TELEGRAM_BOT_TOKEN and optional PORT
	if _, exists := os.LookupEnv("TELEGRAM_BOT_TOKEN"); !exists {
		_ = godotenv.Load()
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set in environment or .env file")
	}

	// Retry connection with timeout handling
	var bot *tgbotapi.BotAPI
	var err error
	maxRetries := 5

	for i := 0; i < maxRetries; i++ {
		log.Printf("🔄 Attempting to connect to Telegram API (attempt %d/%d)...", i+1, maxRetries)
		bot, err = tgbotapi.NewBotAPI(botToken)
		if err == nil {
			break
		}

		log.Printf("⚠️ Connection failed: %v", err)
		if i < maxRetries-1 {
			log.Printf("⏳ Retrying in 3 seconds...")
			time.Sleep(3 * time.Second)
		}
	}

	if err != nil {
		log.Fatal("❌ Failed to connect to Telegram API after multiple attempts. Please check your internet connection and try again.")
	}

	bot.Debug = false
	log.Printf("✅ Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Load users
	loadUsers()

	// Keep Render / hosting alive
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "Telegram summarizer bot is running ✅")
		})
		log.Printf("🌐 Listening on port %s\n", port)
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}()

	// -------------------- BOT LOGIC --------------------

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID
		userName := update.Message.From.FirstName

		// Track users
		if _, exists := userSet[userID]; !exists {
			userSet[userID] = userName
			saveUsers()
			log.Printf("👤 New user added: %s (%d). Total users: %d", userName, userID, len(userSet))
		}

		text := update.Message.Text

		// Handle /start
		if text == "/start" {
			welcomeMsg := fmt.Sprintf(
				"👋 Hi %s!\n\nWelcome to <b>Ethio Tele Balance Bot</b> 🤖\n\nJust forward me your Ethio Telecom balance SMS and I'll format it nicely for you! 📱✨",
				userName,
			)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMsg)
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		// Handle /stats — admin only
		if text == "/stats" {
			if userID == adminID {
				count := len(userSet)
				msgText := fmt.Sprintf("📊 <b>Total users:</b> %d\n\n👥 User list:\n", count)
				for id, name := range userSet {
					msgText += fmt.Sprintf("- <a href=\"tg://user?id=%d\">%s</a>\n", id, name)
				}
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
				msg.ParseMode = "HTML"
				bot.Send(msg)
			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⚠️ This command is for the bot owner only.")
				bot.Send(msg)
			}
			continue
		}

		// Check if it's a balance message
		if balanceInfo := parseBalanceMessage(text); balanceInfo != nil {
			bot.Send(tgbotapi.NewChatAction(update.Message.Chat.ID, tgbotapi.ChatTyping))

			formattedMsg := formatBalanceMessage(balanceInfo)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, formattedMsg)
			msg.ParseMode = "HTML"
			msg.DisableWebPagePreview = true
			bot.Send(msg)
			continue
		}

		// For all other text messages
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🤔 Sorry, I couldn't understand that message.\nPlease send a valid Ethio Telecom balance SMS.")
		bot.Send(msg)
	}
}
