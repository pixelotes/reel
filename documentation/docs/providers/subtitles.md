# Subtitles

Reel can automatically download subtitles for your media using the [OpenSubtitles.com](https://www.opensubtitles.com/) API. Subtitle downloads are integrated into the post-processing pipeline and run after files have been moved and renamed.

---

## How It Works

Subtitles come through [Scout](https://github.com/pixelotes/scout), locally or from a Scout service (see `crawlers` in the [configuration reference](../configuration/reference.md)). Reel computes the file hash itself, so a Scout service never needs access to your media. The search stops at the first step that finds something:

1. **File hash** -- an exact match for this release. Only real hash matches count.
2. **IDs** -- the IMDb or TMDB ID (the show's ID plus season and episode for TV).
3. **Title** -- the title (and year for movies), plus season and episode.

Three providers are searched together and their results merged, so a language one lacks can come from another:

| Provider | Needs | Notes |
|----------|-------|-------|
| OpenSubtitles | `subtitles.api_key` | The only one that matches by file hash. Daily download quota. |
| SubDL | `subtitles.subdl_api_key` (free) | Searches by IMDb/TMDB ID or title. |
| Subdivx | `subtitles.flaresolverr_url` | Spanish only. The site sits behind Cloudflare, so it goes through a FlareSolverr-compatible solver such as [Parasol](https://github.com/pixelotes/parasol). It is scraping, not an API, and can break when the site changes. |

Season packs are opened and the right episode extracted, including anime packs numbered `Show - 05`. Subtitles are converted to UTF-8.

For each configured language that has no subtitle yet, Reel downloads the best candidate: a hash match first, then trusted uploaders, then the most downloaded. Machine translations are only used when nothing else exists.

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

How Reel stays within them:

- Requests are spaced out automatically.
- Downloaded files are cached in `data/subtitle-cache` (or on the Scout service) and never downloaded twice.
- When the API reports the daily quota used up, Reel stops asking until it resets, and the periodic scan for missing subtitles stops early.

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
