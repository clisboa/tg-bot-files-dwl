package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"golang.org/x/time/rate"
)

const (
	// Client API supports files up to 2GB
	MaxFileSize = 2 * 1024 * 1024 * 1024 // 2GB in bytes
)

type Config struct {
	APIID          int
	APIHash        string
	Phone          string
	DownloadFolder string
	ChannelID      int64
	AllowedUserID  int64
	Debug          bool
	AllowedTypes   []string
	SessionFile    string
	CodeFile       string
	PasswordFile   string
}

func main() {
	// Parse command line arguments
	var (
		apiID        = flag.Int("api-id", 0, "Telegram API ID from https://my.telegram.org")
		apiHash      = flag.String("api-hash", os.Getenv("TELEGRAM_API_HASH"), "Telegram API Hash from https://my.telegram.org")
		phone        = flag.String("phone", os.Getenv("TELEGRAM_PHONE"), "Phone number (with country code, e.g., +1234567890)")
		folder       = flag.String("folder", os.Getenv("TELEGRAM_FOLDER"), "Download folder path")
		channelID    = flag.String("channel", os.Getenv("TELEGRAM_CHANNEL_ID"), "Channel/Group ID where bot monitors (optional, use instead of private chat)")
		allowedUID   = flag.String("user", os.Getenv("TELEGRAM_USER_ID"), "Allowed user ID (required)")
		debug        = flag.String("debug", os.Getenv("TELEGRAM_DEBUG"), "Debug mode? (optional - true or false/leave empty for off)")
		allowedTypes = flag.String("types", os.Getenv("TELEGRAM_ALLOWED_TYPES"), "Comma-separated list of allowed file extensions (e.g., pdf,txt,docx). Leave empty to allow all types")
		sessionFile  = flag.String("session", "session.json", "Session file path for storing authentication")
		codeFile     = flag.String("code-file", getEnvOrDefault("TELEGRAM_CODE_FILE", "telegram_code.txt"), "File to read verification code from (will wait for file creation)")
		passwordFile = flag.String("password-file", getEnvOrDefault("TELEGRAM_PASSWORD_FILE", "telegram_password.txt"), "File to read 2FA password from (optional)")
	)
	flag.Parse()

	// Get API ID from environment if not set via flag
	if *apiID == 0 {
		if envID := os.Getenv("TELEGRAM_API_ID"); envID != "" {
			var err error
			*apiID, err = strconv.Atoi(envID)
			if err != nil {
				log.Fatalf("Invalid API ID: %v", err)
			}
		}
	}

	// Validate required parameters
	if *apiID == 0 {
		log.Fatal("API ID is required. Get it from https://my.telegram.org and use -api-id flag or TELEGRAM_API_ID environment variable")
	}
	if *apiHash == "" {
		log.Fatal("API Hash is required. Get it from https://my.telegram.org and use -api-hash flag or TELEGRAM_API_HASH environment variable")
	}
	if *phone == "" {
		log.Fatal("Phone number is required. Use -phone flag or TELEGRAM_PHONE environment variable")
	}
	if *folder == "" {
		log.Fatal("Download folder path is required. Use -folder flag or TELEGRAM_FOLDER environment variable")
	}
	if *allowedUID == "" {
		log.Fatal("Allowed user ID is required. Use -user flag or TELEGRAM_USER_ID environment variable")
	}

	debugMode := false
	if *debug != "" {
		debugMode, _ = strconv.ParseBool(*debug)
	}

	// Parse and validate allowed file types
	var allowedExtensions []string
	if *allowedTypes != "" {
		extensions := strings.SplitSeq(*allowedTypes, ",")
		for ext := range extensions {
			ext = strings.TrimSpace(ext)
			if ext != "" {
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

	// Convert allowed user ID to int64
	allowedUserID, err := strconv.ParseInt(*allowedUID, 10, 64)
	if err != nil {
		log.Fatalf("Invalid user ID format: %v", err)
	}

	// Parse channel ID if provided
	var parsedChannelID int64
	if *channelID != "" {
		parsedChannelID, err = strconv.ParseInt(*channelID, 10, 64)
		if err != nil {
			log.Fatalf("Invalid channel ID format: %v", err)
		}
	}

	config := &Config{
		APIID:          *apiID,
		APIHash:        *apiHash,
		Phone:          *phone,
		DownloadFolder: *folder,
		ChannelID:      parsedChannelID,
		AllowedUserID:  allowedUserID,
		Debug:          debugMode,
		AllowedTypes:   allowedExtensions,
		SessionFile:    *sessionFile,
		CodeFile:       *codeFile,
		PasswordFile:   *passwordFile,
	}

	log.Printf("Download folder: %s", config.DownloadFolder)
	if config.ChannelID != 0 {
		log.Printf("Monitoring channel/group ID: %d", config.ChannelID)
	} else {
		log.Printf("Monitoring private messages")
	}
	log.Printf("Allowed user ID: %d", config.AllowedUserID)
	log.Printf("Session file: %s", config.SessionFile)
	log.Printf("File size limit: %s (Client API)", formatBytes(MaxFileSize))

	// Run the bot
	if err := runBot(context.Background(), config); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}

func runBot(ctx context.Context, config *Config) error {
	// Create client with session storage
	client := telegram.NewClient(config.APIID, config.APIHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{
			Path: config.SessionFile,
		},
		Middlewares: []telegram.Middleware{
			floodwait.NewSimpleWaiter().WithMaxRetries(3),
			ratelimit.New(rate.Every(time.Millisecond*100), 5),
		},
	})

	// Authentication flow
	flow := auth.NewFlow(
		fileAuth{
			phone:        config.Phone,
			codeFile:     config.CodeFile,
			passwordFile: config.PasswordFile,
		},
		auth.SendCodeOptions{},
	)

	return client.Run(ctx, func(ctx context.Context) error {
		// Authenticate
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		log.Println("Authentication successful!")

		// Get current user info
		user, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}

		log.Printf("Logged in as: %s %s (ID: %d)", user.FirstName, user.LastName, user.ID)

		// Send greeting message to allowed user
		if err := sendGreeting(ctx, client, config); err != nil {
			log.Printf("Error sending greeting: %v", err)
		}

		// Set up message handler
		dispatcher := tg.NewUpdateDispatcher()
		gaps := updates.New(updates.Config{
			Handler: dispatcher,
		})

		// Register message handler
		dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
			return handleMessage(ctx, client, e, update, config)
		})

		// Start handling updates
		log.Println("Bot is running... Monitoring for documents")
		return gaps.Run(ctx, client.API(), user.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				log.Println("Update gap handler started")
			},
		})
	})
}

