# Anime

## How Reel Handles Anime

Reel supports anime with dedicated metadata providers, naming conventions, and indexer sources.

1. **Add** -- Search from the UI. Metadata is fetched from AniList (GraphQL API, no API key needed).
2. **Track** -- Monitors for new episodes as they air.
3. **Search** -- Queries anime-specific indexer sources.
4. **Download** -- The best match is sent to the torrent client in the configured `download_folder`.
5. **Process** -- The pipeline creates season folders and renames files using `anime_template`.

## Folder Structure

Reel organizes anime into season subfolders:

```
/media/anime/
  Attack on Titan (2013)/
    S01/
      Attack on Titan - 01x05 [720p].mkv
```

## Configuration

```yaml
anime:
  providers: ["anilist"]
  download_folder: "/downloads/anime"
  destination_folder: "/media/anime"
  move_method: ["hardlink", "copy"]
  sources:
    - type: "scarf"
      url: "http://scarf:8080/torznab/anime"
      api_key: "your_key"
```

### Configuration Options

| Key                  | Description                                              |
|----------------------|----------------------------------------------------------|
| `providers`          | Metadata providers for anime information                 |
| `download_folder`    | Where the torrent client downloads anime files           |
| `destination_folder` | Final organized location for anime                       |
| `move_method`        | How files are transferred (in order of preference)       |
| `sources`            | List of indexer sources to search for anime torrents     |

## Metadata Providers

| Provider | Description                                      | API Key Required |
|----------|--------------------------------------------------|-----------------|
| AniList  | GraphQL API, results sorted by popularity        | No              |
| AniDB    | Comprehensive anime database                     | No              |

!!! tip
    AniList is recommended for most users. It is free, requires no API key, and returns results sorted by popularity which helps surface the most relevant matches.

## Anime Naming

Anime naming can be tricky due to varying conventions across different release groups and trackers.

The default `anime_template` is:

```yaml
file_renaming:
  anime_template: "{title} - {season}x{episode} [{quality}]"
```

This produces filenames like `Attack on Titan - 01x05 [720p].mkv`.

!!! note
    You can customize the template to match your preferred naming convention. See the [File Renaming](../advanced/file-renaming.md) page for all available placeholders.
