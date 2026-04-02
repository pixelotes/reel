# Indexers

Indexers are the sources Reel queries to find torrents for your media. Each media type (movies, TV shows, anime) has its own `sources` list in the configuration, and each source entry points to one indexer.

Reel supports four indexer types, each with a different protocol and trade-offs.

---

## Supported Indexers

| Type | Protocol | API Key Required | Search Capabilities | Self-Hosted |
|------|----------|------------------|---------------------|-------------|
| Scarf | Torznab | Yes | Movie search, TV search | Yes |
| Jackett | Torznab | Yes | Depends on underlying indexers | Yes |
| Prowlarr | JSON API | Yes | Depends on underlying indexers | Yes |
| RSS | RSS/XML | No | Title matching only | No |

---

## Scarf

Scarf is a Torznab-compatible indexer built specifically for Reel. It runs as a separate Docker container and exposes category-based Torznab endpoints.

- Docker image: `pixelotes/scarf`
- Supports `movie-search` and `tv-search` Torznab capabilities
- Each category gets its own endpoint path (e.g., `/torznab/movies`, `/torznab/tv`, `/torznab/anime`)

```yaml title="Source configuration"
sources:
  - type: "scarf"
    url: "http://scarf:8080/torznab/movies"
    api_key: "your_scarf_api_key"
```

!!! tip
    When running both Reel and Scarf in Docker Compose, use the container name as the hostname (e.g., `http://scarf:8080`).

---

## Jackett

Jackett acts as a proxy that translates Torznab queries into requests for dozens of different torrent indexers. You configure your indexers inside Jackett, then point Reel at Jackett's unified Torznab endpoint.

- Default port: `9117`
- The `/api/v2.0/indexers/all/results/torznab` endpoint searches all configured indexers at once
- API key is found in the Jackett web UI under settings

```yaml title="Source configuration"
sources:
  - type: "jackett"
    url: "http://jackett:9117/api/v2.0/indexers/all/results/torznab"
    api_key: "your_jackett_api_key"
```

!!! note
    You can also target a specific Jackett indexer by replacing `all` in the URL with the indexer ID (e.g., `/api/v2.0/indexers/1337x/results/torznab`).

---

## Prowlarr

Prowlarr is an indexer manager that exposes a JSON API rather than Torznab. Reel communicates with Prowlarr using its `/api/v1/search` and `/api/v1/health` endpoints, sending the API key via the `X-Api-Key` header.

- Default port: `9696`
- API key is found in Prowlarr under Settings > General
- The URL should be the Prowlarr base URL without any path suffix

```yaml title="Source configuration"
sources:
  - type: "prowlarr"
    url: "http://prowlarr:9696"
    api_key: "your_prowlarr_api_key"
```

---

## RSS

Plain RSS feeds can be used as a source for monitoring new releases. This is the simplest indexer type but also the most limited.

- No API key required
- No seeders/leechers data available
- Matching is based on title only
- Best suited for monitoring new releases from a known feed

```yaml title="Source configuration"
sources:
  - type: "rss"
    url: "https://example.com/rss/feed.xml"
```

!!! warning
    RSS sources cannot perform targeted searches. They rely on basic title matching against whatever the feed publishes. Use a Torznab-based indexer (Scarf, Jackett) or Prowlarr for reliable search results.

---

## Source Configuration Reference

Every source entry in a media type's `sources` list accepts these fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Indexer type: `scarf`, `jackett`, `prowlarr`, or `rss`. |
| `url` | string | Yes | Endpoint URL for the indexer. |
| `api_key` | string | Depends | API key for authentication. Required for all types except `rss`. |
| `search_mode` | string | No | Overrides the default search mode for this source. |

---

## Full Example

A movies configuration using multiple indexer types:

```yaml
movies:
  providers: ["tmdb"]
  download_folder: "/downloads/movies"
  destination_folder: "/media/movies"
  move_method: ["hardlink", "move"]
  sources:
    - type: "scarf"
      url: "http://scarf:8080/torznab/movies"
      api_key: "scarf_key_here"
    - type: "jackett"
      url: "http://jackett:9117/api/v2.0/indexers/all/results/torznab"
      api_key: "jackett_key_here"
    - type: "prowlarr"
      url: "http://prowlarr:9696"
      api_key: "prowlarr_key_here"
    - type: "rss"
      url: "https://example.com/movies-rss.xml"
```

Reel queries all configured sources in parallel and merges the results before applying quality filters.
