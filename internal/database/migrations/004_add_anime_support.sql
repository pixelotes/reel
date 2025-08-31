-- SQLite does not support dropping or altering CHECK constraints directly.
-- The standard workaround is to rename the table, create a new one with the updated schema,
-- copy the data, and then drop the old table.

PRAGMA foreign_keys=off;

-- 1. Rename the existing 'media' table
ALTER TABLE media RENAME TO media_old;

-- 2. Create the new 'media' table with the updated CHECK constraint for the 'type' column
CREATE TABLE media (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('movie', 'tvshow', 'anime')),
    imdb_id TEXT,
    tmdb_id INTEGER,
    title TEXT NOT NULL,
    year INTEGER,
    language TEXT NOT NULL DEFAULT 'en',
    min_quality TEXT NOT NULL DEFAULT '720p',
    max_quality TEXT NOT NULL DEFAULT '1080p',
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'searching', 'downloading', 'downloaded', 'failed', 'skipped')),
    torrent_hash TEXT,
    torrent_name TEXT,
    download_path TEXT,
    progress REAL DEFAULT 0.0,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    overview TEXT,
    poster_url TEXT,
    rating REAL,
    auto_download BOOLEAN DEFAULT TRUE,
    tv_show_id INTEGER REFERENCES tv_shows(id)
);

-- 3. Copy the data from the old table to the new table
INSERT INTO media (id, type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality, status, torrent_hash, torrent_name, download_path, progress, added_at, completed_at, overview, poster_url, rating, auto_download, tv_show_id)
SELECT id, type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality, status, torrent_hash, torrent_name, download_path, progress, added_at, completed_at, overview, poster_url, rating, auto_download, tv_show_id
FROM media_old;

-- 4. Recreate all indexes from previous migrations on the new table
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_movie_tmdb
ON media(tmdb_id, type)
WHERE type = 'movie' AND tmdb_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_tvshow_title_year
ON media(title, year, type)
WHERE type = 'tvshow';

CREATE INDEX IF NOT EXISTS idx_media_tv_show_id ON media(tv_show_id) WHERE tv_show_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_media_status ON media(status);
CREATE INDEX IF NOT EXISTS idx_media_type ON media(type);
CREATE INDEX IF NOT EXISTS idx_media_added_at ON media(added_at);


-- 5. Drop the old table
DROP TABLE media_old;

PRAGMA foreign_keys=on;