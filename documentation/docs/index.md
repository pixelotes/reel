# Reel

**Lightweight media management for low-end devices.**

Reel is a self-hosted media management tool designed for devices like the Raspberry Pi 3+. It consumes less than 10MB of RAM and minimal CPU, making it ideal for always-on setups on low-power hardware.

## What it does

Reel automates the full lifecycle of media management:

1. **Search** - Finds torrents from your configured indexers (Scarf, Jackett, Prowlarr, RSS)
2. **Download** - Sends them to your torrent client (Transmission, qBittorrent, Aria2, Deluge)
3. **Process** - Moves, renames, and organizes files with a configurable pipeline
4. **Enrich** - Fetches metadata from TMDB, TVmaze, AniList, or Trakt
5. **Notify** - Sends alerts via Telegram or Pushbullet

## Supported media

- **Movies** - Search, download, and organize films
- **TV Shows** - Track series, auto-download new episodes
- **Anime** - Dedicated anime support with AniList/AniDB integration

## Key features

- **Web UI** with password protection
- **12-stage post-processing pipeline** with rollback and crash recovery
- **Multiple indexer support** - Scarf, Jackett, Prowlarr, RSS feeds
- **Quality filtering** - Configurable quality preferences and rejection rules
- **Subtitle downloads** via OpenSubtitles
- **File renaming** with customizable templates
- **Hardlink support** for storage-efficient media libraries
- **Calendar view** for upcoming episodes
- **ARM support** - Pre-built Docker images for ARM32v7

## Quick links

- [Installation](getting-started/installation.md) - Get Reel running in minutes
- [Quick Start](getting-started/quick-start.md) - Minimal configuration to get started
- [Configuration Reference](configuration/reference.md) - Every config option documented
- [Examples](configuration/examples.md) - Complete configs for common setups
