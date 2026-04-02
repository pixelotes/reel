# Notifications

Reel can send notifications when key events occur during media processing. Two notification providers are supported: Telegram and Pushbullet.

---

## Supported Providers

| Provider | Auth Method | Delivery | HTML Formatting |
|----------|-------------|----------|-----------------|
| Telegram | Bot token + chat ID | Single chat | Yes |
| Pushbullet | API key | All devices | No (plain text) |

---

## Notification Events

Both providers send notifications for the same set of events:

| Event | Description |
|-------|-------------|
| Download started | A torrent has been added to the download client. |
| Download complete | A torrent has finished downloading. |
| Download error | A download has failed. |
| Not enough space | The download was skipped due to insufficient disk space. |
| Post-process complete | File renaming, moving, and subtitles have finished successfully. |

---

## Telegram

Telegram notifications are sent through a bot using the Telegram Bot API. Messages are HTML-formatted.

### Setup

1. Open Telegram and message [@BotFather](https://t.me/BotFather)
2. Send `/newbot` and follow the prompts to create your bot
3. Copy the bot token BotFather gives you
4. Start a conversation with your new bot (send it any message)
5. Get your chat ID by visiting `https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates` and looking for `chat.id`

### Configuration

```yaml
notifications:
  telegram:
    bot_token: "123456789:ABCdefGhIjKlMnOpQrStUvWxYz"
    chat_id: "987654321"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `bot_token` | string | Yes | Bot token from @BotFather. |
| `chat_id` | string | Yes | Telegram chat ID where messages are sent. |

!!! tip
    You can send notifications to a group chat. Add the bot to the group, send a message in the group, then check `getUpdates` for the group's chat ID (it will be a negative number).

---

## Pushbullet

Pushbullet broadcasts notifications to all devices linked to your account.

### Setup

1. Sign in at [pushbullet.com](https://www.pushbullet.com/)
2. Go to Settings > Access Tokens
3. Create a new access token and copy it

### Configuration

```yaml
notifications:
  pushbullet:
    api_key: "o.AbCdEfGhIjKlMnOpQrStUvWxYz1234"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `api_key` | string | Yes | Pushbullet access token. |

---

## Enabling Notifications

Configuring a provider's credentials is not enough on its own. You must also add the provider name to the `automation.notifications` list to activate it.

```yaml
notifications:
  telegram:
    bot_token: "your_bot_token"
    chat_id: "your_chat_id"
  pushbullet:
    api_key: "your_pushbullet_key"

automation:
  notifications: ["telegram"]  # only Telegram is active
```

To enable both providers:

```yaml
automation:
  notifications: ["telegram", "pushbullet"]
```

To disable all notifications, leave the list empty:

```yaml
automation:
  notifications: []
```

!!! warning
    If `automation.notifications` is empty or omitted, no notifications will be sent even if credentials are configured.

---

## Testing

Both providers include a `Test()` method that validates the configuration by sending a test notification. This is available through the Reel web UI when testing provider settings.

---

## Full Example

```yaml
notifications:
  telegram:
    bot_token: "123456789:ABCdefGhIjKlMnOpQrStUvWxYz"
    chat_id: "987654321"
  pushbullet:
    api_key: "o.AbCdEfGhIjKlMnOpQrStUvWxYz1234"

automation:
  notifications: ["telegram", "pushbullet"]
```
