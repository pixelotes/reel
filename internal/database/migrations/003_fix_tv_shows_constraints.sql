-- Create a new migration file: internal/database/migrations/003_fix_tv_show_constraints.sql

-- Drop the existing unique constraint that's causing issues
DROP INDEX IF EXISTS idx_media_tmdb_id_type;

-- Create separate unique constraints for movies and TV shows
-- Movies use TMDB IDs
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_movie_tmdb 
ON media(tmdb_id, type) 
WHERE type = 'movie' AND tmdb_id IS NOT NULL;

-- TV shows use a combination of title and year since they might use different metadata providers
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_tvshow_title_year 
ON media(title, year, type) 
WHERE type = 'tvshow';

-- Add index for TV show foreign key
CREATE INDEX IF NOT EXISTS idx_media_tv_show_id ON media(tv_show_id) WHERE tv_show_id IS NOT NULL;