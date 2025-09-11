# Download Workflow

This document outlines the complete lifecycle of a download in Reel, from adding a new media item to the final post-processing.

1.  **Adding Media**:
    * A user adds a new movie, TV show, or anime through the UI.
    * Reel fetches metadata from your configured providers (e.g., TMDB, TVmaze) to get details like the title, year, and episode information.
    * The media item is added to the database with a **`pending`** status.

2.  **Searching**:
    * The **Process Pending Media** scheduled task runs every 30 minutes and adds all media with a **`pending`** status to the search queue.
    * For each item in the queue, Reel searches your configured indexers for a suitable download.
    * The status of the media item is updated to **`searching`**.

3.  **Torrent Selection**:
    * Reel filters the search results based on your quality preferences, rejection rules, and minimum seeder requirements.
    * The remaining torrents are scored based on quality and seeders, and the best one is selected.
    * If no suitable torrent is found, the media item's status is set to **`failed`**.

4.  **Downloading**:
    * The selected torrent is sent to your configured download client (e.g., Transmission, qBittorrent).
    * The media item's status is updated to **`downloading`**.
    * The **Update Download Status** scheduled task runs every 10 seconds to update the download progress in Reel.

5.  **Post-Processing**:
    * Once the download is complete, the **Update Download Status** task marks the media item as **`downloaded`**.
    * The post-processor is triggered, which performs the following actions:
        * Creates a destination folder for the media item.
        * Moves, copies, or creates a symlink for the downloaded files to the destination folder.
        * Renames the files according to your configured patterns.
    * Notifications are sent to inform you that the download is complete and ready to watch.

6.  **Cleanup**:
    * The **Cleanup Completed Torrents** scheduled task runs every 24 hours to remove completed torrents from your download client based on your seeding rules.