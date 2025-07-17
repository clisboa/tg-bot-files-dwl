package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Parse command line arguments
	var (
		botToken   = flag.String("token", os.Getenv("TELEGRAM_BOT_TOKEN"), "Telegram bot token")
		folder     = flag.String("folder", os.Getenv("TELEGRAM_FOLDER"), "Download folder path")
		allowedUID = flag.String("user", os.Getenv("TELEGRAM_USER_ID"), "Allowed user ID (required)")
		debug      = flag.String("debug", os.Getenv("TELEGRAM_DEBUG"), "Debug mode? (optional - true or false/leave empty for off)")
	)
	flag.Parse()

	// Validate required parameters
	if *botToken == "" {
		log.Fatal("Bot token is required. Use -token flag or environment variable TELEGRAM_BOT_TOKEN")
	}
	if *folder == "" {
		log.Fatal("Download folder path is required. Use -folder flag or environment variable TELEGRAM_FOLDER")
	}
	if *allowedUID == "" {
		log.Fatal("Allowed user ID is required. Use -user flag or environment variable TELEGRAM_USER_ID")
	}
	if *debug == "" {
		log.Printf("Debug mode off")
		*debug = "false"
	} else {
		log.Printf("Debug mode on")
	}

	// Create download folder if it doesn't exist
	if err := os.MkdirAll(*folder, 0755); err != nil {
		log.Fatalf("Failed to create download folder: %v", err)
	}

	// Initialize bot
	bot, err := tgbotapi.NewBotAPI(*botToken)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	bot.Debug, _ = strconv.ParseBool(*debug)
	log.Printf("Authorized on account %s", bot.Self.UserName)
	log.Printf("Monitoring direct chats for documents")
	log.Printf("Download folder: %s", *folder)
	log.Printf("Allowed user ID: %s", *allowedUID)

	// Convert allowed user ID to int64
	allowedUserID, err := strconv.ParseInt(*allowedUID, 10, 64)
	if err != nil {
		log.Fatalf("Invalid user ID format: %v", err)
	}

	// Send initial greeting message to the allowed user
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	initialMsg := tgbotapi.NewMessage(allowedUserID, fmt.Sprintf("[%s] Hi, show me the docs!", timestamp))
	if _, err := bot.Send(initialMsg); err != nil {
		log.Printf("Error sending initial message: %v", err)
	} else {
		log.Printf("Sent initial greeting to user %d", allowedUserID)
	}

	// Configure updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Process updates
	for update := range updates {
		if update.Message == nil {
			continue
		}

		message := update.Message

		// Only handle private chats (direct messages)
		if !message.Chat.IsPrivate() {
			continue
		}

		// Check if message is from allowed user
		if message.From.ID != allowedUserID {
			log.Printf("Ignoring message from unauthorized user: %s (ID: %d)",
				message.From.UserName, message.From.ID)
			continue
		}

		// Handle document messages only
		if err := handleDocumentMessage(bot, message, *folder); err != nil {
			log.Printf("Error handling document: %v", err)
		}
	}
}

// handleDocumentMessage processes messages that contain documents only
func handleDocumentMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, downloadFolder string) error {
	// Only handle document messages
	if message.Document == nil {
		return nil
	}

	fileID := message.Document.FileID
	fileName := message.Document.FileName
	fileSize := int64(message.Document.FileSize)

	log.Printf("Found document from user %s: %s (size: %d bytes)",
		message.From.UserName, fileName, fileSize)

	// Send initial download message
	statusMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("📥 Downloading: %s\n⏳ Starting download...", fileName))
	sentMsg, err := bot.Send(statusMsg)
	if err != nil {
		log.Printf("Error sending status message: %v", err)
		// Continue with download even if status message fails
	}

	// Download the document with progress updates
	err = downloadFileWithProgress(bot, fileID, fileName, downloadFolder, fileSize, message.Chat.ID, sentMsg.MessageID)

	return err
}