func sendGreeting(ctx context.Context, client *telegram.Client, config *Config) error {
	sender := message.NewSender(client.API())

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	greetingMsg := fmt.Sprintf("[%s] Hi, show me the docs!\n\n📋 File size limit: %s", timestamp, formatBytes(MaxFileSize))

	if len(config.AllowedTypes) > 0 {
		greetingMsg += fmt.Sprintf("\n📎 Allowed types: %s", strings.Join(config.AllowedTypes, ", "))
	} else {
		greetingMsg += "\n📎 All file types accepted"
	}

	greetingMsg += "\n💡 Using Client API - supports files up to 2GB!"

	// If channel mode, send to channel
	if config.ChannelID != 0 {
		target := &tg.InputPeerChannel{
			ChannelID:  config.ChannelID,
			AccessHash: 0, // Will be resolved
		}

		_, err := sender.To(target).Text(ctx, greetingMsg)
		if err != nil {
			log.Printf("Could not send greeting to channel: %v", err)
			log.Printf("💡 Make sure:")
			log.Printf("   1. Bot account is a member of the channel/group")
			log.Printf("   2. Channel ID is correct (use negative ID for supergroups)")
			return nil
		}

		log.Printf("✅ Sent greeting to channel %d", config.ChannelID)
		return nil
	}

	// For private messages, try to get user from contacts
	contacts, err := client.API().ContactsGetContacts(ctx, 0)
	if err != nil {
		log.Printf("Greeting skipped: could not fetch contacts")
		log.Printf("💡 Use channel mode (-channel flag) for reliable greeting, or:")
		log.Printf("   1. Add user %d to bot account's contacts, OR", config.AllowedUserID)
		log.Printf("   2. Send any message from user to bot first")
		return nil
	}

	var accessHash int64
	var found bool

	if savedContacts, ok := contacts.(*tg.ContactsContacts); ok {
		for _, userClass := range savedContacts.Users {
			if user, ok := userClass.(*tg.User); ok && user.ID == config.AllowedUserID {
				accessHash = user.AccessHash
				found = true
				break
			}
		}
	}

	if !found {
		log.Printf("Greeting skipped: user %d not in contacts", config.AllowedUserID)
		log.Printf("💡 Use channel mode (-channel flag) for reliable greeting, or:")
		log.Printf("   1. Add user %d to bot account's contacts, OR", config.AllowedUserID)
		log.Printf("   2. Send any message from user to bot first")
		return nil
	}

	target := &tg.InputPeerUser{
		UserID:     config.AllowedUserID,
		AccessHash: accessHash,
	}

	_, err = sender.To(target).Text(ctx, greetingMsg)
	if err != nil {
		log.Printf("Could not send greeting: %v", err)
		return nil
	}

	log.Printf("✅ Sent greeting to user %d", config.AllowedUserID)
	return nil
}

