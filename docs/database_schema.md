# Database Schema

Reel uses a SQLite database to store all of its data. Here is a breakdown of the database schema:

### `media`

This is the main table that stores all of your media items, including movies, TV shows, and anime.

| Column          | Type      | Description                                                                 |
| --------------- | --------- | --------------------------------------------------------------------------- |
| `id`            | INTEGER   | The primary key for the media item.                                         |
| `type`          | TEXT      | The type of media, which can be 'movie', 'tvshow', or 'anime'.              |
| `imdb_id`       | TEXT      | The IMDb ID for the media item (optional).                                  |
| `tmdb_id`       | INTEGER   | The TMDB ID for the media item (optional).                                  |
| `title`         | TEXT      | The title of the media item.                                                |
| `year`          | INTEGER   | The release year of the media item.                                         |
| `language`      | TEXT      | The preferred language for the media item.                                  |
| `min_quality`   | TEXT      | The minimum acceptable quality for a download.                              |
| `max_quality`   | TEXT      | The maximum acceptable quality for a download.                              |
| `status`        | TEXT      | The current status of the media item (e.g., 'pending', 'downloading').      |
| `torrent_hash`  | TEXT      | The hash of the torrent file for the download.                              |
| `torrent_name`  | TEXT      | The name of the torrent file.                                               |
| `download_path` | TEXT      | The path where the media item is downloaded.                                |
| `progress`      | REAL      | The download progress, from 0.0 to 1.0.                                     |
| `added_at`      | DATETIME  | The date and time the media item was added to Reel.                         |
| `completed_at`  | DATETIME  | The date and time the download was completed.                               |
| `overview`      | TEXT      | A brief overview or synopsis of the media item.                             |
| `poster_url`    | TEXT      | The URL for the media item's poster image.                                  |
| `rating`        | REAL      | The rating of the media item.                                               |
| `auto_download` | BOOLEAN   | Whether to automatically download the media item when it's found.           |
| `tv_show_id`    | INTEGER   | A foreign key that links to the `tv_shows` table for TV shows and anime.    |

### `tv_shows`

This table stores information specific to TV shows and anime.

| Column    | Type    | Description                                       |
| --------- | ------- | ------------------------------------------------- |
| `id`      | INTEGER | The primary key for the TV show.                  |
| `status`  | TEXT    | The status of the TV show (e.g., 'Running', 'Ended'). |
| `tvmaze_id`| TEXT    | The TVmaze ID for the TV show.                    |

### `seasons`

This table stores information about the seasons of a TV show or anime.

| Column        | Type    | Description                               |
| ------------- | ------- | ----------------------------------------- |
| `id`          | INTEGER | The primary key for the season.           |
| `show_id`     | INTEGER | A foreign key that links to the `tv_shows` table. |
| `season_number`| INTEGER | The season number.                        |

### `episodes`

This table stores information about individual episodes of a TV show or anime.

| Column         | Type     | Description                                     |
| -------------- | -------- | ----------------------------------------------- |
| `id`           | INTEGER  | The primary key for the episode.                |
| `season_id`    | INTEGER  | A foreign key that links to the `seasons` table. |
| `episode_number`| INTEGER  | The episode number.                             |
| `title`        | TEXT     | The title of the episode.                       |
| `air_date`     | TEXT     | The original air date of the episode.           |
| `status`       | TEXT     | The status of the episode (e.g., 'pending').    |
| `torrent_hash` | TEXT     | The hash of the torrent file for the download.  |
| `torrent_name` | TEXT     | The name of the torrent file.                   |
| `progress`     | REAL     | The download progress, from 0.0 to 1.0.         |
| `completed_at` | DATETIME | The date and time the download was completed.   |

### `anime_search_terms`

This table stores alternative search terms for anime, which can be useful for finding releases with different titles.

| Column   | Type    | Description                               |
| -------- | ------- | ----------------------------------------- |
| `id`     | INTEGER | The primary key for the search term.      |
| `media_id`| INTEGER | A foreign key that links to the `media` table. |
| `term`   | TEXT    | The alternative search term.              |
