# Configuration

Reel is configured using a `config.yml` file. Here is a breakdown of all the available options:

### `app`

| Setting                      | Description                                                              |
| ---------------------------- | ------------------------------------------------------------------------ |
| `port`                       | The port to run the web server on.                                       |
| `data_path`                  | The path to the data directory, where the database and logs are stored.  |
| `ui_enabled`                 | Whether to enable the web UI.                                            |
| `ui_password`                | The password for the web UI.                                             |
| `debug`                      | Whether to enable debug logging.                                         |
| `jwt_secret`                 | The secret key for signing JWT tokens.                                   |
| `magnet_to_torrent_enabled`  | Whether to try to convert magnet links to torrent files.                 |
| `magnet_to_torrent_timeout`  | The timeout in seconds for converting magnet links.                      |
| `search_timeout`             | The timeout in seconds for searching indexers.                           |
| `filter_log_level`           | The log level for the torrent filter, can be "none" or "detail".         |

### `torrent_client`

| Setting         | Description                                                          |
| --------------- | -------------------------------------------------------------------- |
| `type`          | The type of torrent client, can be "transmission", "qbittorrent", or "aria2". |
| `host`          | The host and port of the torrent client.                             |
| `username`      | The username for the torrent client.                                 |
| `password`      | The password for the torrent client.                                 |
| `secret`        | The secret for the Aria2 torrent client.                             |
| `download_path` | The default path to download media to.                               |

### `notifications`

| Setting      | Description                                |
| ------------ | ------------------------------------------ |
| `pushbullet` | The configuration for Pushbullet notifications. |
| `api_key`    | The API key for Pushbullet.                |

### `extra_trackers_list`

A list of extra trackers to add to new torrents.

### `metadata`

| Setting    | Description                                       |
| ---------- | ------------------------------------------------- |
| `language` | The preferred language for metadata.              |
| `timeout`  | The timeout in seconds for fetching metadata.     |
| `tmdb`     | The configuration for The Movie Database (TMDB).  |
| `imdb`     | The configuration for IMDb.                       |
| `tvmaze`   | The configuration for TVmaze.                     |
| `anilist`  | The configuration for AniList.                    |
| `trakt`    | The configuration for Trakt.                      |

### `movies`, `tv-shows`, `anime`

| Setting              | Description                                                              |
| -------------------- | ------------------------------------------------------------------------ |
| `providers`          | The order of preference for metadata providers.                          |
| `download_folder`    | The path to download this type of media to.                              |
| `destination_folder` | The path to move this type of media to after post-processing.            |
| `move_method`        | The method to use for post-processing, can be "hardlink", "symlink", "move", or "copy". |
| `sources`            | A list of indexer sources for this type of media.                        |

### `file_renaming`

| Setting           | Description                                    |
| ----------------- | ---------------------------------------------- |
| `movie_template`  | The template for renaming movie files.         |
| `series_template` | The template for renaming TV show files.       |
| `anime_template`  | The template for renaming anime files.         |

### `database`

| Setting | Description                    |
| ------- | ------------------------------ |
| `path`  | The path to the database file. |

### `automation`

| Setting                        | Description                                                              |
| ------------------------------ | ------------------------------------------------------------------------ |
| `search_interval`              | The interval to run the search for pending media.                        |
| `episode_download_delay_hours` | The delay in hours before downloading new episodes.                      |
| `max_concurrent_downloads`     | The maximum number of concurrent downloads.                              |
| `quality_preferences`          | The order of preference for download qualities.                          |
| `min_seeders`                  | The minimum number of seeders for a torrent to be considered.            |
| `keep_torrents_for_days`       | The number of days to keep completed torrents for.                       |
| `keep_torrents_seed_ratio`     | The seed ratio to reach before removing completed torrents.                |
| `notifications`                | A list of notification providers to use.                                 |
| `reject-common`                | A list of regular expressions to use for rejecting releases.             |