func handleMessage(ctx context.Context, client *telegram.Client, entities tg.Entities, update *tg.UpdateNewMessage, config *Config) error {
	msg, ok := update.Message.(*tg.Message)
	if !ok {
		return nil
	}

	// Determine the peer and reply target
	var peer tg.InputPeerClass
	var senderUserID int64

	// Handle channel/group messages
	if config.ChannelID != 0 {
		// Check if message is from the configured channel
		switch p := msg.PeerID.(type) {
		case *tg.PeerChannel:
			if p.ChannelID != config.ChannelID {
				return nil // Not from our channel
			}

			// For channels, we can use the channel ID from config
			// The access hash will be resolved by the sender
			peer = &tg.InputPeerChannel{
				ChannelID:  p.ChannelID,
				AccessHash: 0, // Will be resolved by library
			}

			// Get sender user ID from message
			if msg.FromID != nil {
				if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
					senderUserID = fromUser.UserID
				}
			}
		default:
			return nil // Not a channel message
		}
	} else {
		// Handle private messages
		peerUser, ok := msg.PeerID.(*tg.PeerUser)
		if !ok {
			return nil
		}

		senderUserID = peerUser.UserID

		// Get the user from entities to construct proper input peer
		var accessHash int64
		for _, u := range entities.Users {
			if u.ID == peerUser.UserID {
				accessHash = u.AccessHash
				break
			}
		}

		peer = &tg.InputPeerUser{
			UserID:     peerUser.UserID,
			AccessHash: accessHash,
		}
	}

	// Check if message is from allowed user
	if senderUserID != config.AllowedUserID {
		log.Printf("Ignoring message from unauthorized user ID: %d", senderUserID)
		return nil
	}

	// Handle document messages only
	media, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil
	}

	doc, ok := media.Document.(*tg.Document)
	if !ok {
		return nil
	}

	// Get document attributes
	var fileName string
	var fileSize int64 = doc.Size

	for _, attr := range doc.Attributes {
		if nameAttr, ok := attr.(*tg.DocumentAttributeFilename); ok {
			fileName = nameAttr.FileName
			break
		}
	}

	if fileName == "" {
		fileName = fmt.Sprintf("document_%d", doc.ID)
	}

	log.Printf("Found document from user %d: %s (size: %d bytes)", senderUserID, fileName, fileSize)

	// Check file type if restrictions are enabled
	if len(config.AllowedTypes) > 0 {
		if !isAllowedFileType(fileName, config.AllowedTypes) {
			fileExt := strings.ToLower(filepath.Ext(fileName))
			if fileExt != "" && strings.HasPrefix(fileExt, ".") {
				fileExt = fileExt[1:]
			}

			errorMsg := fmt.Sprintf("❌ File type not allowed: %s\n📎 Extension: %s\n✅ Allowed types: %s\n\n💡 Please convert your file to an allowed format or contact the administrator to add this file type.",
				fileName,
				fileExt,
				strings.Join(config.AllowedTypes, ", "))

			sender := message.NewSender(client.API())
			_, err := sender.To(peer).Text(ctx, errorMsg)
			if err != nil {
				log.Printf("Error sending file type error message: %v", err)
			}

			log.Printf("File %s rejected: extension '%s' not in allowed list %v", fileName, fileExt, config.AllowedTypes)
			return fmt.Errorf("file extension '%s' not allowed", fileExt)
		}
	}

	// Check file size limit (2GB for Client API)
	if fileSize > MaxFileSize {
		errorMsg := fmt.Sprintf("❌ File too large: %s\n📊 Size: %s\n🚫 Maximum limit: %s\n\n💡 Even with Client API, files larger than 2GB are not supported by Telegram.",
			fileName, formatBytes(fileSize), formatBytes(MaxFileSize))

		sender := message.NewSender(client.API())
		_, err := sender.To(peer).Text(ctx, errorMsg)
		if err != nil {
			log.Printf("Error sending file size error message: %v", err)
		}

		log.Printf("File %s rejected: size %d bytes exceeds %d bytes limit", fileName, fileSize, MaxFileSize)
		return fmt.Errorf("file size %d bytes exceeds maximum limit of %d bytes", fileSize, MaxFileSize)
	}

	// Send initial download message
	sender := message.NewSender(client.API())
	statusMsg := fmt.Sprintf("📥 Downloading: %s\n📊 Size: %s\n⏳ Starting download...", fileName, formatBytes(fileSize))

	upd, err := sender.To(peer).Text(ctx, statusMsg)
	var messageID int
	if err != nil {
		log.Printf("Error sending status message: %v", err)
		messageID = 0
	} else {
		// Extract message ID from the updates
		switch u := upd.(type) {
		case *tg.Updates:
			if len(u.Updates) > 0 {
				if msgUpdate, ok := u.Updates[0].(*tg.UpdateMessageID); ok {
					messageID = msgUpdate.ID
				}
			}
		}
	}

	// Download the document with progress updates
	err = downloadDocument(ctx, client, doc, fileName, config.DownloadFolder, fileSize, peer, messageID)
	return err
}

