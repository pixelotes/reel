# Configuration

Reel is configured using a `config.yml` file. Here is a breakdown of all the available options:

### `app`

| Setting                      | Description                                                              |
| ---------------------------- | ------------------------------------------------------------------------ |
| `port`                       | The port to run the web server on.                                       |
| `data_path`                  | The path to the data directory, where the database and logs are stored.  |
| `ui_enabled`                 | Whether to enable the web UI.                                            |
| `ui_password`                | The password for the web UI.                                             |
| `debug`                      | Whether to enable debug logging.                                         |
| `jwt_secret`                 | The secret key for signing JWT tokens.                                   |
| `magnet_to_torrent_enabled`  | Whether to try to convert magnet links to torrent files.                 |
| `magnet_to_torrent_timeout`  | The timeout in seconds for converting magnet links.                      |
| `search_timeout`             | The timeout in seconds for searching indexers.                           |
| `filter_log_level`           | The log level for the torrent filter, can be "none" or "detail".         |

### `torrent_client`

| Setting         | Description                                                          |
| --------------- | -------------------------------------------------------------------- |
| `type`          | The type of torrent client, can be "transmission", "qbittorrent", or "aria2". |
| `host`          | The host and port of the torrent client.                             |
| `username`      | The username for the torrent client.                                 |
| `password`      | The password for the torrent client.                                 |
| `secret`        | The secret for the Aria2 torrent client.                             |
| `download_path` | The default path to download media to.                               |

### `notifications`

| Setting      | Description                                |
| ------------ | ------------------------------------------ |
| `pushbullet` | The configuration for Pushbullet notifications. |
| `pushbullet.api_key`    | The API key for Pushbullet.                |
| `telegram`   | The configuration for Telegram notifications.   |
| `telegram.bot_token`    | The Telegram bot token (from @BotFather).  |
| `telegram.chat_id`      | The Telegram chat ID to send messages to.  |

### `extra_trackers_list`

A list of extra trackers to add to new torrents.

### `metadata`

| Setting    | Description                                       |
| ---------- | ------------------------------------------------- |
| `language` | The preferred language for metadata.              |
| `timeout`  | The timeout in seconds for fetching metadata.     |
| `tmdb`     | The configuration for The Movie Database (TMDB).  |
| `imdb`     | The configuration for IMDb.                       |
| `tvmaze`   | The configuration for TVmaze.                     |
| `anilist`  | The configuration for AniList.                    |
| `trakt`    | The configuration for Trakt.                      |

### `subtitles`

| Setting         | Description                                                          |
| --------------- | -------------------------------------------------------------------- |
| `enabled`       | Whether to enable subtitle downloading.                              |
| `api_key`       | Your OpenSubtitles.com API Key.                                      |
| `languages`     | A list of language codes to download (e.g., `["en", "es"]`).         |
| `download_path` | Optional path to save subtitles. Defaults to video file directory.   |

### `movies`, `tv-shows`, `anime`

| Setting              | Description                                                              |
| -------------------- | ------------------------------------------------------------------------ |
| `providers`          | The order of preference for metadata providers.                          |
| `download_folder`    | The path to download this type of media to.                              |
| `destination_folder` | The path to move this type of media to after post-processing.            |
| `move_method`        | The method to use for post-processing, can be "hardlink", "symlink", "move", or "copy". |
| `sources`            | A list of indexer sources for this type of media.                        |

### `file_renaming`

| Setting           | Description                                    |
| ----------------- | ---------------------------------------------- |
| `movie_template`  | The template for renaming movie files.         |
| `series_template` | The template for renaming TV show files.       |
| `anime_template`  | The template for renaming anime files.         |

### `database`

| Setting | Description                    |
| ------- | ------------------------------ |
| `path`  | The path to the database file. |

### `automation`

| Setting                        | Description                                                              |
| ------------------------------ | ------------------------------------------------------------------------ |
| `search_interval`              | The interval to run the search for pending media.                        |
| `episode_download_delay_hours` | The delay in hours before downloading new episodes.                      |
| `max_concurrent_downloads`     | The maximum number of concurrent downloads.                              |
| `quality_preferences`          | The order of preference for download qualities.                          |
| `min_seeders`                  | The minimum number of seeders for a torrent to be considered.            |
| `keep_torrents_for_days`       | The number of days to keep completed torrents for.                       |
| `keep_torrents_seed_ratio`     | The seed ratio to reach before removing completed torrents.                |
| `notifications`                | A list of notification providers to use.                                 |
| `reject-common`                | A list of regular expressions to use for rejecting releases.             |

### `postprocessing`

Handles everything that happens after a download completes. See the [Postprocessing Pipeline](postprocessing_pipeline.md) documentation for a full reference with examples.

