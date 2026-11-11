-- Which metadata provider tv_shows.tvmaze_id belongs to (tvmaze, tmdb,
-- anilist, kitsu, trakt). Despite its name, tvmaze_id has always held the
-- ID of whichever provider found the show; NULL means unknown (older rows),
-- and Reel then falls back to searching by title.
ALTER TABLE tv_shows ADD COLUMN provider TEXT;
