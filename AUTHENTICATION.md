# File-Based Authentication Guide

## Overview

This bot uses **file-based authentication** instead of interactive terminal input. This makes it perfect for Docker containers and automated deployments where you can't interact with stdin.

## How It Works

Instead of typing the verification code directly, you:
1. Start the bot
2. The bot waits for authentication
3. You create a file with the verification code
4. The bot automatically reads the file and completes authentication
5. The file is automatically deleted for security

## Authentication Flow

### First Time Setup

#### Step 1: Start the Bot

```bash
./tg-bot-files-dwl \
  -api-id YOUR_API_ID \
  -api-hash "YOUR_API_HASH" \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "YOUR_USER_ID"
```

#### Step 2: Bot Waits for Verification Code

You'll see output like:
```
===========================================
VERIFICATION CODE REQUIRED
===========================================
A verification code has been sent to your Telegram app
Please create the file: telegram_code.txt
Write the verification code to this file
Waiting for code file (timeout: 5 minutes)...
===========================================
```

#### Step 3: Check Your Telegram App

Open your Telegram app and you'll see a login code (e.g., `12345`)

#### Step 4: Create the Code File

**On Linux/Mac:**
```bash
echo "12345" > telegram_code.txt
```

**On Windows:**
```powershell
echo 12345 > telegram_code.txt
```

**In Docker:**
```bash
# From host machine
docker exec -i tg-bot sh -c 'echo "12345" > /app/telegram_code.txt'

# Or use a mounted volume
echo "12345" > /path/to/mounted/volume/telegram_code.txt
```

#### Step 5: Bot Reads and Deletes File

The bot will:
- Detect the file
- Read the verification code
- Authenticate
- Delete the file automatically
- Log: `Verification code received and file deleted`

#### Step 6: 2FA Password (If Enabled)

If you have 2FA enabled, you'll see:
```
2FA password required. Waiting for password in file: telegram_password.txt
Please create the file and write your 2FA password to it
```

Create the password file:
```bash
echo "your_2fa_password" > telegram_password.txt
```

The bot will read and delete this file too.

#### Step 7: Session Saved

After successful authentication:
- `session.json` is created
- Future runs won't need authentication
- Keep this file secure!

---

## Configuration Options

### Default File Paths

| File | Default Path | Purpose |
|------|-------------|---------|
| Verification Code | `telegram_code.txt` | Login code from Telegram app |
| 2FA Password | `telegram_password.txt` | Two-factor password (if enabled) |
| Session | `session.json` | Saved authentication session |

### Custom File Paths

You can customize the file paths:

**Using Flags:**
```bash
./tg-bot-files-dwl \
  -code-file "/custom/path/code.txt" \
  -password-file "/custom/path/password.txt" \
  -session "/custom/path/session.json" \
  [other flags...]
```

**Using Environment Variables:**
```bash
export TELEGRAM_CODE_FILE="/custom/path/code.txt"
export TELEGRAM_PASSWORD_FILE="/custom/path/password.txt"

./tg-bot-files-dwl [other flags...]
```

---

## Docker Authentication

### Method 1: Interactive (Easiest)

```bash
# Start bot in interactive mode
docker run -it --rm \
  -e TELEGRAM_API_ID="12345678" \
  -e TELEGRAM_API_HASH="abc123..." \
  -e TELEGRAM_PHONE="+1234567890" \
  -e TELEGRAM_FOLDER="/downloads" \
  -e TELEGRAM_USER_ID="987654321" \
  -v $(pwd)/downloads:/downloads \
  -v $(pwd)/session.json:/session.json \
  tg-bot-files-dwl

# When prompted, in ANOTHER terminal:
echo "12345" > session.json && docker cp session.json tg-bot:/app/telegram_code.txt

# Or simpler - use docker exec:
docker exec -i <container-id> sh -c 'echo "12345" > /app/telegram_code.txt'
```

### Method 2: Mounted Volume (Recommended)

```bash
# Create a shared directory for auth files
mkdir auth-files

# Start bot with mounted auth directory
docker run -d \
  -e TELEGRAM_API_ID="12345678" \
  -e TELEGRAM_API_HASH="abc123..." \
  -e TELEGRAM_PHONE="+1234567890" \
  -e TELEGRAM_FOLDER="/downloads" \
  -e TELEGRAM_USER_ID="987654321" \
  -e TELEGRAM_CODE_FILE="/auth/telegram_code.txt" \
  -e TELEGRAM_PASSWORD_FILE="/auth/telegram_password.txt" \
  -v $(pwd)/downloads:/downloads \
  -v $(pwd)/session.json:/session.json \
  -v $(pwd)/auth-files:/auth \
  --name tg-bot \
  tg-bot-files-dwl

# Watch the logs
docker logs -f tg-bot

# When it asks for code, create the file on host:
echo "12345" > auth-files/telegram_code.txt

# Bot will automatically detect and use it
```

### Method 3: Docker Compose (Best for Production)

**docker-compose.yml:**
```yaml
version: '3.8'

services:
  tg-bot:
    build: .
    container_name: tg-bot-files-dwl
    restart: unless-stopped
    environment:
      - TELEGRAM_API_ID=${TELEGRAM_API_ID}
      - TELEGRAM_API_HASH=${TELEGRAM_API_HASH}
      - TELEGRAM_PHONE=${TELEGRAM_PHONE}
      - TELEGRAM_FOLDER=/downloads
      - TELEGRAM_USER_ID=${TELEGRAM_USER_ID}
      - TELEGRAM_CODE_FILE=/auth/telegram_code.txt
      - TELEGRAM_PASSWORD_FILE=/auth/telegram_password.txt
    volumes:
      - ./downloads:/downloads
      - ./session.json:/session.json
      - ./auth:/auth
```