| Setting                      | Description                                                              | Default |
| ---------------------------- | ------------------------------------------------------------------------ | ------- |
| `enabled`                    | Master switch for post-processing.                                       | `false` |
| `validate_files`             | Enable file validation (extension and size checks).                      | `false` |
| `min_file_size_mb`           | Minimum video file size in MB. Files below this are ignored.             | `1`     |
| `wait_for_file_timeout`      | Seconds to wait for files to appear on disk.                             | `30`    |
| `retry_attempts`             | Number of retries for file move/link operations.                         | `3`     |
| `retry_delay`                | Seconds to wait between retries.                                         | `5`     |
| `cleanup_on_failure`         | Delete processing state after successful completion.                     | `false` |
| `keep_failed_downloads_days` | Days to retain failed download state for debugging.                      | `0`     |

### `postprocessing.pipeline`

| Setting    | Description                                                                | Default |
| ---------- | -------------------------------------------------------------------------- | ------- |
| `enabled`  | Use the pipeline architecture. When `false`, uses the legacy processor.    | `false` |
| `rollback` | Automatically rollback completed stages if a stage fails.                  | `true`  |
| `stages`   | Ordered list of stages. Empty means use the default set.                   | `[]`    |

Each entry in `stages` accepts:

| Field       | Description                                                                |
| ----------- | -------------------------------------------------------------------------- |
| `name`      | Stage name: `validate`, `create_folders`, `move_files`, `rename`, `subtitles`, `notify`, `permission_check`, `space_check`, `extract`, `health_check`, `enrich_metadata`, `duplicate_check`. |
| `enabled`   | Whether the stage is active.                                               |
| `condition` | Optional condition (e.g., `is_movie`, `file_size > 100MB`, `files > 1`).  |

## Examples

### Minimal Configuration

```yaml
app:
  port: 8081
  data_path: "./data"

torrent_client:
  type: "transmission"
  host: "localhost:9091"
  download_path: "/downloads/media"

movies:
  providers: ["tmdb"]
  download_folder: "/downloads/movies"
  destination_folder: "/media/movies"
  move_method: ["hardlink", "move"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/movies"
      api_key: "your_key"

database:
  path: "./data/reel.db"

automation:
  search_interval: "1h"
  quality_preferences: ["1080p", "720p"]
  min_seeders: 5
```

### Full Configuration with Postprocessing Pipeline

```yaml
app:
  port: 8081
  data_path: "./data"
  ui_enabled: true
  ui_password: "changeme"
  debug: false
  jwt_secret: "your-jwt-secret"
  search_timeout: 120

torrent_client:
  type: "qbittorrent"
  host: "localhost:8080"
  username: "admin"
  password: "adminpass"
  download_path: "/downloads/media"

metadata:
  language: "en"
  timeout: 10
  tmdb:
    api_key: "your_tmdb_key"

movies:
  providers: ["tmdb", "imdb"]
  download_folder: "/downloads/movies"
  destination_folder: "/media/movies"
  move_method: ["hardlink", "symlink", "move", "copy"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/movies"
      api_key: "your_key"

tv-shows:
  providers: ["tvmaze"]
  download_folder: "/downloads/shows"
  destination_folder: "/media/shows"
  move_method: ["hardlink", "move"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/tv"
      api_key: "your_key"

anime:
  providers: ["anidb"]
  download_folder: "/downloads/anime"
  destination_folder: "/media/anime"
  move_method: ["hardlink", "move"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/anime"
      api_key: "your_key"

subtitles:
  enabled: true
  api_key: "your_opensubtitles_key"
  languages: ["en", "es"]

file_renaming:
  movie_template: "{title} ({year}) [{quality}]"
  series_template: "{title} - S{season}E{episode} [{quality}]"
  anime_template: "{title} - {season}x{episode} [{quality}]"

database:
  path: "./data/reel.db"

automation:
  search_interval: "30m"
  episode_download_delay_hours: 8
  max_concurrent_downloads: 3
  quality_preferences: ["1080p", "720p"]
  min_seeders: 5
  keep_torrents_for_days: 7
  keep_torrents_seed_ratio: 1.2

postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 50
  wait_for_file_timeout: 60
  retry_attempts: 5
  retry_delay: 10
  cleanup_on_failure: true
  keep_failed_downloads_days: 14
  pipeline:
    enabled: true
    rollback: true
    stages:
      - name: permission_check
        enabled: true
      - name: space_check
        enabled: true
      - name: extract
        enabled: true
      - name: health_check
        enabled: true
      - name: validate
        enabled: true
      - name: enrich_metadata
        enabled: true
      - name: duplicate_check
        enabled: true
        condition: "is_movie"
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
      - name: subtitles
        enabled: true
      - name: notify
        enabled: true

notifications:
  pushbullet:
    api_key: "your_pushbullet_key"
```

### TV Shows with Conditional Subtitles

Only download subtitles for files larger than 500MB (skip samples/extras):

```yaml
postprocessing:
  enabled: true
  pipeline:
    enabled: true
    stages:
      - name: validate
        enabled: true
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
      - name: subtitles
        enabled: true
        condition: "file_size > 500MB"
      - name: notify
        enabled: true
```

### Simple Setup Without Pipeline

Use the legacy post-processor for a simpler setup without stages:

```yaml
postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 50
  pipeline:
    enabled: false
```