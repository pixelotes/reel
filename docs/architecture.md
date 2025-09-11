# Architecture

Reel is a monolithic application written in Go. It is designed to be lightweight and efficient, making it suitable for running on low-end devices.

### Components

* **`main.go`**: The main entry point for the application. It initializes the configuration, database, logger, and manager, and starts the web server.
* **`internal/config`**: This package is responsible for loading and parsing the `config.yml` file.
* **`internal/database`**: This package manages the SQLite database, including running migrations and providing a repository for accessing the data.
* **`internal/core`**: This is the core of the application, containing the manager, torrent selector, and post-processor.
* **`internal/handlers`**: This package contains the web server and API handlers.
* **`internal/clients`**: This package contains the clients for interacting with external services, such as indexers, metadata providers, and torrent clients.
* **`internal/utils`**: This package contains utility functions for things like sanitizing filenames, converting subtitles, and managing the logger.

### Data Flow

1.  A user adds a new media item through the web UI.
2.  The API handler calls the manager to add the media item to the database.
3.  The manager fetches metadata from a metadata provider and adds the media item to the database with a "pending" status.
4.  The scheduler runs a task to process pending media, which adds the media item to the search queue.
5.  The search queue worker searches the configured indexers for a suitable download.
6.  The torrent selector filters and scores the search results and selects the best torrent.
7.  The manager sends the selected torrent to the download client.
8.  The scheduler runs a task to update the download status, which updates the progress in the database.
9.  Once the download is complete, the post-processor moves and renames the files.
10. The scheduler runs a task to clean up completed torrents from the download client.