# Anime

## How Reel Handles Anime

Reel supports anime with dedicated metadata providers, naming conventions, and indexer sources.

1. **Add** -- Search from the UI. Metadata comes from the providers in `anime.providers`, in order (TVmaze, then AniList, by default; no API keys needed).
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
  providers: ["tvmaze", "anilist"]
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

| Provider | Description | API Key Required |
|----------|-------------|-----------------|
| `tvmaze` | One entry per show with every season, episode titles and air dates, plus IMDb and TVDB IDs | No |
| `anilist` | One entry per season, numbered episodes without titles | No |
| `kitsu` | One entry per season, with episode titles | No |
| `tmdb` | Every season, episode titles, IMDb and TVDB IDs | Yes |

!!! tip
    Put `tvmaze` first. AniList and Kitsu list each season of an anime as a separate title, so tracking the show from AniList only gets its first season (for Frieren: season 1's 28 episodes, no season 2), with numbered episodes and no IMDb ID for subtitle searches. Keep `anilist` second as a fallback for titles TVmaze does not have.

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
