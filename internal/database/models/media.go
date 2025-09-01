package models

import (
	"database/sql"
	"fmt"
	"time"
)

type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTVShow MediaType = "tvshow"
	MediaTypeAnime  MediaType = "anime"
)

type MediaStatus string

const (
	StatusPending        MediaStatus = "pending"
	StatusSearching      MediaStatus = "searching"
	StatusDownloading    MediaStatus = "downloading"
	StatusDownloaded     MediaStatus = "downloaded"
	StatusFailed         MediaStatus = "failed"
	StatusSkipped        MediaStatus = "skipped"
	StatusMonitoring     MediaStatus = "monitoring"
	StatusPostProcessing MediaStatus = "post-processing"
	StatusTBA            MediaStatus = "tba"
	StatusArchived       MediaStatus = "archived"
)

type Media struct {
	ID           int         `json:"id" db:"id"`
	Type         MediaType   `json:"type" db:"type"`
	IMDBId       string      `json:"imdb_id,omitempty" db:"imdb_id"`
	TMDBId       *int        `json:"tmdb_id,omitempty" db:"tmdb_id"`
	TVShowID     *int        `json:"tv_show_id,omitempty" db:"tv_show_id"`
	Title        string      `json:"title" db:"title"`
	Year         int         `json:"year" db:"year"`
	Language     string      `json:"language" db:"language"`
	MinQuality   string      `json:"min_quality" db:"min_quality"`
	MaxQuality   string      `json:"max_quality" db:"max_quality"`
	Status       MediaStatus `json:"status" db:"status"`
	TorrentHash  *string     `json:"torrent_hash,omitempty" db:"torrent_hash"`
	TorrentName  *string     `json:"torrent_name,omitempty" db:"torrent_name"`
	DownloadPath *string     `json:"download_path,omitempty" db:"download_path"`
	Progress     float64     `json:"progress" db:"progress"`
	AddedAt      time.Time   `json:"added_at" db:"added_at"`
	CompletedAt  *time.Time  `json:"completed_at,omitempty" db:"completed_at"`
	Overview     *string     `json:"overview,omitempty" db:"overview"`
	PosterURL    *string     `json:"poster_url,omitempty" db:"poster_url"`
	Rating       *float64    `json:"rating,omitempty" db:"rating"`
	AutoDownload bool        `json:"auto_download" db:"auto_download"`
}

type TVShow struct {
	ID       int      `json:"id"`
	Status   string   `json:"status"`
	TVmazeID string   `json:"tvmaze_id"`
	Seasons  []Season `json:"seasons"`
}

type Season struct {
	ID           int       `json:"id"`
	ShowID       int       `json:"show_id"`
	SeasonNumber int       `json:"season_number"`
	Episodes     []Episode `json:"episodes"`
}

type Episode struct {
	ID            int         `json:"id"`
	SeasonID      int         `json:"season_id"`
	EpisodeNumber int         `json:"episode_number"`
	Title         string      `json:"title"`
	AirDate       string      `json:"air_date"`
	Status        MediaStatus `json:"status"`
}

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

