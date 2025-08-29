CREATE TABLE IF NOT EXISTS tv_shows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    status TEXT NOT NULL,
    tvmaze_id TEXT
);

CREATE TABLE IF NOT EXISTS seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    show_id INTEGER NOT NULL,
    season_number INTEGER NOT NULL,
    FOREIGN KEY(show_id) REFERENCES tv_shows(id)
);

CREATE TABLE IF NOT EXISTS episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    title TEXT NOT NULL,
    air_date TEXT,
    status TEXT NOT NULL,
    FOREIGN KEY(season_id) REFERENCES seasons(id)
);

ALTER TABLE media ADD COLUMN tv_show_id INTEGER REFERENCES tv_shows(id);