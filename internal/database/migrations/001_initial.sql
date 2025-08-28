CREATE TABLE IF NOT EXISTS media (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('movie', 'tvshow')),
    imdb_id TEXT,
    tmdb_id INTEGER,
    title TEXT NOT NULL,
    year INTEGER,
    language TEXT NOT NULL DEFAULT 'en',
    min_quality TEXT NOT NULL DEFAULT '720p',
    max_quality TEXT NOT NULL DEFAULT '1080p',
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'searching', 'downloading', 'downloaded', 'failed')),
    torrent_hash TEXT,
    torrent_name TEXT,
    download_path TEXT,
    progress REAL DEFAULT 0.0,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    overview TEXT,
    poster_url TEXT,
    rating REAL
);

-- Create the partial unique index separately
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_imdb_id_type ON media(imdb_id, type) WHERE imdb_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_media_status ON media(status);
CREATE INDEX IF NOT EXISTS idx_media_type ON media(type);
CREATE INDEX IF NOT EXISTS idx_media_added_at ON media(added_at);