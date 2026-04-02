# Configuration Reference

This page documents every configuration option available in Reel. The configuration file uses YAML format and is typically located at `config.yml` in your working directory.

---

## App

General application settings.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `app.port` | int | `8081` | Port the web UI listens on. |
| `app.data_path` | string | `./data` | Directory used for logs, state files, and the database. |
| `app.ui_enabled` | bool | `false` | Enable the built-in web UI. |
| `app.ui_password` | string | | Password required to access the web UI. |
| `app.debug` | bool | `false` | Enable debug-level logging. Produces verbose output. |
| `app.jwt_secret` | string | | **Required.** Secret key used to sign JWT tokens for web UI authentication. |
| `app.filter_log_level` | string | `"none"` | Controls logging of filter decisions. Valid values: `none`, `detail`. |
| `app.magnet_to_torrent_enabled` | bool | `false` | When enabled, magnet links are converted to `.torrent` files before being sent to the torrent client. |
| `app.magnet_to_torrent_timeout` | int | | Timeout in seconds for the magnet-to-torrent conversion process. |
| `app.search_timeout` | int | `30` | Timeout in seconds for indexer search requests. |

!!! warning
    `app.jwt_secret` must be set to a strong, random value. Without it, the web UI authentication will not function.

---

## Torrent Client

Connection settings for the download client Reel manages.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `torrent_client.type` | string | | **Required.** The torrent client to use. Valid values: `transmission`, `qbittorrent`, `aria2`, `deluge`, `mock`. |
| `torrent_client.host` | string | | **Required.** Host and port (or full URL) for the torrent client API. Example: `http://localhost:9091`. |
| `torrent_client.username` | string | | Username for authentication. Used by `transmission`, `qbittorrent`, and `deluge`. |
| `torrent_client.password` | string | | Password for authentication. |
| `torrent_client.secret` | string | | Authentication token. Used by `aria2` only (the RPC secret). |
| `torrent_client.download_path` | string | | Directory where the torrent client saves downloaded files. |

!!! note
    The `mock` client type is intended for development and testing only. It simulates downloads without actually downloading anything.

---

## Metadata

Settings for metadata providers used to look up movie, TV show, and anime information.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `metadata.language` | string | `en` | ISO 639-1 language code for metadata results (e.g., `en`, `es`, `de`, `ja`). |
| `metadata.timeout` | int | `10` | HTTP timeout in seconds for metadata API requests. |

