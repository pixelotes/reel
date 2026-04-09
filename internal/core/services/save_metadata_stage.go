package services

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

// SaveMetadataStage downloads poster artwork and generates NFO files
// in the destination directory for media server compatibility (Kodi, Jellyfin, etc.)
type SaveMetadataStage struct {
	config *config.Config
	logger *utils.Logger
}

func NewSaveMetadataStage(cfg *config.Config, logger *utils.Logger) *SaveMetadataStage {
	return &SaveMetadataStage{config: cfg, logger: logger}
}

func (s *SaveMetadataStage) Name() string {
	return "save_metadata"
}

func (s *SaveMetadataStage) Execute(ctx *ProcessingContext) error {
	if ctx.DestinationDir == "" {
		return nil
	}

	s.downloadPoster(ctx)
	s.generateNFO(ctx)

	return nil
}

func (s *SaveMetadataStage) downloadPoster(ctx *ProcessingContext) {
	if ctx.Media.PosterURL == nil || *ctx.Media.PosterURL == "" {
		s.logger.Info("No poster URL available, skipping poster download")
		return
	}

	ext := ".jpg"
	if idx := strings.LastIndex(*ctx.Media.PosterURL, "."); idx != -1 {
		ext = (*ctx.Media.PosterURL)[idx:]
	}

	// Use standard naming: poster.jpg for folders, or movie-name.jpg for movies
	var posterPath string
	if ctx.Media.Type == models.MediaTypeMovie {
		posterPath = filepath.Join(ctx.DestinationDir, "poster"+ext)
	} else {
		// For TV/anime, save in the show root (parent of season folder)
		showDir := ctx.DestinationDir
		if ctx.SeasonNumber > 0 {
			showDir = filepath.Dir(ctx.DestinationDir)
		}
		posterPath = filepath.Join(showDir, "poster"+ext)
	}

	// Don't re-download if already exists
	if _, err := os.Stat(posterPath); err == nil {
		s.logger.Info("Poster already exists, skipping")
		return
	}

	resp, err := http.Get(*ctx.Media.PosterURL)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to download poster: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn(fmt.Sprintf("Poster download returned HTTP %d", resp.StatusCode))
		return
	}

	file, err := os.Create(posterPath)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to create poster file: %v", err))
		return
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to write poster: %v", err))
		os.Remove(posterPath)
		return
	}

	s.logger.Info(fmt.Sprintf("Downloaded poster to: %s", posterPath))
}

// NFO types for Kodi-compatible XML

type movieNFO struct {
	XMLName  xml.Name `xml:"movie"`
	Title    string   `xml:"title"`
	Year     int      `xml:"year"`
	Plot     string   `xml:"plot,omitempty"`
	Rating   float64  `xml:"rating,omitempty"`
	UniqueID []nfoID  `xml:"uniqueid,omitempty"`
	Thumb    string   `xml:"thumb,omitempty"`
}

type tvshowNFO struct {
	XMLName  xml.Name `xml:"tvshow"`
	Title    string   `xml:"title"`
	Year     int      `xml:"year"`
	Plot     string   `xml:"plot,omitempty"`
	Rating   float64  `xml:"rating,omitempty"`
	UniqueID []nfoID  `xml:"uniqueid,omitempty"`
	Thumb    string   `xml:"thumb,omitempty"`
}

type nfoID struct {
	XMLName xml.Name `xml:"uniqueid"`
	Type    string   `xml:"type,attr"`
	Default bool     `xml:"default,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

func (s *SaveMetadataStage) generateNFO(ctx *ProcessingContext) {
	// For episodes, generate show-level NFO in parent dir; skip if already exists
	nfoDir := ctx.DestinationDir
	nfoName := "movie.nfo"

	isShow := ctx.Media.Type == models.MediaTypeTVShow || ctx.Media.Type == models.MediaTypeAnime
	if isShow {
		if ctx.SeasonNumber > 0 {
			nfoDir = filepath.Dir(ctx.DestinationDir)
		}
		nfoName = "tvshow.nfo"
	}

	nfoPath := filepath.Join(nfoDir, nfoName)

	// Don't overwrite existing NFO
	if _, err := os.Stat(nfoPath); err == nil {
		s.logger.Info("NFO already exists, skipping")
		return
	}

	var overview string
	if ctx.Media.Overview != nil {
		overview = *ctx.Media.Overview
	}
	var rating float64
	if ctx.Media.Rating != nil {
		rating = *ctx.Media.Rating
	}
	var posterURL string
	if ctx.Media.PosterURL != nil {
		posterURL = *ctx.Media.PosterURL
	}

	var ids []nfoID
	if ctx.Media.IMDBId != "" {
		ids = append(ids, nfoID{Type: "imdb", Default: true, Value: ctx.Media.IMDBId})
	}
	if ctx.Media.TMDBId != nil {
		ids = append(ids, nfoID{Type: "tmdb", Value: fmt.Sprintf("%d", *ctx.Media.TMDBId)})
	}

	var data []byte
	var err error

	if isShow {
		nfo := tvshowNFO{
			Title:    ctx.Media.Title,
			Year:     ctx.Media.Year,
			Plot:     overview,
			Rating:   rating,
			UniqueID: ids,
			Thumb:    posterURL,
		}
		data, err = xml.MarshalIndent(nfo, "", "  ")
	} else {
		nfo := movieNFO{
			Title:    ctx.Media.Title,
			Year:     ctx.Media.Year,
			Plot:     overview,
			Rating:   rating,
			UniqueID: ids,
			Thumb:    posterURL,
		}
		data, err = xml.MarshalIndent(nfo, "", "  ")
	}

	if err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to generate NFO: %v", err))
		return
	}

	content := xml.Header + string(data) + "\n"

	// Sanitize the filename for FAT32/exFAT
	safeTitle := utils.SanitizeFilename(nfoName)
	nfoPath = filepath.Join(nfoDir, safeTitle)

	if err := os.WriteFile(nfoPath, []byte(content), 0644); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to write NFO: %v", err))
		return
	}

	s.logger.Info(fmt.Sprintf("Generated NFO: %s", nfoPath))
}

func (s *SaveMetadataStage) Rollback(ctx *ProcessingContext) error {
	// Non-critical, don't remove metadata files on rollback
	return nil
}