// downloadFileWithProgress downloads a file from Telegram servers with progress updates
func downloadFileWithProgress(bot *tgbotapi.BotAPI, fileID, fileName, downloadFolder string, fileSize int64, chatID int64, messageID int) error {
	// Get file info from Telegram
	fileConfig := tgbotapi.FileConfig{FileID: fileID}
	file, err := bot.GetFile(fileConfig)
	if err != nil {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Error getting file info: %s", fileName))
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create file URL
	fileURL := file.Link(bot.Token)

	// Create full file path
	if fileName == "" {
		fileName = fmt.Sprintf("file_%s", file.FileID)
	}

	// Sanitize filename
	fileName = sanitizeFilename(fileName)
	filePath := filepath.Join(downloadFolder, fileName)

	// Handle duplicate filenames
	filePath = getUniqueFilePath(filePath)
	finalFileName := filepath.Base(filePath)

	// Update status: starting download
	updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("📥 Downloading: %s\n🔄 Connecting...", finalFileName))

	// Download file
	log.Printf("Downloading file: %s", finalFileName)
	resp, err := http.Get(fileURL)
	if err != nil {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Download failed: %s", finalFileName))
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Create local file
	outFile, err := os.Create(filePath)
	if err != nil {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Error creating file: %s", finalFileName))
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer outFile.Close()

	// Create progress reader
	progressReader := &ProgressReader{
		Reader:     resp.Body,
		Total:      fileSize,
		bot:        bot,
		chatID:     chatID,
		messageID:  messageID,
		fileName:   finalFileName,
		lastUpdate: time.Now(),
	}

	// Copy file content with progress
	bytesWritten, err := io.Copy(outFile, progressReader)
	if err != nil {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Download failed: %s", finalFileName))
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Update final status
	updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("✅ Downloaded: %s\n📊 Size: %s", finalFileName, formatBytes(bytesWritten)))

	log.Printf("Successfully downloaded: %s (%d bytes)", filePath, bytesWritten)
	return nil
}

// ProgressReader wraps an io.Reader to provide progress updates
type ProgressReader struct {
	Reader     io.Reader
	Total      int64
	Current    int64
	bot        *tgbotapi.BotAPI
	chatID     int64
	messageID  int
	fileName   string
	lastUpdate time.Time
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.Current += int64(n)

	// Update progress every 2 seconds or when download completes
	now := time.Now()
	if now.Sub(pr.lastUpdate) > 2*time.Second || err == io.EOF {
		pr.updateProgress()
		pr.lastUpdate = now
	}

	return n, err
}

func (pr *ProgressReader) updateProgress() {
	if pr.Total <= 0 {
		return
	}

	percentage := float64(pr.Current) / float64(pr.Total) * 100
	progressBar := createProgressBar(percentage)

	status := fmt.Sprintf("📥 Downloading: %s\n%s %.1f%%\n📊 %s / %s",
		pr.fileName,
		progressBar,
		percentage,
		formatBytes(pr.Current),
		formatBytes(pr.Total))

	updateStatusMessage(pr.bot, pr.chatID, pr.messageID, status)
}

// createProgressBar creates a visual progress bar
func createProgressBar(percentage float64) string {
	barLength := 20
	filledLength := int(percentage / 100 * float64(barLength))

	bar := strings.Repeat("█", filledLength) + strings.Repeat("░", barLength-filledLength)
	return fmt.Sprintf("[%s]", bar)
}

// updateStatusMessage updates the progress message
func updateStatusMessage(bot *tgbotapi.BotAPI, chatID int64, messageID int, text string) {
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if _, err := bot.Send(editMsg); err != nil {
		log.Printf("Error updating status message: %v", err)
	}
}

// formatBytes formats bytes into human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// downloadFile downloads a file from Telegram servers (legacy function - kept for compatibility)
func downloadFile(bot *tgbotapi.BotAPI, fileID, fileName, downloadFolder string, fileSize int64) error {
	return downloadFileWithProgress(bot, fileID, fileName, downloadFolder, fileSize, 0, 0)
}

// sanitizeFilename removes or replaces invalid characters in filename
func sanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	// Remove leading/trailing spaces and dots
	filename = strings.Trim(filename, " .")

	// Ensure filename is not empty
	if filename == "" {
		filename = "unnamed_file"
	}

	return filename
}

// getUniqueFilePath ensures the file path is unique by adding a number suffix if needed
func getUniqueFilePath(filePath string) string {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return filePath
	}

	dir := filepath.Dir(filePath)
	ext := filepath.Ext(filePath)
	name := strings.TrimSuffix(filepath.Base(filePath), ext)

	for i := 1; ; i++ {
		newPath := filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, i, ext))
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}