### TMDB

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `metadata.tmdb.api_key` | string | | TMDB API key (v3). Obtain one at [themoviedb.org](https://www.themoviedb.org/settings/api). |

### IMDB

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `metadata.imdb.api_key` | string | | Placeholder field. Not currently used by Reel. |

### TVmaze

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `metadata.tvmaze.api_key` | string | | API key for TVmaze. Not required for basic use as TVmaze offers a free public API. |

### AniList

AniList uses a public GraphQL API and requires no configuration.

### Trakt

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `metadata.trakt.client_id` | string | | Trakt client ID. Create an application at [trakt.tv/oauth/applications](https://trakt.tv/oauth/applications) to obtain one. |

---

## Subtitles

Configure automatic subtitle downloading via OpenSubtitles.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `subtitles.enabled` | bool | `false` | Enable automatic subtitle downloads. |
| `subtitles.api_key` | string | | OpenSubtitles API key. **Required if subtitles are enabled.** |
| `subtitles.languages` | []string | | List of language codes to download subtitles for, e.g., `["en", "es"]`. |
| `subtitles.download_path` | string | | Optional override path where subtitle files are saved. If empty, subtitles are saved alongside the media file. |

---

## Movies

Configuration for movie discovery, downloading, and organization.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `movies.providers` | []string | | Metadata providers to use for movies. Valid values: `tmdb`, `imdb`. |
| `movies.sources` | []SourceConfig | | List of indexer sources to search for movie torrents. See [Source Configuration](#source-configuration) below. |
| `movies.download_folder` | string | | **Required.** Directory where the torrent client downloads movie files. |
| `movies.destination_folder` | string | | **Required.** Final library directory where movies are organized after processing. |
| `movies.move_method` | []string | | **Required.** Ordered list of methods to try when moving files from download to destination. Valid values: `hardlink`, `symlink`, `move`, `copy`. Reel tries each method in order, falling back to the next on failure. |

---

## TV Shows

Configuration for TV show discovery, downloading, and organization. The structure is identical to [Movies](#movies).

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `tvshows.providers` | []string | | Metadata providers to use for TV shows. |
| `tvshows.sources` | []SourceConfig | | List of indexer sources for TV show torrents. See [Source Configuration](#source-configuration). |
| `tvshows.download_folder` | string | | **Required.** Directory where the torrent client downloads TV show files. |
| `tvshows.destination_folder` | string | | **Required.** Final library directory for organized TV shows. |
| `tvshows.move_method` | []string | | **Required.** Ordered list of file move methods. Valid values: `hardlink`, `symlink`, `move`, `copy`. |

---

## Anime

Configuration for anime discovery, downloading, and organization. The structure is identical to [Movies](#movies).

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `anime.providers` | []string | | Metadata providers to use for anime. |
| `anime.sources` | []SourceConfig | | List of indexer sources for anime torrents. See [Source Configuration](#source-configuration). |
| `anime.download_folder` | string | | **Required.** Directory where the torrent client downloads anime files. |
| `anime.destination_folder` | string | | **Required.** Final library directory for organized anime. |
| `anime.move_method` | []string | | **Required.** Ordered list of file move methods. Valid values: `hardlink`, `symlink`, `move`, `copy`. |

---

## Database

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `database.path` | string | | **Required.** Path to the SQLite database file. Example: `./data/reel.db`. |

---

## Notifications

Configure notification services to receive alerts about downloads, errors, and other events.

### Pushbullet

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `notifications.pushbullet.api_key` | string | | Pushbullet API key. Obtain one at [pushbullet.com/#settings](https://www.pushbullet.com/#settings). |

### Telegram

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `notifications.telegram.bot_token` | string | | Telegram bot token obtained from [@BotFather](https://t.me/BotFather). |
| `notifications.telegram.chat_id` | string | | Telegram chat or group ID where notifications are sent. |

!!! note
    To find your Telegram chat ID, send a message to your bot and visit `https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates`.

---

## Automation

Controls scheduling, quality filtering, and automatic behavior.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `automation.search_interval` | string | `@every 30m` | Cron expression or interval for running searches. |
| `automation.rss_processing_interval` | string | `@every 1h` | Cron expression or interval for processing RSS feeds. |
| `automation.download_status_interval` | string | `@every 10s` | Cron expression or interval for checking download status. |
| `automation.new_episodes_check_interval` | string | `@every 6h` | Cron expression or interval for checking for newly aired episodes. |
| `automation.cleanup_interval` | string | `@every 24h` | Cron expression or interval for cleanup tasks (removing completed torrents, etc.). |
| `automation.retry_failed_interval` | string | `@every 1h` | Cron expression or interval for retrying failed downloads. |
| `automation.max_concurrent_downloads` | int | | Maximum number of parallel downloads allowed. |
| `automation.quality_preferences` | []string | | Ordered list of preferred quality levels. The first entry is the most preferred. Example: `["1080p", "720p", "2160p"]`. |
| `automation.min_seeders` | int | | Minimum number of seeders required to accept a torrent. Torrents below this threshold are skipped. |
| `automation.keep_torrents_for_days` | int | | Number of days to keep completed torrents before automatic removal. |
| `automation.keep_torrents_seed_ratio` | float | | Seed ratio threshold. Completed torrents are removed once they reach this ratio. |
| `automation.episode_download_delay_hours` | int | | Number of hours to wait after a TV episode's air date before searching. Useful to allow higher-quality releases to appear. |
| `automation.reject-common` | []string | | List of regex patterns. Torrents matching any pattern are rejected. See also the top-level `reject-common`. |
| `automation.notifications` | []string | | List of notification services to use. Valid values: `telegram`, `pushbullet`. |

!!! note
    Interval values use Go cron syntax. Examples: `@every 30m`, `@every 1h`, `@every 24h`, `@daily`, `0 */6 * * *`.

---

## Top-Level Options

These options sit at the root level of the configuration file, outside any section.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `reject-common` | []string | | Global list of regex patterns to reject torrents. Applied in addition to `automation.reject-common`. |
| `extra_trackers_list` | []string | | List of extra tracker URLs appended to every torrent. Useful for improving peer discovery with public trackers. |

---

## File Renaming

Templates for renaming media files after download. Templates use Go template syntax with metadata variables.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `file_renaming.movie_template` | string | | Template for movie filenames. |
| `file_renaming.series_template` | string | | Template for TV show episode filenames. |
| `file_renaming.anime_template` | string | | Template for anime episode filenames. |

---

## Post-Processing

Controls what happens after a torrent finishes downloading.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `postprocessing.enabled` | bool | `false` | Enable post-processing of completed downloads. |
| `postprocessing.validate_files` | bool | `false` | Validate downloaded files (check they exist and meet size requirements). |
| `postprocessing.min_file_size_mb` | int | `1` | Minimum file size in megabytes. Files smaller than this are considered invalid. |
| `postprocessing.wait_for_file_timeout` | int | `30` | Seconds to wait for a file to appear on disk before giving up. |
| `postprocessing.retry_attempts` | int | `3` | Number of times to retry post-processing on failure. |
| `postprocessing.retry_delay` | int | `5` | Seconds to wait between retry attempts. |
| `postprocessing.cleanup_on_failure` | bool | `false` | Remove partially processed files when post-processing fails. |
| `postprocessing.keep_failed_downloads_days` | int | | Number of days to keep failed downloads before cleanup removes them. |

### Pipeline

The pipeline architecture processes downloads through a series of stages. This is the recommended approach for post-processing.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `postprocessing.pipeline.enabled` | bool | `false` | Enable the pipeline-based post-processing architecture. Recommended over the legacy approach. |
| `postprocessing.pipeline.rollback` | bool | `true` | Automatically rollback all completed stages when a stage fails. |
| `postprocessing.pipeline.stages` | []StageConfig | | Custom stage configuration. When empty, the default pipeline stages are used. See [Stage Configuration](#stage-configuration). |

---

## Source Configuration

Sources define where Reel searches for torrents. They are used inside `movies.sources`, `tvshows.sources`, and `anime.sources`.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `type` | string | | **Required.** The indexer type. Valid values: `scarf`, `jackett`, `prowlarr`, `rss`. |
| `url` | string | | **Required.** Full URL to the indexer API endpoint. |
| `api_key` | string | | API key for authenticating with the indexer. |
| `search_mode` | string | | Optional override for the search mode. Valid values: `movie-search`, `tv-search`, `search`. When omitted, the appropriate mode is selected automatically based on the media type. |

Example:

```yaml
movies:
  sources:
    - type: jackett
      url: http://localhost:9117/api/v2.0/indexers/all/results
      api_key: your-jackett-api-key
    - type: rss
      url: https://example.com/rss/movies
```

---

## Stage Configuration

Stages define the individual steps in the post-processing pipeline. They are used inside `postprocessing.pipeline.stages`.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `name` | string | | **Required.** The name identifying this pipeline stage. |
| `enabled` | bool | | Whether this stage is active. Set to `false` to skip a stage without removing it. |
| `condition` | string | | Optional condition that must be met for the stage to run. |

Example:

```yaml
postprocessing:
  pipeline:
    enabled: true
    rollback: true
    stages:
      - name: validate
        enabled: true
      - name: rename
        enabled: true
      - name: move
        enabled: true
      - name: subtitle
        enabled: true
        condition: subtitles_enabled
```
