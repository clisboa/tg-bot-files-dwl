# Migration Guide: Bot API → Client API (MTProto)

This guide explains the migration from Telegram Bot API to Client API (MTProto) to support larger file downloads.

## What Changed?

### File Size Limits
- **Before (Bot API)**: 20 MB maximum
- **After (Client API)**: 2 GB maximum (100x increase!)

### Authentication Method
- **Before**: Bot token from @BotFather
- **After**: User account authentication (phone number + verification code)

### Library
- **Before**: `github.com/go-telegram-bot-api/telegram-bot-api/v5`
- **After**: `github.com/gotd/td` (MTProto implementation)

## Setup Instructions

### 1. Get API Credentials

You need to obtain API credentials from Telegram:

1. Visit https://my.telegram.org
2. Log in with your phone number
3. Go to "API development tools"
4. Create a new application (if you don't have one)
5. Note down:
   - **API ID** (numeric)
   - **API Hash** (alphanumeric string)

### 2. Install Dependencies

```bash
go mod tidy
```

This will download the new `gotd/td` library and its dependencies.

### 3. Configuration

#### Required Parameters

| Parameter | Flag | Environment Variable | Description |
|-----------|------|---------------------|-------------|
| API ID | `-api-id` | `TELEGRAM_API_ID` | API ID from my.telegram.org |
| API Hash | `-api-hash` | `TELEGRAM_API_HASH` | API Hash from my.telegram.org |
| Phone | `-phone` | `TELEGRAM_PHONE` | Your phone number (with country code) |
| Download Folder | `-folder` | `TELEGRAM_FOLDER` | Local directory for downloads |
| User ID | `-user` | `TELEGRAM_USER_ID` | Whitelisted user ID |

#### Optional Parameters

| Parameter | Flag | Environment Variable | Default | Description |
|-----------|------|---------------------|---------|-------------|
| Debug Mode | `-debug` | `TELEGRAM_DEBUG` | `false` | Enable verbose logging |
| Allowed Types | `-types` | `TELEGRAM_ALLOWED_TYPES` | (all) | Comma-separated extensions |
| Session File | `-session` | - | `session.json` | Path to session storage |

### 4. First Run (Authentication)

On the first run, you'll need to authenticate:

```bash
# Using command-line flags
./tg-bot-files-dwl \
  -api-id 12345678 \
  -api-hash "abcdef1234567890abcdef1234567890" \
  -phone "+1234567890" \
  -folder "/path/to/downloads" \
  -user "123456789"
```

**Or using environment variables:**

```bash
export TELEGRAM_API_ID="12345678"
export TELEGRAM_API_HASH="abcdef1234567890abcdef1234567890"
export TELEGRAM_PHONE="+1234567890"
export TELEGRAM_FOLDER="/path/to/downloads"
export TELEGRAM_USER_ID="123456789"

./tg-bot-files-dwl
```

You'll be prompted to:
1. Enter the verification code sent to your Telegram app
2. Enter your 2FA password (if enabled)

### 5. Subsequent Runs

After the first successful authentication, a `session.json` file is created. This file stores your session, so you won't need to re-authenticate on subsequent runs.

**Important**: Keep `session.json` secure! It contains authentication credentials.

## Usage Examples

### Basic Usage
```bash
./tg-bot-files-dwl \
  -api-id 12345678 \
  -api-hash "your-api-hash" \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "987654321"
```

### With File Type Restrictions
```bash
./tg-bot-files-dwl \
  -api-id 12345678 \
  -api-hash "your-api-hash" \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "987654321" \
  -types "pdf,docx,txt,zip"
```

### With Debug Mode
```bash
./tg-bot-files-dwl \
  -api-id 12345678 \
  -api-hash "your-api-hash" \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "987654321" \
  -debug "true"
```

## Docker Support

### Build the Docker Image

```bash
docker build -t tg-bot-files-dwl .
```

### Run with Docker

```bash
docker run -d \
  -e TELEGRAM_API_ID="12345678" \
  -e TELEGRAM_API_HASH="your-api-hash" \
  -e TELEGRAM_PHONE="+1234567890" \
  -e TELEGRAM_FOLDER="/downloads" \
  -e TELEGRAM_USER_ID="987654321" \
  -v /host/downloads:/downloads \
  -v /host/session:/app/session.json \
  --name tg-bot \
  tg-bot-files-dwl
```

**Note**: For Docker, you'll need to handle the initial authentication differently since it requires interactive input. Options:
1. Run authentication once locally, then copy `session.json` to the Docker volume
2. Use Docker's interactive mode for the first run: `docker run -it ...`

## Features Preserved

All existing features from the Bot API version are maintained:

✅ User whitelist (single user ID)  
✅ File type validation (optional extension filter)  
✅ Real-time download progress with progress bar  
✅ Duplicate filename handling  
✅ Filename sanitization  
✅ Error handling with user-friendly messages  
✅ ETA calculation  
✅ Download speed tracking  

## New Features

🎉 **Large file support**: Download files up to 2 GB  
🎉 **Session persistence**: No need to re-authenticate on every restart  
🎉 **Flood protection**: Built-in rate limiting and flood wait handling  
🎉 **Better reliability**: Direct MTProto connection, no Bot API middleman  

## Troubleshooting

### "Invalid phone number" error
- Make sure to include the country code (e.g., `+1` for US)
- Format: `+[country code][number]` (e.g., `+1234567890`)

### "Session file is corrupted" error
- Delete `session.json` and re-authenticate
- Make sure the session file has proper read/write permissions

### "Flood wait" errors
- The bot includes automatic flood wait handling
- If you hit rate limits, the bot will automatically wait and retry
- For severe rate limiting, you may need to wait several hours

### "API ID/Hash invalid" error
- Double-check your credentials from https://my.telegram.org
- Make sure there are no extra spaces or quotes in the values

### File download fails for large files
- Check available disk space
- Verify folder permissions
- Check your internet connection stability

## Security Notes

1. **Protect your credentials**: Never commit `session.json` or expose API credentials
2. **Session file security**: The session file allows anyone to act as your account
3. **User ID whitelist**: Only the specified user can use the bot
4. **File type restrictions**: Use `-types` to limit allowed file extensions

## Differences from Bot API Version

| Feature | Bot API | Client API |
|---------|---------|------------|
| Max file size | 20 MB | 2 GB |
| Authentication | Bot token | Phone number + code |
| Session storage | None | session.json file |
| Account type | Bot account | User account |
| Rate limits | 20 req/sec | Varies by method |
| Startup time | Instant | ~2-3 seconds |

## Getting User IDs

To find a user's Telegram ID, you can:
1. Use bots like @userinfobot or @getidsbot
2. Forward a message from the user to @userinfobot
3. Check Telegram Desktop: Settings → Advanced → Enable debug mode → Right-click on user

## Support

If you encounter issues:
1. Check the logs (enable debug mode with `-debug true`)
2. Verify all configuration parameters
3. Ensure `session.json` is valid
4. Try deleting `session.json` and re-authenticating

## Building from Source

```bash
# Clone the repository
git clone <repository-url>
cd tg-bot-files-dwl

# Install dependencies
go mod tidy

# Build
go build -o tg-bot-files-dwl .

# Run
./tg-bot-files-dwl [flags]
```

## Environment Variables Template

Create a `.env` file (don't commit it!):

```bash
TELEGRAM_API_ID=12345678
TELEGRAM_API_HASH=abcdef1234567890abcdef1234567890
TELEGRAM_PHONE=+1234567890
TELEGRAM_FOLDER=/path/to/downloads
TELEGRAM_USER_ID=987654321
TELEGRAM_DEBUG=false
TELEGRAM_ALLOWED_TYPES=pdf,docx,txt,zip
```

Then source it:
```bash
source .env
./tg-bot-files-dwl
```
