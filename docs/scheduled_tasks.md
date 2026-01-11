# Scheduled Tasks

Reel runs several automated tasks in the background to keep your media library up-to-date. These tasks are managed by a scheduler and run at predefined intervals.

| Task                          | Config Key | Default | Description |
| ----------------------------- | ---------- | ------- | ----------- |
| **Process Pending Media**     | `search_interval` | `@every 30m` | Searches for any media marked as "pending" or "failed" and adds them to the search queue to find a suitable download. |
| **Check for New Episodes**    | `new_episodes_check_interval` | `@every 6h` | For TV shows and anime, this task checks for new episodes that have aired and adds them to the database with a "pending" status. |
| **Update Download Status**    | `download_status_interval` | `@every 10s` | Checks the status of all active downloads in your torrent client and updates the progress in Reel. |
| **Process RSS Feeds**         | `rss_processing_interval` | `@every 1h` | Fetches the latest items from your configured RSS feeds and matches them against your pending media to find and start new downloads. |
| **Cleanup Completed Torrents**| `cleanup_interval` | `@every 24h` | Removes completed torrents from your download client based on the seeding rules you've configured (e.g., seed ratio or time). |
| **Retry Failed Downloads**    | `retry_failed_interval` | `@every 1h` | Automatically retries any downloads that have previously failed. |