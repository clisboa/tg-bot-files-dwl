# Channel Mode - Simplified Setup

## Why Use Channel Mode?

Using a **channel or group** instead of private messages solves several issues:

✅ **No access hash problems** - Entities are always available in channel updates  
✅ **Greeting always works** - Can send messages to channel immediately  
✅ **Simpler code** - No need to manage contacts  
✅ **Better for teams** - Multiple users can see the bot's activity  
✅ **Easier to debug** - All messages visible in one place  

## Setup Instructions

### Step 1: Create a Private Channel or Group

1. Open Telegram
2. Create a new **Private Channel** or **Private Group**
   - Channels: Only you can post, bot responds there
   - Groups: You and bot can both interact
3. Name it something like "File Downloads" or "Bot Storage"

### Step 2: Add Your Bot Account to the Channel

1. Add your bot account as a member
   - For channels: Add as administrator with "Post Messages" permission
   - For groups: Just add as a regular member
2. Make sure the bot account has joined

### Step 3: Get the Channel ID

**Method 1: Using @userinfobot**
1. Forward any message from your channel to @userinfobot
2. It will reply with the channel ID (e.g., `-1001234567890`)
3. Copy this ID

**Method 2: Using Web Telegram**
1. Open https://web.telegram.org
2. Open your channel
3. Look at the URL: `https://web.telegram.org/k/#-1001234567890`
4. The number after `#` is your channel ID

**Method 3: Using the bot logs**
1. Send a message to the channel
2. Check bot logs - it will show the channel ID

### Step 4: Configure the Bot

Add the channel ID to your configuration:

**Using environment variable:**
```bash
export TELEGRAM_CHANNEL_ID="-1001234567890"
```

**Using command-line flag:**
```bash
./tg-bot-files-dwl \
  -channel "-1001234567890" \
  -api-id 12345678 \
  -api-hash "abc..." \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "987654321"
```

**Using .env file:**
```bash
TELEGRAM_CHANNEL_ID=-1001234567890
```

### Step 5: Run the Bot

```bash
./tg-bot-files-dwl [your flags...]
```

You should see:
```
Monitoring channel/group ID: -1001234567890
✅ Sent greeting to channel -1001234567890
Bot is running... Monitoring for documents
```

### Step 6: Use the Bot

1. Go to your private channel/group
2. Send any document file
3. Bot downloads it automatically
4. Bot sends progress updates in the channel

---

## Example Configurations

### Private Channel (Recommended)

```bash
# .env
TELEGRAM_API_ID=12345678
TELEGRAM_API_HASH=abcdef...
TELEGRAM_PHONE=+1234567890
TELEGRAM_FOLDER=/downloads
TELEGRAM_CHANNEL_ID=-1001234567890
TELEGRAM_USER_ID=987654321
```

**Advantages:**
- Only you can post
- Clean message history
- Easy to archive

### Private Group

```bash
# Same as above, just use group ID instead
TELEGRAM_CHANNEL_ID=-1001234567890  # Your group ID
```

**Advantages:**
- Can add other users to watch
- More interactive
- Can have multiple bots

---

## Docker with Channel Mode

```yaml
# docker-compose.yml
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
      - TELEGRAM_CHANNEL_ID=${TELEGRAM_CHANNEL_ID}  # Channel mode
      - TELEGRAM_USER_ID=${TELEGRAM_USER_ID}
      - TELEGRAM_CODE_FILE=/auth/telegram_code.txt
      - TELEGRAM_PASSWORD_FILE=/auth/telegram_password.txt
    volumes:
      - ./downloads:/downloads
      - ./session.json:/session.json
      - ./auth:/auth
```

```bash
# Start
docker-compose up -d

# Authenticate (first time only)
echo "12345" > auth/telegram_code.txt

# Done! Bot is running in channel mode
```

---

## Comparison: Private Chat vs Channel Mode

| Feature | Private Chat | Channel Mode |
|---------|-------------|--------------|
| **Setup complexity** | Hard (needs contacts) | Easy (just create channel) |
| **Greeting on start** | ❌ Often fails | ✅ Always works |
| **Access hash issues** | ⚠️ Common | ✅ No issues |
| **Code complexity** | More complex | Simpler |
| **Multi-user** | ❌ One user only | ✅ Can share channel |
| **Message history** | ❌ Scattered | ✅ All in one place |
| **Privacy** | ✅ Completely private | ✅ Private (if channel is private) |

---

## Troubleshooting

### "Could not send greeting to channel"

**Problem:** Bot can't post to channel

**Solutions:**
1. Make sure bot is a member of the channel
2. For channels: Bot needs admin rights with "Post Messages"
3. For groups: Bot just needs to be a member
4. Check channel ID is correct (should be negative for supergroups)

### "Ignoring message from unauthorized user"

**Problem:** Bot ignores messages from others

**This is correct!** Only messages from `TELEGRAM_USER_ID` are processed, even in channel mode.

**To allow multiple users:** Run multiple bot instances with different user IDs, or modify the code to accept multiple users.

### "Not from our channel" in logs

**Problem:** Bot sees messages but ignores them

**Solution:** Check that `TELEGRAM_CHANNEL_ID` matches the channel you're posting to.

---

## Getting Channel IDs

### For Regular Groups
- Groups have positive IDs
- Example: `123456789`

### For Supergroups and Channels
- Start with `-100`
- Example: `-1001234567890`
- This is what you'll usually use

### Quick Command to Get ID

Send this message to your channel:
```
/id
```

Then check bot debug logs to see the channel ID.

---

## Migration from Private Chat

Already running in private chat mode? Easy to migrate:

1. Create a private channel
2. Add bot account to channel
3. Get channel ID
4. Add `TELEGRAM_CHANNEL_ID` to config
5. Restart bot
6. Done! Now using channel mode

Your old private messages won't work anymore, but channel messages will work perfectly.

---

## Benefits Summary

**For Users:**
- ✅ No more "greeting failed" messages
- ✅ All downloads in one organized place
- ✅ Can review download history easily
- ✅ Can archive the channel for long-term storage

**For Admins:**
- ✅ Simpler setup and maintenance
- ✅ Fewer edge cases and bugs
- ✅ Easier to debug issues
- ✅ Can monitor multiple users (if desired)

---

## Recommended Setup

**Best practice for personal use:**

1. Create a **Private Channel** named "Downloads"
2. Add only yourself and the bot
3. Use channel mode
4. All your downloads in one place
5. Perfect message history
6. Zero access hash issues

```bash
./tg-bot-files-dwl \
  -channel "-1001234567890" \
  -api-id 12345678 \
  -api-hash "your-hash" \
  -phone "+1234567890" \
  -folder "./downloads" \
  -user "your-user-id" \
  -types "epub,pdf,mobi"
```

That's it! Enjoy hassle-free file downloads! 🎉
