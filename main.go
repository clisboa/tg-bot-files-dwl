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

const (
	// Telegram Bot API file size limit
	TelegramMaxFileSize = 20 * 1024 * 1024 // 20MB in bytes
)

func main() {
	// Parse command line arguments
	var (
		botToken     = flag.String("token", os.Getenv("TELEGRAM_BOT_TOKEN"), "Telegram bot token")
		folder       = flag.String("folder", os.Getenv("TELEGRAM_FOLDER"), "Download folder path")
		allowedUID   = flag.String("user", os.Getenv("TELEGRAM_USER_ID"), "Allowed user ID (required)")
		debug        = flag.String("debug", os.Getenv("TELEGRAM_DEBUG"), "Debug mode? (optional - true or false/leave empty for off)")
		allowedTypes = flag.String("types", os.Getenv("TELEGRAM_ALLOWED_TYPES"), "Comma-separated list of allowed file extensions (e.g., pdf,txt,docx). Leave empty to allow all types")
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

	// Parse and validate allowed file types
	var allowedExtensions []string
	if *allowedTypes != "" {
		extensions := strings.Split(*allowedTypes, ",")
		for _, ext := range extensions {
			ext = strings.TrimSpace(ext)
			if ext != "" {
				// Normalize extension (remove leading dot, convert to lowercase)
				ext = strings.TrimPrefix(ext, ".")
				allowedExtensions = append(allowedExtensions, strings.ToLower(ext))
			}
		}
		log.Printf("Allowed file types: %v", allowedExtensions)
	} else {
		log.Printf("All file types allowed")
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
	greetingMsg := fmt.Sprintf("[%s] Hi, show me the docs!\n\n📋 File size limit: %s", timestamp, formatBytes(TelegramMaxFileSize))

	if len(allowedExtensions) > 0 {
		greetingMsg += fmt.Sprintf("\n📎 Allowed types: %s", strings.Join(allowedExtensions, ", "))
	} else {
		greetingMsg += "\n📎 All file types accepted"
	}

	greetingMsg += "\n💡 For larger files, consider using cloud storage services."

	initialMsg := tgbotapi.NewMessage(allowedUserID, greetingMsg)
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
		if err := handleDocumentMessage(bot, message, *folder, allowedExtensions); err != nil {
			log.Printf("Error handling document: %v", err)
		}
	}
}

// handleDocumentMessage processes messages that contain documents only
func handleDocumentMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, downloadFolder string, allowedExtensions []string) error {
	// Only handle document messages
	if message.Document == nil {
		return nil
	}

	fileID := message.Document.FileID
	fileName := message.Document.FileName
	fileSize := int64(message.Document.FileSize)

	log.Printf("Found document from user %s: %s (size: %d bytes)",
		message.From.UserName, fileName, fileSize)

	// Check file type if restrictions are enabled
	if len(allowedExtensions) > 0 {
		if !isAllowedFileType(fileName, allowedExtensions) {
			fileExt := strings.ToLower(filepath.Ext(fileName))
			if fileExt != "" && strings.HasPrefix(fileExt, ".") {
				fileExt = fileExt[1:] // Remove the dot
			}

			errorMsg := fmt.Sprintf("❌ File type not allowed: %s\n📎 Extension: %s\n✅ Allowed types: %s\n\n💡 Please convert your file to an allowed format or contact the administrator to add this file type.",
				fileName,
				fileExt,
				strings.Join(allowedExtensions, ", "))

			statusMsg := tgbotapi.NewMessage(message.Chat.ID, errorMsg)
			if _, err := bot.Send(statusMsg); err != nil {
				log.Printf("Error sending file type error message: %v", err)
			}

			log.Printf("File %s rejected: extension '%s' not in allowed list %v", fileName, fileExt, allowedExtensions)
			return fmt.Errorf("file extension '%s' not allowed", fileExt)
		}
	}

	// Check file size limit
	if fileSize > TelegramMaxFileSize {
		errorMsg := fmt.Sprintf("❌ File too large: %s\n📊 Size: %s\n🚫 Telegram limit: %s\n\n💡 Suggestions:\n• Use cloud storage (Google Drive, Dropbox, etc.)\n• Compress the file\n• Split into smaller parts\n• Send a download link instead",
			fileName, formatBytes(fileSize), formatBytes(TelegramMaxFileSize))

		statusMsg := tgbotapi.NewMessage(message.Chat.ID, errorMsg)
		if _, err := bot.Send(statusMsg); err != nil {
			log.Printf("Error sending file size error message: %v", err)
		}

		log.Printf("File %s rejected: size %d bytes exceeds %d bytes limit", fileName, fileSize, TelegramMaxFileSize)
		return fmt.Errorf("file size %d bytes exceeds Telegram limit of %d bytes", fileSize, TelegramMaxFileSize)
	}

	// Send initial download message
	statusMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("📥 Downloading: %s\n📊 Size: %s\n⏳ Starting download...", fileName, formatBytes(fileSize)))
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
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Error getting file info: %s\n🔍 This might be due to file size restrictions", fileName))
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Double-check file size from API response
	if file.FileSize > TelegramMaxFileSize {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ File too large: %s\n📊 Size: %s\n🚫 Telegram API limit: %s", fileName, formatBytes(int64(file.FileSize)), formatBytes(TelegramMaxFileSize)))
		return fmt.Errorf("file size %d bytes exceeds Telegram API limit", file.FileSize)
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
	updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("📥 Downloading: %s\n📊 Size: %s\n🔄 Connecting...", finalFileName, formatBytes(fileSize)))

	// Download file
	log.Printf("Downloading file: %s", finalFileName)
	resp, err := http.Get(fileURL)
	if err != nil {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Download failed: %s\n🌐 Network error occurred", finalFileName))
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Download failed: %s\n📡 Server returned: %s", finalFileName, resp.Status))
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Create local file
	outFile, err := os.Create(filePath)
	if err != nil {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Error creating file: %s\n💾 Check disk space and permissions", finalFileName))
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer outFile.Close()

	// Use actual file size from response if available
	actualFileSize := fileSize
	if resp.ContentLength > 0 {
		actualFileSize = resp.ContentLength
	}

	// Create progress reader
	progressReader := &ProgressReader{
		Reader:     resp.Body,
		Total:      actualFileSize,
		bot:        bot,
		chatID:     chatID,
		messageID:  messageID,
		fileName:   finalFileName,
		lastUpdate: time.Now(),
	}

	// Copy file content with progress
	bytesWritten, err := io.Copy(outFile, progressReader)
	if err != nil {
		updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("❌ Download failed: %s\n💾 Error writing to disk", finalFileName))
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Update final status
	duration := time.Since(progressReader.lastUpdate)
	avgSpeed := formatBytes(bytesWritten) + "/s"
	if duration.Seconds() > 0 {
		avgSpeed = formatBytes(int64(float64(bytesWritten)/duration.Seconds())) + "/s"
	}

	updateStatusMessage(bot, chatID, messageID, fmt.Sprintf("✅ Downloaded: %s\n📊 Size: %s\n⚡ Avg Speed: %s\n📁 Saved to: %s",
		finalFileName, formatBytes(bytesWritten), avgSpeed, downloadFolder))

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
		// Handle unknown file size
		status := fmt.Sprintf("📥 Downloading: %s\n🔄 Progress: %s downloaded\n⏱️ In progress...",
			pr.fileName,
			formatBytes(pr.Current))
		updateStatusMessage(pr.bot, pr.chatID, pr.messageID, status)
		return
	}

	percentage := float64(pr.Current) / float64(pr.Total) * 100
	progressBar := createProgressBar(percentage)

	// Calculate estimated time remaining
	elapsed := time.Since(pr.lastUpdate)
	var eta string
	if pr.Current > 0 && elapsed.Seconds() > 0 {
		bytesPerSecond := float64(pr.Current) / elapsed.Seconds()
		remainingBytes := pr.Total - pr.Current
		if bytesPerSecond > 0 {
			etaSeconds := float64(remainingBytes) / bytesPerSecond
			eta = fmt.Sprintf(" • ETA: %s", formatDuration(time.Duration(etaSeconds)*time.Second))
		}
	}

	status := fmt.Sprintf("📥 Downloading: %s\n%s %.1f%%\n📊 %s / %s%s",
		pr.fileName,
		progressBar,
		percentage,
		formatBytes(pr.Current),
		formatBytes(pr.Total),
		eta)

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

// formatDuration formats a duration into human readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*float64(int(d.Minutes())))
	}
	return fmt.Sprintf("%.0fh %.0fm", d.Hours(), d.Minutes()-60*float64(int(d.Hours())))
}

// downloadFile downloads a file from Telegram servers (legacy function - kept for compatibility)
func downloadFile(bot *tgbotapi.BotAPI, fileID, fileName, downloadFolder string, fileSize int64) error {
	return downloadFileWithProgress(bot, fileID, fileName, downloadFolder, fileSize, 0, 0)
}

// isAllowedFileType checks if a filename has an allowed extension
func isAllowedFileType(filename string, allowedExtensions []string) bool {
	if len(allowedExtensions) == 0 {
		return true // No restrictions
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" && strings.HasPrefix(ext, ".") {
		ext = ext[1:] // Remove the dot
	}

	for _, allowedExt := range allowedExtensions {
		if ext == allowedExt {
			return true
		}
	}

	return false
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
