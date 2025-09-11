# API Endpoints

Reel provides a RESTful API for managing your media library. All API endpoints are prefixed with `/api/v1`.

### Authentication

* **`POST /login`**: Authenticates a user and returns a JWT token.

### Media

* **`GET /media`**: Get a list of all media items in your library.
* **`POST /media`**: Add a new media item to your library.
* **`DELETE /media/{id}`**: Delete a media item from your library.
* **`POST /media/{id}/retry`**: Retry a failed download for a media item.
* **`GET /media/{id}/search`**: Manually search for a download for a media item.
* **`POST /media/{id}/download`**: Manually start a download for a media item.
* **`GET /media/{id}/tv-details`**: Get the details for a TV show or anime.
* **`POST /media/{id}/settings`**: Update the settings for a media item.
* **`POST /media/clear-failed`**: Clear all failed media items from your library.
* **`GET /search-metadata`**: Search for metadata for a media item.

### Episodes

* **`GET /media/{id}/season/{season}/episode/{episode}/search`**: Manually search for a download for a specific episode.
* **`POST /media/{id}/season/{season}/episode/{episode}/download`**: Manually start a download for a specific episode.
* **`GET /media/{id}/season/{season}/episode/{episode}/details`**: Get the details for a specific episode.

### Streaming

* **`GET /stream/video/{id}`**: Stream a video file.
* **`GET /stream/subtitles/{id}`**: Get the subtitles for a video file.
* **`GET /subtitles/{id}/available`**: Get a list of all available subtitles for a video file.

### System

* **`GET /status`**: Get the status of the system, including the torrent client and indexers.
* **`GET /test/indexer`**: Test the connection to an indexer.
* **`GET /test/torrent`**: Test the connection to the torrent client.
* **`GET /config`**: Get the current configuration.
* **`POST /config`**: Save and reload the configuration.

### Anime

* **`GET /media/{id}/anime-search-terms`**: Get the alternative search terms for an anime.
* **`POST /media/{id}/anime-search-terms`**: Add an alternative search term for an anime.
* **`DELETE /media/anime-search-terms/{term_id}`**: Delete an alternative search term for an anime.

### Calendar

* **`GET /calendar`**: Get the calendar of upcoming episodes.

### Logs

* **`GET /logs/ws`**: A WebSocket endpoint for streaming the application logs.