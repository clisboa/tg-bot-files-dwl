# How to Enable Greeting Message on Bot Start

## The Issue

When using the **Client API** (MTProto), Telegram requires an **access hash** to send messages to users. This is a security feature to prevent spam.

You can only get someone's access hash if:
- ✅ They're in your contacts
- ✅ They've sent you a message before
- ✅ You're both in a mutual group

## Solution: Add User to Contacts

To enable the greeting message when the bot starts, you need to **add the target user to your bot account's contacts**.

### **Method 1: Add Contact from Telegram App (Easiest)**

1. **Login to your bot account** on Telegram (desktop or mobile)
   - Use the same phone number you configured in `TELEGRAM_PHONE`

2. **Add the user to contacts:**
   - Click "New Contact" or "Add Contact"
   - Enter the user's phone number
   - Save the contact

3. **Restart the bot**
   - The greeting will now work!

### **Method 2: Send Message from User First**

Alternatively, the user can simply:

1. Open Telegram with their personal account
2. Search for your bot account by phone number or username
3. Send any message (even just "hi")
4. The bot will respond with downloads from that point forward
5. On next bot restart, greeting will work

### **Method 3: Add Contact via Bot (Not Implemented)**

This would require adding code to import contacts programmatically, which is more complex and not recommended.

---

## What You'll See

### **Before Adding Contact:**

```
2025/12/27 18:17:54 Greeting skipped: user 1302911586 not in contacts
2025/12/27 18:17:54 💡 To receive greeting on bot start:
2025/12/27 18:17:54    1. Add user 1302911586 to bot account's contacts, OR
2025/12/27 18:17:54    2. Send any message from user to bot first
2025/12/27 18:17:54 Bot is running... Monitoring for documents
```

### **After Adding Contact:**

```
2025/12/27 18:20:00 Authentication successful!
2025/12/27 18:20:00 Logged in as: Tasca bot  (ID: 7161662715)
2025/12/27 18:20:00 ✅ Sent greeting to user 1302911586
2025/12/27 18:20:00 Bot is running... Monitoring for documents
```

---

## Why This Limitation Exists

This is a **Telegram platform limitation**, not a bug in the bot:

1. **Bot API (old):** Uses bot tokens, can't send unsolicited messages
2. **Client API (new):** Uses user accounts, requires access hash for security

Both have this restriction - you can't send messages to random users without prior interaction.

---

## Summary

**Option A (Recommended):**
- Login to bot account on Telegram app
- Add user to contacts
- Restart bot → greeting works ✅

**Option B (Simpler):**
- Just use the bot normally
- Send a document from your user account
- Bot works perfectly ✅
- Next restart, greeting will work

**The bot is fully functional either way!** The greeting is just a nice-to-have feature. The core functionality (downloading files) works perfectly regardless.
