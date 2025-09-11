package notifications

import (
	"fmt"

	"reel/internal/database/models"
	"reel/internal/utils"

	"github.com/xconstruct/go-pushbullet"
)

// PushbulletClient implements the Notifier interface for Pushbullet.
type PushbulletClient struct {
	apiKey string
	pb     *pushbullet.Client
	logger *utils.Logger
}

// NewPushbulletClient creates a new client for sending Pushbullet notifications.
func NewPushbulletClient(apiKey string, logger *utils.Logger) *PushbulletClient {
	pb := pushbullet.New(apiKey)
	return &PushbulletClient{
		apiKey: apiKey,
		pb:     pb,
		logger: logger,
	}
}

// sendPush sends a note to all of the user's devices.
func (c *PushbulletClient) sendPush(title, body string) error {
	// The first argument to PushNote is the device iden. Empty means all devices.
	err := c.pb.PushNote("", title, body)
	return err
}

// NotifyDownloadStart sends a notification when a download begins.
func (c *PushbulletClient) NotifyDownloadStart(media *models.Media, torrentName string) {
	title := fmt.Sprintf("Download Started: %s", media.Title)
	body := fmt.Sprintf("Started downloading: %s", torrentName)
	if err := c.sendPush(title, body); err != nil {
		c.logger.Error("Error sending Pushbullet notification:", err)
	}
}

// NotifyDownloadComplete sends a notification when a download finishes.
func (c *PushbulletClient) NotifyDownloadComplete(media *models.Media, torrentName string) {
	title := fmt.Sprintf("Download Complete: %s", media.Title)
	body := fmt.Sprintf("Finished downloading: %s", torrentName)
	if err := c.sendPush(title, body); err != nil {
		c.logger.Error("Error sending Pushbullet notification:", err)
	}
}

func (c *PushbulletClient) NotifyPostProcessComplete(media *models.Media, torrentName string) {
	title := fmt.Sprintf("Ready to Watch: %s", media.Title)
	body := fmt.Sprintf("Post-processing complete for: %s", torrentName)
	if err := c.sendPush(title, body); err != nil {
		c.logger.Error("Error sending Pushbullet post-process notification:", err)
	}
}

func (c *PushbulletClient) NotifyNotEnoughSpace(media *models.Media, torrentName string) {
	title := fmt.Sprintf("Error downloading %s", media.Title)
	body := fmt.Sprintf("Not enough space on disk")
	if err := c.sendPush(title, body); err != nil {
		c.logger.Error("Error sending Pushbullet post-process notification:", err)
	}
}

func (c *PushbulletClient) NotifyDownloadError(media *models.Media, torrentName string) {
	title := fmt.Sprintf("Error downloading %s", media.Title)
	body := fmt.Sprintf("Download process failed for %s", torrentName)
	if err := c.sendPush(title, body); err != nil {
		c.logger.Error("Error sending Pushbullet post-process notification:", err)
	}
}

// Test verifies the API key is valid by fetching user info.
func (c *PushbulletClient) Test() error {
	_, err := c.pb.Me()
	if err != nil {
		return fmt.Errorf("pushbullet authentication failed: %w", err)
	}
	return nil
}
