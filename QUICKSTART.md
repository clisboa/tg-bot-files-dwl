# Quick Start Guide

## TL;DR - Get Started in 5 Minutes

### Step 1: Get API Credentials (2 minutes)

1. Go to https://my.telegram.org
2. Log in with your phone number
3. Click "API development tools"
4. Fill in the application form (any name works)
5. Copy your **API ID** and **API Hash**

### Step 2: Find Your User ID (1 minute)

1. Open Telegram
2. Search for `@userinfobot`
3. Start the bot and it will show your User ID

### Step 3: Run the Bot (2 minutes)

#### Using Command Line Flags:

```bash
./tg-bot-files-dwl \
  -api-id YOUR_API_ID \
  -api-hash "YOUR_API_HASH" \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "YOUR_USER_ID"
```

#### Or Using Environment Variables:

```bash
# Create .env file (copy from .env.example)
cp .env.example .env

# Edit .env with your values
nano .env

# Run the bot
export $(cat .env | xargs) && ./tg-bot-files-dwl
```

### Step 4: Authenticate (First Run Only)

When you run the bot for the first time:

1. The bot will display:
   ```
   VERIFICATION CODE REQUIRED
   Please create the file: telegram_code.txt
   Waiting for code file (timeout: 5 minutes)...
   ```

2. Check your Telegram app for the verification code (e.g., `12345`)

3. Create the code file:
   ```bash
   echo "12345" > telegram_code.txt
   ```

4. The bot automatically reads the file, authenticates, and deletes it

5. If you have 2FA enabled, repeat with password:
   ```bash
   echo "your_password" > telegram_password.txt
   ```

That's it! The bot is now running and will download any documents you send to yourself.

**Note:** For detailed authentication instructions, especially for Docker, see [AUTHENTICATION.md](AUTHENTICATION.md)

---

## What Changed from Bot API Version?

| Before | After |
|--------|-------|
| 20 MB file limit | 2 GB file limit |
| Bot token (@BotFather) | User account (phone number) |
| Instant setup | One-time authentication |
| No session file | Saves session.json |

---

## Common Issues

### "Invalid phone number"
Make sure to include the country code: `+1234567890` not `1234567890`

### "API ID/Hash invalid"
Double-check your credentials from https://my.telegram.org - no extra spaces!

### Can't send verification code
The bot needs to authenticate with YOUR account, so the code goes to YOUR Telegram app

### "Session file corrupted"
Delete `session.json` and run the bot again to re-authenticate

---

## Example Configurations

### Basic (all file types):
```bash
./tg-bot-files-dwl \
  -api-id 12345678 \
  -api-hash "abc123..." \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "987654321"
```

### With file type restrictions:
```bash
./tg-bot-files-dwl \
  -api-id 12345678 \
  -api-hash "abc123..." \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "987654321" \
  -types "pdf,zip,docx"
```

### With debug logging:
```bash
./tg-bot-files-dwl \
  -api-id 12345678 \
  -api-hash "abc123..." \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "987654321" \
  -debug "true"
```

---

## Testing

Send yourself a document on Telegram and the bot will:
1. Show a download progress bar
2. Save it to your downloads folder
3. Notify you when complete

---

## Building from Source

```bash
# Clone and build
git clone <your-repo>
cd tg-bot-files-dwl
go mod tidy
go build -o tg-bot-files-dwl .

# Run
./tg-bot-files-dwl -api-id ... -api-hash ... -phone ... -folder ... -user ...
```

---

## Docker (Advanced)

```bash
# Build
docker build -t tg-bot-files-dwl .

# First run (interactive for authentication)
docker run -it --rm \
  -e TELEGRAM_API_ID="12345678" \
  -e TELEGRAM_API_HASH="abc123..." \
  -e TELEGRAM_PHONE="+1234567890" \
  -e TELEGRAM_FOLDER="/downloads" \
  -e TELEGRAM_USER_ID="987654321" \
  -v $(pwd)/downloads:/downloads \
  -v $(pwd)/session.json:/session.json \
  tg-bot-files-dwl

# After authentication, run in background
docker run -d \
  -e TELEGRAM_API_ID="12345678" \
  -e TELEGRAM_API_HASH="abc123..." \
  -e TELEGRAM_PHONE="+1234567890" \
  -e TELEGRAM_FOLDER="/downloads" \
  -e TELEGRAM_USER_ID="987654321" \
  -v $(pwd)/downloads:/downloads \
  -v $(pwd)/session.json:/session.json \
  --name tg-bot \
  tg-bot-files-dwl
```

Or use docker-compose:

```bash
# Setup environment
cp .env.example .env
# Edit .env with your values

# First run (interactive)
docker-compose run --rm tg-bot

# After authentication, run in background
docker-compose up -d
```

---

## Security Tips

1. **Never commit** `.env` or `session.json` to git
2. The `session.json` file = full access to your account - keep it safe!
3. Only the whitelisted user (your User ID) can trigger downloads
4. Use file type restrictions (`-types`) to limit what gets downloaded

---

## Need Help?

Check the full documentation in [README_MIGRATION.md](README_MIGRATION.md)

Happy downloading! 🚀
