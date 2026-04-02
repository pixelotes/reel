# File Renaming

Reel renames media files after moving them to the destination. Templates are configured per media type in the `file_renaming` section of your configuration.

## Templates

Each media type has its own naming template. Configure them under `file_renaming`:

```yaml
file_renaming:
  movie_template: "{title} ({year}) [{quality}]"
  series_template: "{title} - S{season}E{episode} [{quality}]"
  anime_template: "{title} - {season}x{episode} [{quality}]"
```

## Available Placeholders

| Placeholder  | Description                                        | Example    |
|--------------|----------------------------------------------------|------------|
| `{title}`    | Media title                                        | `Inception`|
| `{year}`     | Release year                                       | `2010`     |
| `{season}`   | Season number (zero-padded)                        | `01`       |
| `{episode}`  | Episode number (zero-padded)                       | `05`       |
| `{quality}`  | Detected quality from torrent name                 | `1080p`    |

## Quality Detection

Quality is parsed from the torrent name. The following values are detected:

| Quality     | Also Matches |
|-------------|-------------|
| 2160p       | 4K, UHD     |
| 1080p       |             |
| 720p        |             |
| 480p        |             |
| 360p        |             |
| WEB-DL      |             |
| WEBRip      |             |
| BluRay      |             |
| HDTV        |             |
| DVDRip      |             |

!!! note
    If the quality cannot be determined from the torrent name, it returns `Unknown`.

## Result Examples

With the default templates applied:

- **Movie:** `Inception (2010) [1080p].mkv`
- **TV Show:** `Breaking Bad - S05E16 [1080p].mkv`
- **Anime:** `Attack on Titan - 01x05 [720p].mkv`

## Default Templates

If no template is configured, Reel falls back to these defaults:

| Media Type | Default Template                              |
|------------|-----------------------------------------------|
| Movies     | `{title} ({year}) [{quality}].ext`            |
| TV Shows   | `{title} - S{season}E{episode} [{quality}].ext` |
| Anime      | `{title} - S{season}E{episode} [{quality}].ext` |

!!! warning
    Subtitle files (`.srt`, `.sub`, `.ass`) are **not** renamed by this stage. They are moved alongside the media file but retain their original filenames.
