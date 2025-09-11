package notifications

import "reel/internal/database/models"

type Notifier interface {
	NotifyDownloadStart(media *models.Media, torrentName string)
	NotifyNotEnoughSpace(media *models.Media, torrentName string)
	NotifyDownloadError(media *models.Media, torrentName string)
	NotifyDownloadComplete(media *models.Media, torrentName string)
	NotifyPostProcessComplete(media *models.Media, torrentName string)
	Test() error
}