**Usage:**
```bash
# Start the bot
docker-compose up -d

# Watch logs
docker-compose logs -f

# When prompted, create code file
echo "12345" > auth/telegram_code.txt

# Bot authenticates automatically
```

---

## Security Best Practices

### 1. File Permissions

The bot automatically deletes auth files after reading them, but you should still:

```bash
# Restrict file permissions before creating
touch telegram_code.txt
chmod 600 telegram_code.txt
echo "12345" > telegram_code.txt
```

### 2. Session File Protection

```bash
# Protect session file (contains full account access)
chmod 600 session.json

# In Docker, use a volume with restricted permissions
docker volume create --driver local \
  --opt type=none \
  --opt o=bind \
  --opt device=/secure/path/session.json \
  telegram-session
```

### 3. Never Commit Auth Files

The `.gitignore` already excludes:
- `telegram_code.txt`
- `telegram_password.txt`
- `session.json`
- `.env`

### 4. Environment Variables

Use environment variables for sensitive data:
```bash
# Don't hardcode in docker-compose.yml
# Use .env file (excluded from git)

# .env file:
TELEGRAM_API_ID=12345678
TELEGRAM_API_HASH=abc123...
TELEGRAM_PHONE=+1234567890
TELEGRAM_USER_ID=987654321
```

---

## Troubleshooting

### "Timeout waiting for file"

**Problem:** Bot times out after 5 minutes

**Solutions:**
1. Check the file path is correct: `cat telegram_code.txt`
2. Verify file has content: `ls -la telegram_code.txt`
3. In Docker, ensure volume is mounted correctly
4. Check file is in the correct directory

### "File is empty, waiting for content..."

**Problem:** File exists but has no content

**Solution:** Ensure you wrote content to the file:
```bash
# Wrong (creates empty file)
touch telegram_code.txt

# Correct
echo "12345" > telegram_code.txt
```

### File Not Being Detected in Docker

**Problem:** Bot doesn't see the file in Docker container

**Solutions:**

1. **Verify mount point:**
```bash
docker exec tg-bot ls -la /app/
```

2. **Check volume mapping:**
```bash
docker inspect tg-bot | grep -A 10 Mounts
```

3. **Use absolute paths:**
```bash
-v /full/path/to/auth:/auth
```

4. **Create file inside container:**
```bash
docker exec tg-bot sh -c 'echo "12345" > /app/telegram_code.txt'
```

### Permission Denied Errors

**Problem:** Bot can't read the file

**Solutions:**
```bash
# Fix file permissions
chmod 644 telegram_code.txt

# In Docker, ensure container user has access
docker exec tg-bot ls -la /app/telegram_code.txt
```

### Code Already Used

**Problem:** "Code invalid" or "Code already used"

**Solutions:**
1. Request a new code (restart the bot)
2. Don't reuse old codes
3. The code expires after a few minutes

### Session Corrupted

**Problem:** "Session invalid" or similar errors

**Solution:**
```bash
# Delete session and re-authenticate
rm session.json
./tg-bot-files-dwl [your flags...]
```

---

## Automation Examples

### Automated Authentication Script

```bash
#!/bin/bash
# auto-auth.sh

# Start bot in background
./tg-bot-files-dwl \
  -api-id "$TELEGRAM_API_ID" \
  -api-hash "$TELEGRAM_API_HASH" \
  -phone "$TELEGRAM_PHONE" \
  -folder "./downloads" \
  -user "$TELEGRAM_USER_ID" &

BOT_PID=$!

echo "Bot started with PID: $BOT_PID"
echo "Waiting for code request..."

# Wait for code file request (check logs)
sleep 5

# Prompt user for code
read -p "Enter verification code from Telegram: " CODE
echo "$CODE" > telegram_code.txt

echo "Code file created. Bot should authenticate now."

# Wait for bot to complete
wait $BOT_PID
```

### Kubernetes Secret-Based Auth

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: telegram-auth
type: Opaque
data:
  code: MTIzNDU=  # base64 encoded code
---
apiVersion: v1
kind: Pod
metadata:
  name: tg-bot
spec:
  containers:
  - name: tg-bot
    image: tg-bot-files-dwl
    volumeMounts:
    - name: auth
      mountPath: /auth
  volumes:
  - name: auth
    secret:
      secretName: telegram-auth
      items:
      - key: code
        path: telegram_code.txt
```

---

## Re-Authentication

If you need to re-authenticate:

```bash
# 1. Stop the bot
docker stop tg-bot  # or Ctrl+C if running locally

# 2. Delete session
rm session.json

# 3. Restart bot
docker start tg-bot

# 4. Provide new code when prompted
echo "54321" > telegram_code.txt
```

---

## Comparison: File-Based vs Interactive

| Feature | File-Based | Interactive (Terminal) |
|---------|-----------|----------------------|
| Docker-friendly | ✅ Yes | ❌ No |
| Automation | ✅ Easy | ❌ Difficult |
| Remote access | ✅ Yes | ⚠️ SSH required |
| Security | ✅ Auto-delete | ⚠️ Terminal history |
| Kubernetes | ✅ Yes | ❌ No |
| CI/CD | ✅ Yes | ❌ No |

---

## Summary

The file-based authentication approach:

✅ **Works in Docker** - No stdin required  
✅ **Works in Kubernetes** - Use secrets/configmaps  
✅ **Automation-friendly** - Scripts can create files  
✅ **Secure** - Files auto-delete after reading  
✅ **Flexible** - Custom file paths supported  
✅ **Remote-friendly** - Just create a file  

Perfect for production deployments!
