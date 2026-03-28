package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"reel/internal/database/models"
	"reel/internal/utils"
)

// TelegramClient implements the Notifier interface for Telegram.
type TelegramClient struct {
	botToken string
	chatID   string
	logger   *utils.Logger
}

// NewTelegramClient creates a new client for sending Telegram notifications.
func NewTelegramClient(botToken, chatID string, logger *utils.Logger) *TelegramClient {
	return &TelegramClient{
		botToken: botToken,
		chatID:   chatID,
		logger:   logger,
	}
}

func (c *TelegramClient) sendMessage(text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.botToken)

	payload, err := json.Marshal(map[string]string{
		"chat_id":    c.chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *TelegramClient) NotifyDownloadStart(media *models.Media, torrentName string) {
	text := fmt.Sprintf("<b>Download Started: %s</b>\nStarted downloading: %s", media.Title, torrentName)
	if err := c.sendMessage(text); err != nil {
		c.logger.Error("Error sending Telegram notification:", err)
	}
}

func (c *TelegramClient) NotifyDownloadComplete(media *models.Media, torrentName string) {
	text := fmt.Sprintf("<b>Download Complete: %s</b>\nFinished downloading: %s", media.Title, torrentName)
	if err := c.sendMessage(text); err != nil {
		c.logger.Error("Error sending Telegram notification:", err)
	}
}

func (c *TelegramClient) NotifyPostProcessComplete(media *models.Media, torrentName string) {
	text := fmt.Sprintf("<b>Ready to Watch: %s</b>\nPost-processing complete for: %s", media.Title, torrentName)
	if err := c.sendMessage(text); err != nil {
		c.logger.Error("Error sending Telegram notification:", err)
	}
}

func (c *TelegramClient) NotifyNotEnoughSpace(media *models.Media, torrentName string) {
	text := fmt.Sprintf("<b>Error downloading %s</b>\nNot enough space on disk", media.Title)
	if err := c.sendMessage(text); err != nil {
		c.logger.Error("Error sending Telegram notification:", err)
	}
}

func (c *TelegramClient) NotifyDownloadError(media *models.Media, torrentName string) {
	text := fmt.Sprintf("<b>Error downloading %s</b>\nDownload process failed for %s", media.Title, torrentName)
	if err := c.sendMessage(text); err != nil {
		c.logger.Error("Error sending Telegram notification:", err)
	}
}

// Test verifies the bot token is valid by calling the getMe endpoint.
func (c *TelegramClient) Test() error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", c.botToken)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("telegram authentication failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram authentication failed with status %d", resp.StatusCode)
	}
	return nil
}
