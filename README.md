# Reel

Reel is a lightweight, self-hosted automation tool that finds, downloads and files movies, TV shows and anime (plus public-domain ebooks).

Part of [MangoStack](https://github.com/pixelotes/MangoStack), a self-hosted media stack for the Raspberry Pi.

## Footprint

- Memory: about 16 MB RSS idle, observed inside MangoStack (container limit 64 MB, web UI enabled, scheduler running, `storm` client, a few tracked titles). An older figure of "under 10 MB" predates the current feature set and needs re-measuring.
- Things that raise memory and CPU: searches and indexer result parsing (every `search_interval`), the post-processing pipeline while it runs (archive extraction calls `unrar`), and in-browser video playback, which spawns `ffmpeg` to remux non-MP4 files on the fly (video copied, audio transcoded to stereo AAC).
- Disk writes:
  - A SQLite database at `database.path` (default `./data/reel.db`) and processing state in `app.data_path`.
  - The config file, when it is saved from the web UI.
  - Library files: completed downloads are placed in each media type's `destination_folder` using the first working method from `move_method` (`hardlink`, `symlink`, `move`, `copy`). Hardlinks and symlinks cost no extra space; `copy` duplicates the data.
  - Subtitles next to the media files, and metadata files written by the `save_metadata` stage.

Compared with Sonarr + Radarr (.NET, typically 100+ MB each), Reel covers the core movie/TV/anime workflow in a single Go binary, with far fewer features.

## Features

- Media types: movies, TV shows, anime, and ebooks (Project Gutenberg via Gutendex; optional, only validated if configured). No manga or music.
- Torrent clients (`torrent_client.type`):
  - `storm`: REST API shared by Storm and [Tango](https://github.com/pixelotes/tango) (web UI/REST wrapper around aria2c). Used by MangoStack.
  - `transmission`, `qbittorrent`, `aria2` (JSON-RPC with `secret`), `deluge`, `rain`.
  - `direct` (plain HTTP downloads, used for ebooks) and `mock` (testing).
- Indexers (`sources[].type`): `scarf`, `jackett`, `prowlarr` (Torznab/Newznab), `rss` feeds, and `gutendex` for ebooks. `search_mode` selects the Torznab search type (`search`, `movie-search`, `tv-search`).
- Metadata providers (per media type, tried in order), from [Scout](https://github.com/pixelotes/scout):
  - `tmdb` (needs an API key)
  - `imdb` (keyless: IMDb suggestions + Cinemeta; good fallback for movies)
  - `tvmaze`, `anilist`, `kitsu` (keyless), `trakt` (needs `client_id`)
  - `googlebooks`, `openlibrary`, `gutendex` for ebooks (built into Reel)
- Lookups run inside Reel or against a shared Scout service (`crawlers.backends`), one as fallback for the other.
- Torrent selection: quality preferences, minimum seeders, regex rejection list (`reject-common`), optional magnet-to-.torrent conversion (bundled `magnet2torrent` binary).
- Extra trackers appended to every added torrent (`extra_trackers_list`).
- Post-processing pipeline with configurable, conditional stages and rollback on failure. Default stages: `validate`, `create_folders`, `move_files`, `rename`, `save_metadata`, `subtitles`, `notify`. Extra stages available: `permission_check`, `space_check`, `extract`, `health_check`, `enrich_metadata`, `duplicate_check`.
- File renaming templates per media type.
- Subtitles via Scout from OpenSubtitles, SubDL and Subdivx (Spanish, through a FlareSolverr-compatible solver such as Parasol), searched together: OpenSubtitles by file hash, then IMDb/TMDB ID, then title; the best candidate per language (hash match, trusted, most downloaded). Downloaded files are cached in `data/subtitle-cache`, and the periodic scan for missing subtitles stops once the daily quota is used up.
- TV: new-episode checks, per-episode search/download, configurable delay after air date, custom anime search terms.
- Calendar of upcoming episode air dates.
- Cleanup of completed torrents by age (`keep_torrents_for_days`) or seed ratio (`keep_torrents_seed_ratio`).
- Notifications: Pushbullet and Telegram.
- Web UI (single password, JWT): library, add media, manual search, calendar, live logs over WebSocket, in-browser video player and EPUB reader, and a YAML config editor that saves and reloads the config.

## Quick start

Image: `pixelotes/reel:arm64`. The container listens on port 8081, runs as a non-root user, and reads `/app/config/config.yml`.

```yaml
services:
  reel:
    image: pixelotes/reel:arm64
    container_name: reel
    restart: unless-stopped
    ports:
      - "8081:8081"
    volumes:
      - ./reel/config:/app/config   # config.yml goes here
      - ./reel/data:/app/data       # SQLite DB and state
      - /path/to/storage:/storage   # downloads and library (same filesystem for hardlinks)
    environment:
      - TZ=Europe/Madrid
```

Downloads and library folders must be on the same filesystem, and visible at the same paths to Reel and the torrent client, for hardlinks to work.

Inside MangoStack, Reel is already wired in `docker-compose.yaml`, talking to Scarf and Tango on the internal network.

## Configuration

Reel reads a single YAML file (`-config`; in the image `/app/config/config.yml`). There is no environment variable or `${VAR}` substitution. Required: `app.jwt_secret`, `torrent_client.type`, `torrent_client.host`, `database.path`, and `download_folder`, `destination_folder`, `move_method` for movies, tv-shows and anime.

```yaml
app:
  port: 8081                     # default 8081
  data_path: "./data"            # default ./data
  ui_enabled: true
  ui_password: "change-me"
  jwt_secret: "long-random-string"   # openssl rand -hex 48
  debug: false
  search_timeout: 60             # seconds, default 30
  filter_log_level: "detail"     # "none" or "detail"
  magnet_to_torrent_enabled: false
  magnet_to_torrent_timeout: 60

torrent_client:
  type: "storm"                  # transmission, qbittorrent, aria2, rain, storm, deluge, direct, mock
  host: "http://tango:7246"
  username: ""
  password: ""
  secret: ""                     # aria2 RPC secret
  download_path: "/storage/Downloads/complete"

database:
  path: "./data/reel.db"

# Where metadata and subtitle lookups run, via scout
# (https://github.com/pixelotes/scout). "local" runs the providers inside
# Reel; "service" asks a scout server. With both, the second is only used
# when the first fails (down, timeout), never when it answers "not found".
crawlers:
  backends: ["local"]            # default ["local"]; e.g. ["service", "local"]
  service:
    url: ""                      # e.g. http://scout:8090
    token: ""                    # scout's server.token, if set
    timeout: 10                  # seconds, default 10

metadata:
  language: "en"                 # default en
  timeout: 10                    # seconds, default 10
  tmdb:
    api_key: ""
  trakt:
    client_id: ""
  google_books:
    api_key: ""

movies:
  providers: ["tmdb", "imdb"]    # imdb is keyless and used when tmdb fails or finds nothing
  download_folder: "/storage/Downloads/Movies"
  destination_folder: "/storage/Completed/Movies"
  move_method: ["hardlink", "symlink", "move", "copy"]
  sources:
    - type: "scarf"              # scarf, jackett, prowlarr, rss
      url: "http://scarf:8080/torznab/all"
      api_key: ""
      search_mode: movie-search

tv-shows:
  providers: ["tvmaze"]
  download_folder: "/storage/Downloads/Series"
  destination_folder: "/storage/Completed/TV"
  move_method: ["hardlink", "symlink", "move", "copy"]
  sources:
    - type: "rss"
      url: "https://example.com/your-private-rss"
    - type: "scarf"
      url: "http://scarf:8080/torznab/all"
      api_key: ""
      search_mode: search

anime:
  providers: ["tvmaze", "anilist"]   # tvmaze: every season, episode titles, IMDb ID
  download_folder: "/storage/Downloads/Anime"
  destination_folder: "/storage/Completed/Anime"
  move_method: ["hardlink", "symlink", "move", "copy"]
  sources:
    - type: "scarf"
      url: "http://scarf:8080/torznab/all"
      api_key: ""
      search_mode: tv-search

# Optional
ebooks:
  providers: ["gutendex"]
  download_folder: "/storage/Downloads/Ebooks"
  destination_folder: "/storage/Completed/Ebooks"
  move_method: ["move", "copy"]
  sources:
    - type: "gutendex"
      url: "https://gutendex.com"

file_renaming:
  movie_template: "{title} ({year}) [{quality}]"
  series_template: "{title} - S{season}E{episode} [{quality}]"
  anime_template: "{title} - {season}x{episode} [{quality}]"
  ebook_template: "{title} ({year})"

postprocessing:
  enabled: true
  validate_files: true
  min_file_size_mb: 1            # default 1
  wait_for_file_timeout: 30      # seconds, default 30
  retry_attempts: 3              # default 3
  retry_delay: 5                 # seconds, default 5
  cleanup_on_failure: true
  keep_failed_downloads_days: 7
  pipeline:
    enabled: true
    rollback: true
    # stages: omit to use the defaults listed under Features
    # - name: duplicate_check
    #   enabled: true
    #   condition: "is_movie"

automation:
  # Cron specs (robfig/cron): "@every 30m", "0 */2 * * *", ...
  search_interval: "@every 30m"            # default
  new_episodes_check_interval: "@every 6h" # default
  download_status_interval: "@every 10s"   # default
  rss_processing_interval: "@every 1h"     # default
  cleanup_interval: "@every 24h"           # default
  retry_failed_interval: "@every 1h"       # default
  subtitle_scan_interval: "@every 12h"     # default
  max_concurrent_downloads: 3
  quality_preferences: ["1080p", "720p"]
  min_seeders: 5
  keep_torrents_for_days: 7
  keep_torrents_seed_ratio: 1.0
  episode_download_delay_hours: 8
  notifications: ["pushbullet"]  # pushbullet, telegram
  reject-common:
    - \bhdcam?\b
    - \bscreener\b

extra_trackers_list:
  - udp://tracker.example.org:1337/announce

notifications:
  pushbullet:
    api_key: ""
  telegram:
    bot_token: ""
    chat_id: ""

subtitles:
  enabled: true
  api_key: ""                    # OpenSubtitles
  subdl_api_key: ""              # SubDL (free key from your subdl.com profile)
  flaresolverr_url: ""           # enables Subdivx (Spanish), e.g. http://parasol:8191
  # providers: ["opensubtitles", "subdl", "subdivx"]   # default; each needs its key above
  languages: ["en", "es"]
```

More detail (pipeline conditions, stages, recovery, providers) lives in `documentation/` (MkDocs).

## Usage

- Web UI: `http://<host>:8081` when `app.ui_enabled` is true; log in with `app.ui_password`.
- REST API under `/api/v1` (JWT from `POST /api/v1/login`), including:
  - `GET/POST /media`, `DELETE /media/{id}`, `POST /media/{id}/retry`
  - `GET /media/{id}/search`, `POST /media/{id}/download` (manual search and grab)
  - episode search/download under `/media/{id}/season/{s}/episode/{e}/...`
  - `GET /search-metadata`, `GET /metadata-details` (episode list for one title), `GET /calendar`, `GET /status`
  - `GET /test/indexer`, `GET /test/torrent`
  - `GET/POST /config` (read, save and reload the YAML config)
  - `/stream/video/{id}`, `/stream/subtitles/{id}`, `/stream/ebook/{id}`
  - `/logs/ws` (live log WebSocket)

## Building from source

```bash
git clone https://github.com/pixelotes/reel.git
cd reel
go build -o reel .
go build -o magnet2torrent ./cmd/magnet2torrent
./reel -config ./config/config.yml
```

Optional runtime tools: `ffmpeg` (video remuxing in the player), `unrar` (`extract` stage), `magnet2torrent` on `PATH` (magnet conversion). The Docker image ships `ffmpeg` and `magnet2torrent`.

## Limits

- Single user, single password; no multi-user support.
- Far fewer features than Sonarr/Radarr (no automatic quality upgrades).
- `qbittorrent` and `deluge` clients have had little testing.
- Config is YAML only; no env var substitution, secrets live in the file.
- The log WebSocket accepts any origin. Do not expose Reel to the internet; it assumes a trusted LAN.

## Part of MangoStack

| App | Role |
|---|---|
| [Scarf](https://github.com/pixelotes/scarf) | Indexer proxy (Torznab) |
| [Parasol](https://github.com/pixelotes/parasol) | Optional Cloudflare solver for Scarf |
| [Reel](https://github.com/pixelotes/reel) | Movie & TV automation |
| [Tango](https://github.com/pixelotes/tango) | Web UI and REST API for aria2c |
| [RMS](https://github.com/pixelotes/rms) | Media server (Jellyfin API) |
| [Suika](https://github.com/pixelotes/suika) | Manga reader |
| [Kaze](https://github.com/pixelotes/kaze) | Music server (Subsonic API) |
| [Sumi](https://github.com/pixelotes/sumi) | EPUB reader |
| [Dashboarr](https://github.com/pixelotes/dashboarr) | Homepage |

## License

MIT
