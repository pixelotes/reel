# Movies

## How Reel Handles Movies

Reel automates the full lifecycle of movie management, from search to organized storage.

1. **Add** -- Search from the UI or add manually. Metadata is fetched from configured providers (TMDB).
2. **Search** -- Reel queries configured movie sources (indexers) periodically.
3. **Filter** -- Results are filtered by quality preferences, seeders, and rejection rules.
4. **Download** -- The best match is sent to the torrent client in the configured `download_folder`.
5. **Process** -- The pipeline moves the file to `destination_folder` and renames it using `movie_template`.

## Folder Structure

Reel creates a folder per movie, named with the title and year:

```
/media/movies/
  Inception (2010)/
    Inception (2010) [1080p].mkv
```

## Configuration

```yaml
movies:
  providers: ["tmdb"]
  download_folder: "/downloads/movies"
  destination_folder: "/media/movies"
  move_method: ["hardlink", "copy"]
  sources:
    - type: "scarf"
      url: "http://scarf:8080/torznab/movies"
      api_key: "your_key"
```

### Configuration Options

| Key                  | Description                                              |
|----------------------|----------------------------------------------------------|
| `providers`          | Metadata providers for movie information                 |
| `download_folder`    | Where the torrent client downloads movie files           |
| `destination_folder` | Final organized location for movies                      |
| `move_method`        | How files are transferred (in order of preference)       |
| `sources`            | List of indexer sources to search for movie torrents     |

## Metadata Providers

| Provider | Description                          | API Key Required |
|----------|--------------------------------------|-----------------|
| TMDB     | The Movie Database -- primary and most complete provider for movies | Yes |

!!! tip
    TMDB is the recommended provider for movies. It offers comprehensive metadata including titles, release dates, posters, and descriptions.

## Move Methods

The `move_method` list defines the preferred transfer strategy, in order of fallback:

| Method     | Description                                              |
|------------|----------------------------------------------------------|
| `hardlink` | Creates a hard link (instant, no extra disk space)       |
| `copy`     | Copies the file (uses additional disk space)             |

!!! note
    Hardlinking is preferred because it allows the torrent client to continue seeding without using extra disk space. If hardlinking fails (e.g. cross-filesystem), Reel falls back to the next method in the list.
