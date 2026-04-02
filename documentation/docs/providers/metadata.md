# Metadata Providers

Metadata providers supply information about your media: titles, years, posters, episode data, ratings, and more. Reel uses this data for search queries, file renaming, and library organization.

Each media type specifies which providers to use in its `providers` list. Reel queries them in the order listed.

---

## Provider Compatibility

| Provider | Movies | TV Shows | Anime | API Key Required |
|----------|--------|----------|-------|------------------|
| TMDB | Yes | No | No | Yes |
| TVmaze | No | Yes | No | No |
| AniList | No | No | Yes | No |
| Trakt | No | Yes | No | Yes (`client_id`) |
| IMDb | -- | -- | -- | N/A (not functional) |

---

## TMDB

[The Movie Database](https://www.themoviedb.org/) is the primary provider for movie metadata. It returns titles, release years, poster URLs, and ratings.

- Requires a free API key (v3)
- Obtain one at [themoviedb.org/settings/api](https://www.themoviedb.org/settings/api)
- Used for movie search and poster artwork

```yaml
metadata:
  language: "en"
  tmdb:
    api_key: "your_tmdb_v3_api_key"

movies:
  providers: ["tmdb"]
```

!!! tip
    TMDB API keys are free for non-commercial use. Sign up for an account and request a key under your account settings.

---

## TVmaze

[TVmaze](https://www.tvmaze.com/) provides TV show metadata including seasons, episodes, air dates, and show status. It has a free public API that does not require an API key for basic use.

- Returns season and episode listings with air dates
- Tracks show status (running, ended, etc.)
- No API key needed for basic queries

```yaml
metadata:
  tvmaze:
    api_key: ""  # optional, not required for basic use

tv-shows:
  providers: ["tvmaze"]
```

---

## AniList

[AniList](https://anilist.co/) provides anime metadata through a public GraphQL API. No API key is required.

- Returns both English and Romaji titles
- Provides episode counts, ratings, and airing status
- Results are sorted by popularity
- Public GraphQL endpoint, no authentication needed

```yaml
metadata:
  anilist: {}  # no configuration needed

anime:
  providers: ["anilist"]
```

---

## Trakt

[Trakt](https://trakt.tv/) provides detailed TV show and episode metadata. It requires a client ID obtained by creating an application on Trakt.

- Create an application at [trakt.tv/oauth/applications](https://trakt.tv/oauth/applications)
- Integrates with TMDB for additional data (posters, artwork)
- Automatically skips specials (season 0)

```yaml
metadata:
  trakt:
    client_id: "your_trakt_client_id"

tv-shows:
  providers: ["trakt"]
```

!!! note
    Trakt can be used alongside TVmaze. List both in the `providers` array and Reel will query them in order, using the first successful result.

---

## IMDb

IMDb is listed as a provider type but is not currently functional. There is no public IMDb API, so this entry exists only as a placeholder for potential future integration.

```yaml
metadata:
  imdb:
    api_key: ""  # placeholder, not used
```

!!! warning
    Do not rely on IMDb as a metadata provider. It will not return results. Use TMDB for movies and TVmaze or Trakt for TV shows.

---

## Configuration Reference

### Global Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `metadata.language` | string | `en` | ISO 639-1 language code for metadata results. |
| `metadata.timeout` | int | `10` | HTTP timeout in seconds for metadata API requests. |

### Provider Settings

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `metadata.tmdb.api_key` | string | Yes (for movies) | TMDB API key (v3). |
| `metadata.tvmaze.api_key` | string | No | Optional TVmaze API key. |
| `metadata.trakt.client_id` | string | Yes (for Trakt) | Trakt application client ID. |
| `metadata.imdb.api_key` | string | No | Placeholder. Not used. |
| `metadata.anilist` | object | No | Empty object. No configuration needed. |

---

## Recommended Setup

```yaml
metadata:
  language: "en"
  timeout: 10
  tmdb:
    api_key: "your_tmdb_key"
  tvmaze:
    api_key: ""
  anilist: {}
  trakt:
    client_id: "your_trakt_client_id"

movies:
  providers: ["tmdb"]

tv-shows:
  providers: ["tvmaze", "trakt"]

anime:
  providers: ["anilist"]
```
