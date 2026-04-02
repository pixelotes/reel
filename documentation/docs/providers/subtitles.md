# Subtitles

Reel can automatically download subtitles for your media using the [OpenSubtitles.com](https://www.opensubtitles.com/) API. Subtitle downloads are integrated into the post-processing pipeline and run after files have been moved and renamed.

---

## How It Works

When subtitle downloading is enabled, Reel uses a two-step search strategy:

1. **File hash search** -- Computes a hash of the video file and searches OpenSubtitles for an exact match. This is the most accurate method.
2. **Metadata search** -- If no hash match is found, Reel falls back to searching by title, year, TMDB ID, and season/episode number.

Subtitle files are saved alongside the video file using the naming pattern:

```
{VideoFileName}.{language}.srt
```

For example: `The.Movie.2024.1080p.mkv` with English subtitles becomes `The.Movie.2024.1080p.en.srt`.

!!! info "Non-critical stage"
    Subtitle failures do not stop the post-processing pipeline. If subtitles cannot be found or downloaded, Reel logs a warning and continues with the remaining stages.

---

## Configuration

```yaml
subtitles:
  enabled: true
  api_key: "your_opensubtitles_api_key"
  languages: ["en", "es"]
  download_path: ""  # optional, defaults to same folder as media
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `subtitles.enabled` | bool | `false` | Enable automatic subtitle downloads. |
| `subtitles.api_key` | string | | OpenSubtitles API key. Required when enabled. |
| `subtitles.languages` | []string | | List of ISO 639-1 language codes to download (e.g., `en`, `es`, `fr`, `de`, `ja`). |
| `subtitles.download_path` | string | | Optional override path for subtitle files. If empty, subtitles are saved in the same directory as the video file. |

---

## Getting an API Key

1. Create a free account at [opensubtitles.com](https://www.opensubtitles.com/)
2. Go to your profile and navigate to the [API consumers](https://www.opensubtitles.com/en/consumers) page
3. Register a new consumer to receive your API key
4. Copy the API key into your Reel configuration

!!! note
    OpenSubtitles.com (the `.com` version) uses API v1, which is separate from the older opensubtitles.org site. Make sure you create an account on the `.com` domain.

---

## Rate Limits and Retries

The OpenSubtitles free tier imposes strict rate limits:

| Limit | Value |
|-------|-------|
| Requests per minute | 5 |
| Daily download quota | 20 subtitles |

Reel handles rate limiting automatically with built-in retry logic:

- **3 retry attempts** per request
- **Exponential backoff** between attempts
- Respects the API's rate limit headers

!!! tip
    If you have a large library, the daily download quota may be a bottleneck. Consider processing new additions in batches or upgrading to a paid OpenSubtitles plan for higher limits.

---

## Language Codes

Languages are specified using ISO 639-1 two-letter codes. Common examples:

| Code | Language |
|------|----------|
| `en` | English |
| `es` | Spanish |
| `fr` | French |
| `de` | German |
| `it` | Italian |
| `pt` | Portuguese |
| `ja` | Japanese |
| `zh` | Chinese |
| `ko` | Korean |
| `ar` | Arabic |

Reel downloads one subtitle file per language. If you configure `["en", "es"]`, each video will get both an `.en.srt` and an `.es.srt` file (when available).

---

## Full Example

```yaml
subtitles:
  enabled: true
  api_key: "your_opensubtitles_api_key"
  languages: ["en", "es", "fr"]

postprocessing:
  enabled: true
  pipeline:
    enabled: true
    stages:
      - name: validate
        enabled: true
      - name: create_folders
        enabled: true
      - name: move_files
        enabled: true
      - name: rename
        enabled: true
      - name: subtitles
        enabled: true
      - name: notify
        enabled: true
```