func (r *MediaRepository) Create(media *Media) error {
	query := `
        INSERT INTO media (type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality, 
                          status, overview, poster_url, rating, auto_download, tv_show_id)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	fmt.Printf("DEBUG: Creating media - Title: %s, Type: %s, TMDB ID: %v, TV Show ID: %v\n",
		media.Title, media.Type, media.TMDBId, media.TVShowID)

	result, err := r.db.Exec(query, media.Type, media.IMDBId, media.TMDBId, media.Title,
		media.Year, media.Language, media.MinQuality, media.MaxQuality, media.Status,
		media.Overview, media.PosterURL, media.Rating, media.AutoDownload, media.TVShowID)

	if err != nil {
		fmt.Printf("ERROR: Insert failed: %v\n", err)
		fmt.Printf("ERROR: Query was: %s\n", query)
		fmt.Printf("ERROR: Values were: %v, %v, %v, %v, %v, %v, %v, %v, %v, %v, %v, %v, %v, %v\n",
			media.Type, media.IMDBId, media.TMDBId, media.Title, media.Year, media.Language,
			media.MinQuality, media.MaxQuality, media.Status, media.Overview, media.PosterURL,
			media.Rating, media.AutoDownload, media.TVShowID)
		return err
	}

	id, _ := result.LastInsertId()
	media.ID = int(id)
	media.AddedAt = time.Now()

	fmt.Printf("DEBUG: Media created successfully with ID: %d\n", media.ID)
	return nil
}

func scanMedia(row interface {
	Scan(dest ...interface{}) error
}) (*Media, error) {
	var m Media
	var tmdbID, tvShowID sql.NullInt64
	var imdbID, torrentHash, torrentName, downloadPath, overview, posterURL sql.NullString
	var completedAt sql.NullTime
	var rating sql.NullFloat64

	err := row.Scan(&m.ID, &m.Type, &imdbID, &tmdbID, &m.Title, &m.Year, &m.Language,
		&m.MinQuality, &m.MaxQuality, &m.Status, &torrentHash, &torrentName,
		&downloadPath, &m.Progress, &m.AddedAt, &completedAt,
		&overview, &posterURL, &rating, &m.AutoDownload, &tvShowID)
	if err != nil {
		return nil, err
	}

	if imdbID.Valid {
		m.IMDBId = imdbID.String
	}
	if tmdbID.Valid {
		val := int(tmdbID.Int64)
		m.TMDBId = &val
	}
	if tvShowID.Valid {
		val := int(tvShowID.Int64)
		m.TVShowID = &val
	}
	if torrentHash.Valid {
		m.TorrentHash = &torrentHash.String
	}
	if torrentName.Valid {
		m.TorrentName = &torrentName.String
	}
	if downloadPath.Valid {
		m.DownloadPath = &downloadPath.String
	}
	if completedAt.Valid {
		m.CompletedAt = &completedAt.Time
	}
	if overview.Valid {
		m.Overview = &overview.String
	}
	if posterURL.Valid {
		m.PosterURL = &posterURL.String
	}
	if rating.Valid {
		m.Rating = &rating.Float64
	}

	return &m, nil
}

func (r *MediaRepository) GetByID(id int) (*Media, error) {
	query := `
        SELECT id, type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality,
               status, torrent_hash, torrent_name, download_path, progress, added_at, completed_at,
               overview, poster_url, rating, auto_download, tv_show_id
        FROM media WHERE id = ?
    `
	row := r.db.QueryRow(query, id)
	media, err := scanMedia(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil if no media found
		}
		return nil, err
	}
	return media, nil
}

func (r *MediaRepository) GetAll() ([]Media, error) {
	query := `
        SELECT id, type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality,
               status, torrent_hash, torrent_name, download_path, progress, added_at, completed_at,
               overview, poster_url, rating, auto_download, tv_show_id
        FROM media ORDER BY added_at DESC
    `

	//fmt.Printf("DEBUG: Executing GetAll query: %s\n", query)

	rows, err := r.db.Query(query)
	if err != nil {
		fmt.Printf("ERROR: Query failed: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	var mediaList []Media
	rowCount := 0

	for rows.Next() {
		rowCount++
		fmt.Printf("DEBUG: Processing row %d\n", rowCount)

		media, err := scanMedia(rows)
		if err != nil {
			fmt.Printf("ERROR: Failed to scan row %d: %v\n", rowCount, err)
			return nil, err
		}

		fmt.Printf("DEBUG: Scanned media - ID: %d, Title: %s, Type: %s, TV Show ID: %v\n",
			media.ID, media.Title, media.Type, media.TVShowID)

		mediaList = append(mediaList, *media)
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("ERROR: Rows iteration error: %v\n", err)
		return nil, err
	}

	fmt.Printf("DEBUG: GetAll returning %d media items\n", len(mediaList))
	return mediaList, nil
}

func (r *MediaRepository) GetByStatus(status MediaStatus) ([]Media, error) {
	query := `
        SELECT id, type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality,
               status, torrent_hash, torrent_name, download_path, progress, added_at, completed_at,
               overview, poster_url, rating, auto_download, tv_show_id
        FROM media WHERE status = ? ORDER BY added_at DESC
    `
	rows, err := r.db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []Media
	for rows.Next() {
		media, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		mediaList = append(mediaList, *media)
	}
	return mediaList, nil
}

func (r *MediaRepository) UpdateStatus(id int, status MediaStatus) error {
	query := `UPDATE media SET status = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, id)
	return err
}

func (r *MediaRepository) UpdateDownloadInfo(id int, status MediaStatus, hash, name *string) error {
	query := `UPDATE media SET status = ?, torrent_hash = ?, torrent_name = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, hash, name, id)
	return err
}

func (r *MediaRepository) UpdateProgress(id int, status MediaStatus, progress float64, completedAt *time.Time) error {
	query := `UPDATE media SET status = ?, progress = ?, completed_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, progress, completedAt, id)
	return err
}

func (r *MediaRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM media WHERE id = ?", id)
	return err
}

// --- TV Show Specific Functions ---

func (r *MediaRepository) CreateTVShow(show *TVShow) error {
	res, err := r.db.Exec("INSERT INTO tv_shows (status, tvmaze_id) VALUES (?, ?)", show.Status, show.TVmazeID)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	show.ID = int(id)
	return nil
}

func (r *MediaRepository) CreateSeason(season *Season) error {
	res, err := r.db.Exec("INSERT INTO seasons (show_id, season_number) VALUES (?, ?)", season.ShowID, season.SeasonNumber)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	season.ID = int(id)
	return nil
}

func (r *MediaRepository) CreateEpisode(episode *Episode) error {
	res, err := r.db.Exec("INSERT INTO episodes (season_id, episode_number, title, air_date, status) VALUES (?, ?, ?, ?, ?)",
		episode.SeasonID, episode.EpisodeNumber, episode.Title, episode.AirDate, episode.Status)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	episode.ID = int(id)
	return nil
}

