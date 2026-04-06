# TV Shows

## How Reel Handles TV Shows

Reel automates tracking, downloading, and organizing TV series with full season and episode management.

1. **Add** -- Search from the UI. Metadata and episode list are fetched from providers (TVmaze, Trakt).
2. **Track** -- Reel monitors for new episodes based on air dates.
3. **Search** -- Queries indexers with show name, season, and episode number.
4. **Download** -- The best match is sent to the torrent client in the configured `download_folder`.
5. **Process** -- The pipeline creates season folders, moves files, and renames using `series_template`.

## Folder Structure

Reel organizes TV shows into season subfolders automatically:

```
/media/shows/
  Breaking Bad (2008)/
    S05/
      Breaking Bad - S05E16 [1080p].mkv
```

## Configuration

```yaml
tv-shows:
  providers: ["tvmaze"]
  download_folder: "/downloads/shows"
  destination_folder: "/media/shows"
  move_method: ["hardlink", "copy"]
  sources:
    - type: "scarf"
      url: "http://scarf:8080/torznab/tv"
      api_key: "your_key"
```

### Configuration Options

| Key                  | Description                                              |
|----------------------|----------------------------------------------------------|
| `providers`          | Metadata providers for episode information               |
| `download_folder`    | Where the torrent client downloads TV show files         |
| `destination_folder` | Final organized location for TV shows                    |
| `move_method`        | How files are transferred (in order of preference)       |
| `sources`            | List of indexer sources to search for TV show torrents   |

## Metadata Providers

| Provider | Description                          | API Key Required |
|----------|--------------------------------------|-----------------|
| TVmaze   | Free TV metadata and episode data    | No              |
| Trakt    | Comprehensive tracking and metadata  | Yes (`client_id`) |

!!! tip
    TVmaze is recommended for most users since it is free and requires no API key. Trakt offers additional features like watch history syncing but requires a `client_id`.

## Episode Download Delay

The `episode_download_delay_hours` setting lets you wait after a show's air date before searching. This gives time for higher quality releases to appear.

```yaml
automation:
  episode_download_delay_hours: 6
```

!!! info
    Setting a delay of 6 hours is a good starting point. Early releases tend to be lower quality (e.g. HDTV), while proper WEB-DL encodes usually appear within a few hours of airing.
