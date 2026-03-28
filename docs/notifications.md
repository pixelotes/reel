# Notifications

Reel can send notifications at key points during the download and post-processing lifecycle. Multiple notification providers can be active at the same time.

## Events

| Event                  | When it fires                                      |
|------------------------|----------------------------------------------------|
| Download Started       | A torrent begins downloading.                      |
| Download Complete      | A torrent finishes downloading.                    |
| Download Error         | A download fails.                                  |
| Not Enough Space       | There isn't enough disk space to start a download.  |
| Post-Process Complete  | Post-processing finishes and the media is ready.   |

## Configuration

Notifications require two things:

1. **Provider credentials** in the `notifications` section.
2. **Provider name** listed in `automation.notifications`.

```yaml
notifications:
  pushbullet:
    api_key: "your-pushbullet-api-key"
  telegram:
    bot_token: "your-telegram-bot-token"
    chat_id: "your-chat-id"

automation:
  notifications: ["pushbullet", "telegram"]  # Enable one or both
```

Only providers listed in `automation.notifications` will be initialized. You can configure credentials for a provider without activating it.

## Providers

### Pushbullet

Sends push notifications to all devices linked to your Pushbullet account.

```yaml
notifications:
  pushbullet:
    api_key: "your-pushbullet-api-key"

automation:
  notifications: ["pushbullet"]
```

| Setting   | Description                                                        |
|-----------|--------------------------------------------------------------------|
| `api_key` | Your Pushbullet API key. Get it from https://www.pushbullet.com/#settings/account |

### Telegram

Sends messages to a Telegram chat via the Bot API. Messages use HTML formatting.

```yaml
notifications:
  telegram:
    bot_token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
    chat_id: "-1001234567890"

automation:
  notifications: ["telegram"]
```

| Setting     | Description                                                              |
|-------------|--------------------------------------------------------------------------|
| `bot_token` | The bot token from [@BotFather](https://t.me/BotFather).                 |
| `chat_id`   | The chat ID to send messages to. Can be a user, group, or channel ID.    |

#### Getting your Telegram chat ID

1. Create a bot with [@BotFather](https://t.me/BotFather) and copy the token.
2. Send a message to your bot (or add it to a group).
3. Open `https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates` in a browser.
4. Look for `"chat":{"id":...}` in the response — that's your `chat_id`.

For groups and channels, the chat ID is typically a negative number (e.g., `-1001234567890`).

## Examples

### Telegram only

```yaml
notifications:
  telegram:
    bot_token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
    chat_id: "-1001234567890"

automation:
  notifications: ["telegram"]
```

### Pushbullet only

```yaml
notifications:
  pushbullet:
    api_key: "o.ABCdef123456"

automation:
  notifications: ["pushbullet"]
```

### Both providers

```yaml
notifications:
  pushbullet:
    api_key: "o.ABCdef123456"
  telegram:
    bot_token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
    chat_id: "-1001234567890"

automation:
  notifications: ["pushbullet", "telegram"]
```

### Configured but disabled

Credentials are set but the provider is not listed in `automation.notifications`, so no notifications are sent:

```yaml
notifications:
  telegram:
    bot_token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
    chat_id: "-1001234567890"

automation:
  notifications: []  # Nothing active
```
