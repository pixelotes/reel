# Quick Start

This guide gets you running with a minimal configuration: movies only, using Transmission and Scarf.

## Minimal config.yml

```yaml
app:
  port: 8081
  data_path: "./data"
  ui_enabled: true
  ui_password: "changeme"
  jwt_secret: "generate-a-random-string-here"

torrent_client:
  type: "transmission"
  host: "transmission:9091"
  download_path: "/downloads/media"

metadata:
  tmdb:
    api_key: "your_tmdb_api_key"

movies:
  providers: ["tmdb"]
  download_folder: "/downloads/movies"
  destination_folder: "/media/movies"
  move_method: ["hardlink", "copy"]
  sources:
    - type: "scarf"
      url: "http://scarf:8080/torznab/movies"
      api_key: "your_scarf_api_key"

tv-shows:
  providers: ["tvmaze"]
  download_folder: "/downloads/shows"
  destination_folder: "/media/shows"
  move_method: ["hardlink", "copy"]
  sources:
    - type: "scarf"
      url: "http://scarf:8080/torznab/tv"
      api_key: "your_scarf_api_key"

anime:
  providers: ["anilist"]
  download_folder: "/downloads/anime"
  destination_folder: "/media/anime"
  move_method: ["hardlink", "copy"]
  sources:
    - type: "scarf"
      url: "http://scarf:8080/torznab/anime"
      api_key: "your_scarf_api_key"

database:
  path: "./data/reel.db"

automation:
  search_interval: "1h"
  quality_preferences: ["1080p", "720p"]
  min_seeders: 3

postprocessing:
  enabled: true
  pipeline:
    enabled: true
```

## First steps

1. Place `config.yml` in your config volume (`./reel/config/config.yml`)
2. Start the stack: `docker compose up -d`
3. Open `http://your-host:8081` in your browser
4. Log in with the password you set in `ui_password`
5. Add a movie or TV show from the search page

## What happens next

When you add media, Reel will:

1. Search your configured indexers for matching torrents
2. Pick the best match based on quality preferences and seeders
3. Send it to your torrent client
4. Monitor download progress
5. Once complete, run the post-processing pipeline (move, rename, notify)

## Getting a TMDB API key

1. Create an account at [themoviedb.org](https://www.themoviedb.org/)
2. Go to Settings > API
3. Request an API key (free for personal use)
4. Copy the "API Key (v3 auth)" into your config
