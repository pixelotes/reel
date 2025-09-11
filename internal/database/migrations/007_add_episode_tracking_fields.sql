ALTER TABLE episodes ADD COLUMN torrent_hash TEXT;
ALTER TABLE episodes ADD COLUMN torrent_name TEXT;
ALTER TABLE episodes ADD COLUMN progress REAL NOT NULL DEFAULT 0;
ALTER TABLE episodes ADD COLUMN completed_at DATETIME;