func (r *MediaRepository) GetTVShowByMediaID(mediaID int) (*TVShow, error) {
	var show TVShow
	// First, get the tv_show_id from the media table
	var tvShowID sql.NullInt64
	err := r.db.QueryRow("SELECT tv_show_id FROM media WHERE id = ?", mediaID).Scan(&tvShowID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No media entry found
		}
		return nil, err
	}

	if !tvShowID.Valid {
		return nil, nil // Not a TV show, return nil without error
	}

	// Now get the show details
	err = r.db.QueryRow("SELECT id, status, tvmaze_id FROM tv_shows WHERE id = ?", tvShowID.Int64).Scan(&show.ID, &show.Status, &show.TVmazeID)
	if err != nil {
		return nil, err
	}

	// Get all seasons first (collect into slice)
	seasonRows, err := r.db.Query("SELECT id, season_number FROM seasons WHERE show_id = ? ORDER BY season_number", show.ID)
	if err != nil {
		return nil, err
	}

	type seasonData struct {
		ID           int
		SeasonNumber int
	}
	var seasons []seasonData

	for seasonRows.Next() {
		var s seasonData
		if err := seasonRows.Scan(&s.ID, &s.SeasonNumber); err != nil {
			seasonRows.Close()
			return nil, err
		}
		seasons = append(seasons, s)
	}
	seasonRows.Close() // Close before starting new queries

	// Now process each season
	for _, seasonInfo := range seasons {
		season := Season{
			ID:           seasonInfo.ID,
			ShowID:       show.ID,
			SeasonNumber: seasonInfo.SeasonNumber,
		}

		// Get episodes for this season
		episodeRows, err := r.db.Query("SELECT id, episode_number, title, air_date, status FROM episodes WHERE season_id = ? ORDER BY episode_number", season.ID)
		if err != nil {
			return nil, err
		}

		for episodeRows.Next() {
			var e Episode
			e.SeasonID = season.ID
			if err := episodeRows.Scan(&e.ID, &e.EpisodeNumber, &e.Title, &e.AirDate, &e.Status); err != nil {
				episodeRows.Close()
				return nil, err
			}
			season.Episodes = append(season.Episodes, e)
		}
		episodeRows.Close()

		show.Seasons = append(show.Seasons, season)
	}

	return &show, nil
}

// UpdateEpisodeDownloadInfo updates a specific episode's download information
func (r *MediaRepository) UpdateEpisodeDownloadInfo(mediaID int, seasonNumber int, episodeNumber int, status MediaStatus, hash, torrentName *string) error {
	// First get the TV show ID from media
	var tvShowID sql.NullInt64
	err := r.db.QueryRow("SELECT tv_show_id FROM media WHERE id = ?", mediaID).Scan(&tvShowID)
	if err != nil {
		return fmt.Errorf("failed to get TV show ID: %w", err)
	}

	if !tvShowID.Valid {
		return fmt.Errorf("media is not a TV show")
	}

	// Get the season ID
	var seasonID int
	err = r.db.QueryRow("SELECT id FROM seasons WHERE show_id = ? AND season_number = ?",
		tvShowID.Int64, seasonNumber).Scan(&seasonID)
	if err != nil {
		return fmt.Errorf("season not found: %w", err)
	}

	// Update the specific episode
	_, err = r.db.Exec(`
		UPDATE episodes 
		SET status = ? 
		WHERE season_id = ? AND episode_number = ?`,
		status, seasonID, episodeNumber)

	if err != nil {
		return fmt.Errorf("failed to update episode status: %w", err)
	}

	// Also update the main media record with the download info (for tracking purposes)
	if hash != nil && torrentName != nil {
		_, err = r.db.Exec(`
			UPDATE media 
			SET torrent_hash = ?, torrent_name = ?, status = ?
			WHERE id = ?`,
			*hash, *torrentName, StatusDownloading, mediaID)
		if err != nil {
			return fmt.Errorf("failed to update media download info: %w", err)
		}
	}

	return nil
}

// GetEpisodeByDetails gets a specific episode by media ID, season, and episode number
func (r *MediaRepository) GetEpisodeByDetails(mediaID int, seasonNumber int, episodeNumber int) (*Episode, error) {
	// First get the TV show ID from media
	var tvShowID sql.NullInt64
	err := r.db.QueryRow("SELECT tv_show_id FROM media WHERE id = ?", mediaID).Scan(&tvShowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get TV show ID: %w", err)
	}

	if !tvShowID.Valid {
		return nil, fmt.Errorf("media is not a TV show")
	}

	// Get the episode
	var episode Episode
	query := `
		SELECT e.id, e.season_id, e.episode_number, e.title, e.air_date, e.status
		FROM episodes e
		JOIN seasons s ON e.season_id = s.id
		WHERE s.show_id = ? AND s.season_number = ? AND e.episode_number = ?`

	err = r.db.QueryRow(query, tvShowID.Int64, seasonNumber, episodeNumber).Scan(
		&episode.ID, &episode.SeasonID, &episode.EpisodeNumber,
		&episode.Title, &episode.AirDate, &episode.Status)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("episode S%02dE%02d not found", seasonNumber, episodeNumber)
		}
		return nil, err
	}

	return &episode, nil
}