func downloadDocument(ctx context.Context, client *telegram.Client, doc *tg.Document, fileName, downloadFolder string, fileSize int64, peer tg.InputPeerClass, messageID int) error {
	// Sanitize filename
	fileName = sanitizeFilename(fileName)
	filePath := filepath.Join(downloadFolder, fileName)

	// Handle duplicate filenames
	filePath = getUniqueFilePath(filePath)
	finalFileName := filepath.Base(filePath)

	// Update status: starting download
	updateStatusMessage(ctx, client, peer, messageID, fmt.Sprintf("📥 Downloading: %s\n📊 Size: %s\n🔄 Connecting...", finalFileName, formatBytes(fileSize)))

	log.Printf("Downloading file: %s", finalFileName)

	// Create local file
	outFile, err := os.Create(filePath)
	if err != nil {
		updateStatusMessage(ctx, client, peer, messageID, fmt.Sprintf("❌ Error creating file: %s\n💾 Check disk space and permissions", finalFileName))
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer outFile.Close()

	// Create downloader
	d := downloader.NewDownloader()

	// Create progress tracker
	progress := &ProgressTracker{
		Total:      fileSize,
		client:     client,
		peer:       peer,
		messageID:  messageID,
		fileName:   finalFileName,
		lastUpdate: time.Now(),
		startTime:  time.Now(),
	}

	// Create file location
	location := &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}

	// Download with progress tracking
	_, err = d.Download(client.API(), location).
		Stream(ctx, &progressWriter{
			writer:   outFile,
			progress: progress,
		})

	if err != nil {
		updateStatusMessage(ctx, client, peer, messageID, fmt.Sprintf("❌ Download failed: %s\n🌐 Network error occurred", finalFileName))
		return fmt.Errorf("failed to download file: %w", err)
	}

	// Update final status
	duration := time.Since(progress.startTime)
	avgSpeed := formatBytes(progress.Current) + "/s"
	if duration.Seconds() > 0 {
		avgSpeed = formatBytes(int64(float64(progress.Current)/duration.Seconds())) + "/s"
	}

	updateStatusMessage(ctx, client, peer, messageID, fmt.Sprintf("✅ Downloaded: %s\n📊 Size: %s\n⚡ Avg Speed: %s\n📁 Saved to: %s",
		finalFileName, formatBytes(progress.Current), avgSpeed, downloadFolder))

	log.Printf("Successfully downloaded: %s (%d bytes)", filePath, progress.Current)
	return nil
}

// ProgressTracker tracks download progress
type ProgressTracker struct {
	Total      int64
	Current    int64
	client     *telegram.Client
	peer       tg.InputPeerClass
	messageID  int
	fileName   string
	lastUpdate time.Time
	startTime  time.Time
}

type progressWriter struct {
	writer   io.Writer
	progress *ProgressTracker
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	pw.progress.Current += int64(n)

	// Update progress every 2 seconds
	now := time.Now()
	if now.Sub(pw.progress.lastUpdate) > 2*time.Second {
		pw.progress.updateProgress()
		pw.progress.lastUpdate = now
	}

	return n, err
}

