# Quality Filtering

Reel filters torrents based on quality preferences, minimum seeder counts, and rejection rules. This ensures only appropriate releases are downloaded.

## Quality Preferences

Configure preferred qualities under `automation.quality_preferences`. Higher position in the list means higher priority. Reel picks the best available match.

```yaml
automation:
  quality_preferences:
    - "1080p"
    - "720p"
```

!!! info
    In the example above, Reel prefers `1080p` releases. If none are available, it falls back to `720p`.

## Minimum Seeders

Reject torrents below a minimum seeder count to avoid slow or stalled downloads:

```yaml
automation:
  min_seeders: 3
```

!!! warning
    Setting this too high may cause Reel to skip valid releases, especially for older or niche content.

## Rejection Rules

Regex patterns that reject matching torrent names. Configured under `automation.reject-common` or as a top-level `reject-common` key:

```yaml
reject-common:
  - \bscreener\b
  - \bhdcam\b
  - \btelecine\b
  - \bvostfr\b
  - \bfrench\b
  - \brus\b
  - \bdvdscr\b
```

!!! note
    Patterns are **case-insensitive**. Use `\b` for word boundaries to avoid false positives (e.g. `\brus\b` matches "RUS" but not "trust").

### Common Patterns to Reject

| Category        | Patterns                                      |
|-----------------|-----------------------------------------------|
| Low quality     | `screener`, `hdcam`, `telecine`, `cam`, `ts`, `dvdscr` |
| Wrong language  | `vostfr`, `french`, `rus`, `ita`, `german`    |
| Other           | `md` (mic dubbed)                             |

## Torrent Scoring

After filtering, Reel scores each remaining torrent to pick the best one. The score is calculated from the torrent name using a token-based system.

The formula is:

```
Score = (QualityScore x 1000) + min(Seeders, 100)
```

Quality dominates the score (multiplied by 1000), with seeders as a tiebreaker (capped at 100 to prevent a high-seeder low-quality torrent from winning).

### Quality score tokens

Each recognized token in the torrent name adds points:

| Category | Token | Points |
|----------|-------|--------|
| **Resolution** | `2160p` / `4k` / `uhd` | 8 |
| | `1080p` / `fhd` | 5 |
| | `720p` / `hd` | 4 |
| | `480p` / `sd` | 2-3 |
| **Source** | `remux` | 10 |
| | `bluray` / `bdrip` | 8 |
| | `webdl` (web-dl) | 7 |
| | `web` | 6 |
| | `brrip` | 6 |
| | `webrip` | 5 |
| | `hdtv` | 4 |
| | `dvdrip` | 3 |
| | `cam` / `ts` | 1 |
| **Codec** | `av1` | 6 |
| | `x265` / `h265` / `hevc` | 5 |
| | `x264` / `h264` / `avc` | 2 |
| **Audio** | `dolbyvision` / `dv` | 3 |
| | `atmos` / `truehd` / `dtshd` / `dtsx` | 3 |
| | `dts` / `eac3` | 2 |
| | `ac3` / `aac` | 1 |
| **HDR** | `hdr` / `hdr10` | 2 |
| | `imax` | 2 |
| **Special** | `repack` / `proper` / `extended` | 1 |

Scores are **additive** - a torrent named `Movie.2024.2160p.BluRay.REMUX.DTS-HD` gets: 8 (2160p) + 8 (bluray) + 10 (remux) + 3 (dtshd) = **29** quality points, for a total score of 29000 + seeders.

### Score examples

| Torrent name | Quality score | With 50 seeders |
|---|---|---|
| `Movie.2024.2160p.BluRay.REMUX.DTS-HD.x265` | 8+8+10+3+5 = 34 | 34050 |
| `Movie.2024.1080p.WEB-DL.x264.AAC` | 5+7+2+1 = 15 | 15050 |
| `Movie.2024.1080p.HDTV.x264` | 5+4+2 = 11 | 11050 |
| `Movie.2024.720p.WEBRip` | 4+5 = 9 | 9050 |
| `Movie.2024.CAM` | 1 | 1050 |

!!! tip
    Enable `filter_log_level: "detail"` in the app config to see scoring decisions in `data/filter.log`. Each torrent logs as `PASS: [Score: N] Title` or `REJECT: [reason] Title`.

### Selection pipeline

The full selection process runs in this order:

1. **Reject patterns** - Remove torrents matching `reject-common` regex
2. **Episode matching** - For TV/anime: filter by correct season/episode number
3. **Series name matching** - For TV/anime: verify the title matches the series
4. **Quality range** - Filter by media's `min_quality` / `max_quality` range
5. **Minimum seeders** - Remove below `min_seeders` threshold
6. **Score and sort** - Calculate score for remaining torrents, pick highest

## Extra Trackers

Add extra trackers to every torrent to improve peer connectivity:

```yaml
extra_trackers_list:
  - udp://tracker.opentrackr.org:1337/announce
  - udp://open.stealth.si:80/announce
  - udp://tracker.torrent.eu.org:451/announce
```

!!! tip
    Public trackers help improve download speeds, especially for popular releases. These are appended to whatever trackers the torrent already has.

## Full Configuration Example

```yaml
automation:
  quality_preferences:
    - "1080p"
    - "720p"
  min_seeders: 3
  reject-common:
    - \bscreener\b
    - \bhdcam\b
    - \btelecine\b
    - \bcam\b
    - \bts\b
    - \bdvdscr\b
    - \bvostfr\b
    - \bfrench\b
    - \brus\b
    - \bita\b
    - \bgerman\b
    - \bmd\b

extra_trackers_list:
  - udp://tracker.opentrackr.org:1337/announce
  - udp://open.stealth.si:80/announce
```
