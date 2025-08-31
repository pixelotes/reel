package notifications

import "reel/internal/database/models"

// Notifier defines the standard interface for sending notifications.
type Notifier interface {
	// NotifyDownloadStart is called when a download begins.
	NotifyDownloadStart(media *models.Media, torrentName string)
	// NotifyDownloadComplete is called when a download finishes.
	NotifyDownloadComplete(media *models.Media, torrentName string)
	// Test sends a test notification to verify settings.
	Test() error
}
