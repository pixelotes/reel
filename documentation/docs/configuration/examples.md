# Configuration Examples

This page provides complete, copy-pasteable `config.yml` files for common Reel setups. Each example is a valid, standalone configuration -- just fill in your API keys and paths.

## Minimal - Movies Only with Transmission

The simplest possible Reel setup. Downloads movies using Transmission and a single Scarf indexer, with TMDB for metadata. No subtitles, no notifications, no custom pipeline stages.

```yaml
# Minimal Reel configuration - Movies only
# Replace API keys and paths with your own values.

app:
  port: 8081
  data_path: "./data"
  ui_enabled: true
  ui_password: "changeme"
  jwt_secret: "replace-with-a-long-random-string"

torrent_client:
  type: "transmission"
  host: "localhost:9091"
  # username: ""   # Uncomment if Transmission requires auth
  # password: ""
  download_path: "/downloads/media"

metadata:
  language: "en"
  tmdb:
    api_key: "your_tmdb_api_key"

movies:
  providers: ["tmdb"]
  download_folder: "/downloads/movies"
  destination_folder: "/media/movies"
  move_method: ["move"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/movies"
      api_key: "your_scarf_api_key"

database:
  path: "./data/reel.db"

automation:
  search_interval: "1h"
  quality_preferences:
    - "1080p"
    - "720p"
  min_seeders: 5
```

!!! tip
    This is a great starting point. Once everything works, add TV shows, subtitles,
    or notifications by copying the relevant sections from the full example below.

---

## Full Setup - Movies + TV + Anime

Everything enabled: all three media types with Scarf indexers, TMDB + TVmaze + AniList for metadata, Telegram notifications, OpenSubtitles, custom file renaming templates, the full postprocessing pipeline with every stage, and automation tuned for regular use.

```yaml
# Full Reel configuration - Movies, TV Shows, and Anime
# All features enabled. Replace every "your_*" value with real credentials.

app:
  port: 8081
  data_path: "./data"
  ui_enabled: true
  ui_password: "a-strong-password"
  debug: false
  jwt_secret: "replace-with-a-long-random-string"
  magnet_to_torrent_enabled: true
  magnet_to_torrent_timeout: 60
  search_timeout: 120
  filter_log_level: "detail"

torrent_client:
  type: "transmission"
  host: "localhost:9091"
  username: ""
  password: ""
  download_path: "/downloads/media"

notifications:
  telegram:
    bot_token: "your_telegram_bot_token"
    chat_id: "your_telegram_chat_id"

extra_trackers_list:
  - "udp://open.stealth.si:80/announce"
  - "udp://tracker.opentrackr.org:1337/announce"

metadata:
  language: "en"
  timeout: 15
  tmdb:
    api_key: "your_tmdb_api_key"
  tvmaze:
    api_key: ""        # TVmaze works without a key for basic use
  anilist: {}          # AniList public API, no key needed
  trakt:
    client_id: "your_trakt_client_id"

movies:
  providers: ["tmdb"]
  download_folder: "/downloads/movies"
  destination_folder: "/media/movies"
  move_method: ["hardlink", "move"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/movies"
      api_key: "your_scarf_api_key"

tv-shows:
  providers: ["tvmaze", "trakt"]
  download_folder: "/downloads/shows"
  destination_folder: "/media/shows"
  move_method: ["hardlink", "move"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/tv"
      api_key: "your_scarf_api_key"

anime:
  providers: ["anilist"]
  download_folder: "/downloads/anime"
  destination_folder: "/media/anime"
  move_method: ["hardlink", "move"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/anime"
      api_key: "your_scarf_api_key"

subtitles:
  enabled: true
  api_key: "your_opensubtitles_api_key"
  languages: ["en", "es"]

file_renaming:
  movie_template: "{title} ({year}) [{quality}]"
  series_template: "{title} - S{season}E{episode} [{quality}]"
  anime_template: "{title} - {season}x{episode} [{quality}]"

postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 50
  wait_for_file_timeout: 60
  retry_attempts: 5
  retry_delay: 10
  cleanup_on_failure: false
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

database:
  path: "./data/reel.db"

automation:
  search_interval: "30m"
  episode_download_delay_hours: 8
  max_concurrent_downloads: 3
  quality_preferences:
    - "1080p"
    - "720p"
  min_seeders: 5
  keep_torrents_for_days: 7
  keep_torrents_seed_ratio: 1.2
  notifications: ["telegram"]
  reject-common:
    - \bscreener\b
    - \bhdcam\b
    - \btelecine\b
```

