# Manga

Reel can manage manga using MangaDex as both metadata and download source.

## How it works

1. **Add** - Search from the UI. Metadata fetched from MangaDex.
2. **Track** - Monitors for new chapters.
3. **Search** - Queries MangaDex for chapters in your configured languages.
4. **Download** - Chapters are downloaded as CBZ files (one per chapter) with rate limiting between downloads.
5. **Process** - Pipeline moves to destination_folder and renames using manga_template.
6. **Read** - Chapters can be read directly in the built-in manga viewer.

## Folder structure

```
/media/manga/
  One Piece (1997)/
    One Piece - Chapter 001.cbz
    One Piece - Chapter 002.cbz
```

## Configuration

```yaml
manga:
  providers:
    - mangadex
  languages:
    - en
    - es
  download_folder: "/downloads/manga"
  destination_folder: "/media/manga"
  move_method: ["hardlink", "copy"]

file_renaming:
  manga_template: "{title} - S{season}E{episode}"
```

### Configuration options

| Key                  | Description                                              |
|----------------------|----------------------------------------------------------|
| `providers`          | Metadata providers (currently only `mangadex`)           |
| `languages`          | Chapter languages in priority order                      |
| `download_folder`    | Where chapters are downloaded                            |
| `destination_folder` | Final organized location for manga                       |
| `move_method`        | How files are transferred (in order of preference)       |

!!! tip
    Manga configuration is optional. If you don't configure the `manga` section, Reel works exactly as before.

## Languages

The `languages` array controls which chapter translations are downloaded, in priority order. If a chapter exists in multiple languages, the first match wins.

```yaml
manga:
  languages:
    - es    # Spanish first
    - en    # English as fallback
```

Uses ISO 639-1 language codes. Common values: `en`, `es`, `fr`, `de`, `pt-br`, `it`, `ja`, `ko`, `zh`.

!!! warning
    Some manga may have no chapters available in your configured languages. When this happens, the UI will show a message indicating no chapters were found. This is a MangaDex availability issue, not a bug.

## Metadata and download source

MangaDex serves as both the metadata provider and download source. No additional indexers are needed.

| Feature | Details |
|---------|---------|
| Source | MangaDex API |
| API Key | Not required |
| Format | CBZ (ZIP archive of page images) |
| Rate limiting | 30 seconds between chapters, 200ms between pages |

## Built-in manga viewer

Downloaded chapters can be read directly in the browser. Click the **Read** button on any downloaded chapter.

Features:

- Right-to-left and left-to-right reading modes
- Page and scroll view modes
- Reading progress saved automatically
- Keyboard and touch navigation
- Close with the back button or Escape key

## Chapter structure

MangaDex chapters are organized as volumes (seasons) and chapters (episodes):

- **Volume** = Season number
- **Chapter** = Episode number

The UI displays these as "Volume X" with chapters listed inside.

## Pipeline conditions

Use `is_manga` to run stages only for manga:

```yaml
postprocessing:
  pipeline:
    stages:
      - name: validate
        enabled: true
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
```