func (pt *ProgressTracker) updateProgress() {
	ctx := context.Background()

	if pt.Total <= 0 {
		status := fmt.Sprintf("📥 Downloading: %s\n🔄 Progress: %s downloaded\n⏱️ In progress...",
			pt.fileName,
			formatBytes(pt.Current))
		updateStatusMessage(ctx, pt.client, pt.peer, pt.messageID, status)
		return
	}

	percentage := float64(pt.Current) / float64(pt.Total) * 100
	progressBar := createProgressBar(percentage)

	// Calculate estimated time remaining
	elapsed := time.Since(pt.startTime)
	var eta string
	if pt.Current > 0 && elapsed.Seconds() > 0 {
		bytesPerSecond := float64(pt.Current) / elapsed.Seconds()
		remainingBytes := pt.Total - pt.Current
		if bytesPerSecond > 0 {
			etaSeconds := float64(remainingBytes) / bytesPerSecond
			eta = fmt.Sprintf(" • ETA: %s", formatDuration(time.Duration(etaSeconds)*time.Second))
		}
	}

	status := fmt.Sprintf("📥 Downloading: %s\n%s %.1f%%\n📊 %s / %s%s",
		pt.fileName,
		progressBar,
		percentage,
		formatBytes(pt.Current),
		formatBytes(pt.Total),
		eta)

	updateStatusMessage(ctx, pt.client, pt.peer, pt.messageID, status)
}

func updateStatusMessage(ctx context.Context, client *telegram.Client, peer tg.InputPeerClass, messageID int, text string) {
	sender := message.NewSender(client.API())
	_, err := sender.To(peer).Edit(messageID).Text(ctx, text)
	if err != nil {
		log.Printf("Error updating status message: %v", err)
	}
}

// fileAuth implements auth.UserAuthenticator for file-based authentication
type fileAuth struct {
	phone        string
	codeFile     string
	passwordFile string
}

func (a fileAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a fileAuth) Password(ctx context.Context) (string, error) {
	log.Printf("2FA password required. Waiting for password in file: %s", a.passwordFile)
	log.Printf("Please create the file and write your 2FA password to it")

	password, err := waitForFileContent(ctx, a.passwordFile, 5*time.Minute)
	if err != nil {
		return "", err
	}

	// Delete the password file for security
	os.Remove(a.passwordFile)
	log.Printf("Password file deleted for security")

	return password, nil
}

func (a fileAuth) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	log.Printf("===========================================")
	log.Printf("VERIFICATION CODE REQUIRED")
	log.Printf("===========================================")
	log.Printf("A verification code has been sent to your Telegram app")
	log.Printf("Please create the file: %s", a.codeFile)
	log.Printf("Write the verification code to this file")
	log.Printf("Waiting for code file (timeout: 5 minutes)...")
	log.Printf("===========================================")

	code, err := waitForFileContent(ctx, a.codeFile, 5*time.Minute)
	if err != nil {
		return "", err
	}

	// Delete the code file after reading
	os.Remove(a.codeFile)
	log.Printf("Verification code received and file deleted")

	return code, nil
}

func (a fileAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (a fileAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("signup not supported")
}

// waitForFileContent waits for a file to be created and reads its content
func waitForFileContent(ctx context.Context, filePath string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for file: %s", filePath)
		case <-ticker.C:
			if _, err := os.Stat(filePath); err == nil {
				// File exists, read it
				content, err := os.ReadFile(filePath)
				if err != nil {
					return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
				}

				// Trim whitespace and return
				result := strings.TrimSpace(string(content))
				if result == "" {
					log.Printf("File %s is empty, waiting for content...", filePath)
					continue
				}

				return result, nil
			}
		}
	}
}

// getEnvOrDefault gets environment variable or returns default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Helper functions

func createProgressBar(percentage float64) string {
	barLength := 20
	filledLength := int(percentage / 100 * float64(barLength))

	bar := strings.Repeat("█", filledLength) + strings.Repeat("░", barLength-filledLength)
	return fmt.Sprintf("[%s]", bar)
}

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

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*float64(int(d.Minutes())))
	}
	return fmt.Sprintf("%.0fh %.0fm", d.Hours(), d.Minutes()-60*float64(int(d.Hours())))
}

func isAllowedFileType(filename string, allowedExtensions []string) bool {
	if len(allowedExtensions) == 0 {
		return true
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" && strings.HasPrefix(ext, ".") {
		ext = ext[1:]
	}

	return slices.Contains(allowedExtensions, ext)
}

func sanitizeFilename(filename string) string {
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	filename = strings.Trim(filename, " .")

	if filename == "" {
		filename = "unnamed_file"
	}

	return filename
}

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
