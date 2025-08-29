package models

import (
	"database/sql"
	"time"
)

type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeTVShow MediaType = "tvshow"
)

type MediaStatus string

const (
	StatusPending     MediaStatus = "pending"
	StatusSearching   MediaStatus = "searching"
	StatusDownloading MediaStatus = "downloading"
	StatusDownloaded  MediaStatus = "downloaded"
	StatusFailed      MediaStatus = "failed"
)

type Media struct {
	ID           int         `json:"id" db:"id"`
	Type         MediaType   `json:"type" db:"type"`
	IMDBId       string      `json:"imdb_id,omitempty" db:"imdb_id"`
	TMDBId       *int        `json:"tmdb_id" db:"tmdb_id"`
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

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

func (r *MediaRepository) Create(media *Media) error {
	query := `
        INSERT INTO media (type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality, 
                          status, overview, poster_url, rating, auto_download)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	result, err := r.db.Exec(query, media.Type, media.IMDBId, media.TMDBId, media.Title,
		media.Year, media.Language, media.MinQuality, media.MaxQuality, media.Status,
		media.Overview, media.PosterURL, media.Rating, media.AutoDownload)
	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()
	media.ID = int(id)
	media.AddedAt = time.Now()
	return nil
}

func (r *MediaRepository) GetByID(id int) (*Media, error) {
	query := `
        SELECT id, type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality,
               status, torrent_hash, torrent_name, download_path, progress, added_at, completed_at,
               overview, poster_url, rating, auto_download
        FROM media WHERE id = ?
    `
	row := r.db.QueryRow(query, id)

	var m Media
	err := row.Scan(&m.ID, &m.Type, &m.IMDBId, &m.TMDBId, &m.Title, &m.Year, &m.Language,
		&m.MinQuality, &m.MaxQuality, &m.Status, &m.TorrentHash, &m.TorrentName,
		&m.DownloadPath, &m.Progress, &m.AddedAt, &m.CompletedAt,
		&m.Overview, &m.PosterURL, &m.Rating, &m.AutoDownload)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil if no media found
		}
		return nil, err
	}
	return &m, nil
}

func (r *MediaRepository) GetAll() ([]Media, error) {
	query := `
        SELECT id, type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality,
               status, torrent_hash, torrent_name, download_path, progress, added_at, completed_at,
               overview, poster_url, rating, auto_download
        FROM media ORDER BY added_at DESC
    `
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []Media
	for rows.Next() {
		var m Media
		err := rows.Scan(&m.ID, &m.Type, &m.IMDBId, &m.TMDBId, &m.Title, &m.Year, &m.Language,
			&m.MinQuality, &m.MaxQuality, &m.Status, &m.TorrentHash, &m.TorrentName,
			&m.DownloadPath, &m.Progress, &m.AddedAt, &m.CompletedAt,
			&m.Overview, &m.PosterURL, &m.Rating, &m.AutoDownload)
		if err != nil {
			return nil, err
		}
		mediaList = append(mediaList, m)
	}
	return mediaList, nil
}

func (r *MediaRepository) GetByStatus(status MediaStatus) ([]Media, error) {
	query := `
        SELECT id, type, imdb_id, tmdb_id, title, year, language, min_quality, max_quality,
               status, torrent_hash, torrent_name, download_path, progress, added_at, completed_at,
               overview, poster_url, rating, auto_download
        FROM media WHERE status = ? ORDER BY added_at DESC
    `
	rows, err := r.db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []Media
	for rows.Next() {
		var m Media
		err := rows.Scan(&m.ID, &m.Type, &m.IMDBId, &m.TMDBId, &m.Title, &m.Year, &m.Language,
			&m.MinQuality, &m.MaxQuality, &m.Status, &m.TorrentHash, &m.TorrentName,
			&m.DownloadPath, &m.Progress, &m.AddedAt, &m.CompletedAt,
			&m.Overview, &m.PosterURL, &m.Rating, &m.AutoDownload)
		if err != nil {
			return nil, err
		}
		mediaList = append(mediaList, m)
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