!!! warning
    Keep your `jwt_secret` private and unique. Anyone with this value can forge
    authentication tokens for the web UI.

---

## Prowlarr as Central Indexer

Uses Prowlarr to manage all indexers in one place instead of configuring Scarf or Jackett sources individually. Paired with qBittorrent as the torrent client. This is ideal when you already run Prowlarr in your *arr stack.

```yaml
# Prowlarr + qBittorrent configuration
# Point each media type at your Prowlarr instance. Prowlarr handles the indexers.

app:
  port: 8081
  data_path: "./data"
  ui_enabled: true
  ui_password: "changeme"
  jwt_secret: "replace-with-a-long-random-string"
  search_timeout: 120

torrent_client:
  type: "qbittorrent"
  host: "localhost:8080"
  username: "admin"
  password: "your_qbittorrent_password"
  download_path: "/downloads/media"

metadata:
  language: "en"
  timeout: 10
  tmdb:
    api_key: "your_tmdb_api_key"
  tvmaze:
    api_key: ""

movies:
  providers: ["tmdb"]
  download_folder: "/downloads/movies"
  destination_folder: "/media/movies"
  move_method: ["hardlink", "move"]
  sources:
    # Prowlarr exposes each indexer as a Torznab feed.
    # You can add multiple Prowlarr sources if you split indexers by category.
    - type: "prowlarr"
      url: "http://prowlarr:9696"
      api_key: "your_prowlarr_api_key"

tv-shows:
  providers: ["tvmaze"]
  download_folder: "/downloads/shows"
  destination_folder: "/media/shows"
  move_method: ["hardlink", "move"]
  sources:
    - type: "prowlarr"
      url: "http://prowlarr:9696"
      api_key: "your_prowlarr_api_key"

subtitles:
  enabled: true
  api_key: "your_opensubtitles_api_key"
  languages: ["en"]

file_renaming:
  movie_template: "{title} ({year}) [{quality}]"
  series_template: "{title} - S{season}E{episode} [{quality}]"

postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 10
  pipeline:
    enabled: true
    rollback: true

database:
  path: "./data/reel.db"

automation:
  search_interval: "1h"
  episode_download_delay_hours: 4
  max_concurrent_downloads: 5
  quality_preferences:
    - "1080p"
    - "720p"
    - "2160p"
  min_seeders: 3
  keep_torrents_for_days: 7
  keep_torrents_seed_ratio: 1.0
```

!!! tip
    Make sure the Prowlarr API key matches the one shown in Prowlarr under
    **Settings > General > Security > API Key**. Each media type can use the same
    Prowlarr URL and key -- Prowlarr routes searches to the correct indexers based
    on the query categories.

---

## NAS with Hardlinks

Optimized for a NAS (Synology, TrueNAS, Unraid, etc.) where the download directory and the media library live on the same filesystem. Hardlinks are the primary move method so completed media appears in the library instantly without using extra disk space. Deluge is the torrent client.

```yaml
# NAS-optimized configuration with hardlinks
# IMPORTANT: download_folder and destination_folder MUST be on the same filesystem
# for hardlinks to work. If they are on different volumes, change move_method to ["copy"].

app:
  port: 8081
  data_path: "/volume1/docker/reel/data"
  ui_enabled: true
  ui_password: "changeme"
  jwt_secret: "replace-with-a-long-random-string"

torrent_client:
  type: "deluge"
  host: "localhost:8112"
  username: ""
  password: "your_deluge_password"
  download_path: "/volume1/downloads"

notifications:
  telegram:
    bot_token: "your_telegram_bot_token"
    chat_id: "your_telegram_chat_id"

metadata:
  language: "en"
  timeout: 10
  tmdb:
    api_key: "your_tmdb_api_key"
  tvmaze:
    api_key: ""
  anilist: {}

# All folders are on /volume1 so hardlinks work across them.
movies:
  providers: ["tmdb"]
  download_folder: "/volume1/downloads/movies"
  destination_folder: "/volume1/media/movies"
  move_method: ["hardlink"]    # Single method -- hardlink only
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/movies"
      api_key: "your_scarf_api_key"

tv-shows:
  providers: ["tvmaze"]
  download_folder: "/volume1/downloads/shows"
  destination_folder: "/volume1/media/shows"
  move_method: ["hardlink"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/tv"
      api_key: "your_scarf_api_key"

anime:
  providers: ["anilist"]
  download_folder: "/volume1/downloads/anime"
  destination_folder: "/volume1/media/anime"
  move_method: ["hardlink"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/anime"
      api_key: "your_scarf_api_key"

subtitles:
  enabled: true
  api_key: "your_opensubtitles_api_key"
  languages: ["en"]

file_renaming:
  movie_template: "{title} ({year}) [{quality}]"
  series_template: "{title} - S{season}E{episode} [{quality}]"
  anime_template: "{title} - {season}x{episode} [{quality}]"

postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 50
  wait_for_file_timeout: 60
  retry_attempts: 3
  retry_delay: 5
  cleanup_on_failure: false
  pipeline:
    enabled: true
    rollback: true
    stages:
      - name: permission_check
        enabled: true
      - name: space_check
        enabled: true
      - name: validate
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

database:
  path: "/volume1/docker/reel/data/reel.db"

automation:
  search_interval: "1h"
  episode_download_delay_hours: 6
  max_concurrent_downloads: 3
  quality_preferences:
    - "1080p"
    - "720p"
  min_seeders: 5
  keep_torrents_for_days: 14
  keep_torrents_seed_ratio: 2.0     # Seed longer on a NAS -- it's always on
  notifications: ["telegram"]
```

