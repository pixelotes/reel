# Ebooks

Reel can manage ebooks alongside your movies, TV shows, and anime.

## How it works

1. **Add** - Search from the UI. Metadata fetched from Gutendex (Project Gutenberg).
2. **Search** - Reel queries Gutendex for matching ebooks in your configured language.
3. **Download** - Best match downloaded to ebooks download_folder.
4. **Process** - Pipeline moves to destination_folder and renames using ebook_template.
5. **Read** - EPUB files can be opened directly in the built-in reader.

## Folder structure

```
/media/ebooks/
  Dune (1965)/
    Dune (1965).epub
```

!!! note
    Ebooks don't have seasons or episodes, so the folder structure is flat - similar to movies.

## Configuration

```yaml
ebooks:
  providers: ["gutendex"]
  sources:
    - type: gutendex
  download_folder: "/downloads/ebooks"
  destination_folder: "/media/ebooks"
  move_method: ["hardlink", "copy"]

file_renaming:
  ebook_template: "{title} ({year})"
```

!!! tip
    Ebook configuration is optional. If you don't configure the `ebooks` section, Reel works exactly as before with movies, TV shows, and anime.

## Download source

Gutendex serves as both metadata provider and download source, ensuring the book you find is exactly the one you download.

### Gutendex (Project Gutenberg)

Searches Project Gutenberg's catalog of 70,000+ public domain books. Returns direct download URLs in EPUB, MOBI, PDF, or HTML format (in that priority order). Results are filtered by your configured `metadata.language`.

```yaml
ebooks:
  providers: ["gutendex"]
  sources:
    - type: gutendex
```

!!! note
    Gutendex only indexes public domain books from Project Gutenberg. For other sources (Open Library, Anna's Archive, etc.), configure them as indexers in Scarf.

### Torrent indexers

You can also use standard torrent indexers (Jackett, Prowlarr, Scarf) to search for ebooks via torrent.

```yaml
ebooks:
  sources:
    - type: gutendex
    - type: jackett
      url: "http://jackett:9117/api/v2.0/indexers/all/results/torznab"
      api_key: "your_jackett_key"
```

## Language filtering

Gutendex filters results by the language configured in `metadata.language`. Only books in the matching language will be returned.

```yaml
metadata:
  language: "es"  # Only return Spanish books
```

Uses ISO 639-1 language codes: `en`, `es`, `fr`, `de`, `it`, `pt`, etc.

## Built-in EPUB reader

Downloaded EPUB files can be read directly in the browser. Click the **Read** button on any downloaded ebook to open the viewer.

Features:

- Page navigation with buttons or arrow keys (← →)
- Reading progress saved automatically (per book)
- Resumes from last position when reopening
- Close with the ✕ button or Escape key

!!! note
    The built-in reader supports EPUB format only. PDF and other formats will need an external reader.

## Supported file formats

The pipeline recognizes these ebook extensions:

| Format | Extension |
|--------|-----------|
| EPUB | `.epub` |
| PDF | `.pdf` |
| Kindle | `.mobi`, `.azw3` |
| FictionBook | `.fb2` |

## File renaming

Default template: `{title} ({year})`

Available placeholders:

- `{title}` - Book title
- `{year}` - Publication year
- `{quality}` - Detected quality (less relevant for ebooks)

Example:

```yaml
file_renaming:
  ebook_template: "{title} ({year})"
```

Result: `Dune (1965).epub`

## Pipeline conditions

Use `is_ebook` to run stages only for ebooks:

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
      - name: notify
        enabled: true
        condition: "is_ebook"
```
