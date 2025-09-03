CREATE TABLE IF NOT EXISTS media_titles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    language TEXT NOT NULL,
    title TEXT NOT NULL,
    FOREIGN KEY(media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_media_titles_media_id ON media_titles(media_id);