!!! warning
    Hardlinks only work when the source and destination are on the **same filesystem
    (volume)**. If your downloads go to `/volume1` but media lives on `/volume2`,
    hardlinks will fail silently and Reel will not fall back automatically unless you
    add a second method: `move_method: ["hardlink", "copy"]`.

!!! tip
    On a NAS you can seed indefinitely without wasting space. Set
    `keep_torrents_seed_ratio` to a high value (or remove it) to keep seeding
    completed torrents while the hardlinked files are already organized in your
    media library.

---

## Raspberry Pi - Conservative

A low-resource configuration for a Raspberry Pi 3 or similar constrained hardware. Longer search intervals, a single concurrent download, minimal metadata providers, and no subtitle downloads to keep CPU and memory usage down.

```yaml
# Raspberry Pi 3 / low-resource configuration
# Tuned for minimal CPU and memory usage.

app:
  port: 8081
  data_path: "/home/pi/reel/data"
  ui_enabled: true
  ui_password: "changeme"
  debug: false
  jwt_secret: "replace-with-a-long-random-string"
  magnet_to_torrent_enabled: false    # Save CPU -- skip magnet conversion
  search_timeout: 180                 # Allow extra time on slow hardware

torrent_client:
  type: "transmission"
  host: "localhost:9091"
  download_path: "/home/pi/downloads"

metadata:
  language: "en"
  timeout: 20                         # Generous timeout for slow connections
  tmdb:
    api_key: "your_tmdb_api_key"

movies:
  providers: ["tmdb"]
  download_folder: "/home/pi/downloads/movies"
  destination_folder: "/home/pi/media/movies"
  move_method: ["move"]               # Move instead of hardlink to free download space
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/movies"
      api_key: "your_scarf_api_key"

tv-shows:
  providers: ["tvmaze"]
  download_folder: "/home/pi/downloads/shows"
  destination_folder: "/home/pi/media/shows"
  move_method: ["move"]
  sources:
    - type: "scarf"
      url: "http://localhost:8080/torznab/tv"
      api_key: "your_scarf_api_key"

# No anime section -- keep it simple.
# No subtitles -- saves API calls and disk I/O.

file_renaming:
  movie_template: "{title} ({year})"
  series_template: "{title} - S{season}E{episode}"

postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 10
  wait_for_file_timeout: 60           # SD cards can be slow
  retry_attempts: 2
  retry_delay: 10
  pipeline:
    enabled: true
    rollback: true
    # Minimal pipeline -- only the essentials
    stages:
      - name: validate
        enabled: true
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true

database:
  path: "/home/pi/reel/data/reel.db"

automation:
  search_interval: "4h"                  # Search much less frequently
  episode_download_delay_hours: 24       # Wait a full day for better-seeded releases
  max_concurrent_downloads: 1            # One download at a time
  quality_preferences:
    - "720p"                             # Prefer 720p to save bandwidth and space
  min_seeders: 10                        # Only well-seeded torrents
  keep_torrents_for_days: 3              # Clean up quickly to save disk space
  keep_torrents_seed_ratio: 1.0
```

!!! tip
    If you are running Reel on a Raspberry Pi with an external USB drive, make sure
    both `download_folder` and `destination_folder` point to that drive. Writing to
    the SD card for large media files will wear it out quickly.

!!! warning
    Avoid enabling the `health_check`, `extract`, or `enrich_metadata` pipeline
    stages on a Raspberry Pi. These are CPU-intensive and can cause the device to
    become unresponsive during postprocessing.
