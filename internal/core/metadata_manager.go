// internal/core/metadata_manager.go

package core

import (
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reel/internal/clients/metadata"
	"reel/internal/database/models"
	"reel/internal/utils"
	"strconv"
	"strings"
)

// Jellyfin-compatible NFO structures
type MovieNFO struct {
	XMLName       xml.Name `xml:"movie"`
	Title         string   `xml:"title"`
	OriginalTitle string   `xml:"originaltitle"`
	Year          int      `xml:"year"`
	Plot          string   `xml:"plot"`
	Tagline       string   `xml:"tagline,omitempty"`
	IMDBId        string   `xml:"imdbid,omitempty"`
	TMDBId        string   `xml:"tmdbid,omitempty"`
}

type TVShowNFO struct {
	XMLName xml.Name `xml:"tvshow"`
	Title   string   `xml:"title"`
	Plot    string   `xml:"plot"`
	IMDBId  string   `xml:"imdbid,omitempty"`
	TMDBId  string   `xml:"tmdbid,omitempty"`
}

type EpisodeNFO struct {
	XMLName xml.Name `xml:"episodedetails"`
	Title   string   `xml:"title"`
	Season  int      `xml:"season"`
	Episode int      `xml:"episode"`
	Plot    string   `xml:"plot"`
}

type MetadataManager struct {
	logger        *utils.Logger
	tmdbClient    *metadata.TMDBClient
	tvmazeClient  *metadata.TVmazeClient
	anilistClient *metadata.AniListClient
	httpClient    *http.Client
}

func NewMetadataManager(logger *utils.Logger, tmdbClient *metadata.TMDBClient, tvmazeClient *metadata.TVmazeClient, anilistClient *metadata.AniListClient) *MetadataManager {
	return &MetadataManager{
		logger:        logger,
		tmdbClient:    tmdbClient,
		tvmazeClient:  tvmazeClient,
		anilistClient: anilistClient,
		httpClient:    &http.Client{},
	}
}

func (mm *MetadataManager) GenerateAndDownloadMetadata(media *models.Media, destinationPath string, seasonNumber, episodeNumber int) {
	switch media.Type {
	case models.MediaTypeMovie:
		if media.TMDBId == nil {
			mm.logger.Warn("Cannot fetch metadata without a TMDB ID for movie:", media.Title)
			return
		}
		mm.processMovieMetadata(media, destinationPath)
	case models.MediaTypeTVShow:
		mm.processTVShowMetadata(media, destinationPath, seasonNumber, episodeNumber)
	case models.MediaTypeAnime:
		mm.processTVShowMetadata(media, destinationPath, seasonNumber, episodeNumber) // Anime can be treated like a TV show for metadata
	}
}

func (mm *MetadataManager) processMovieMetadata(media *models.Media, destinationPath string) {
	details, err := mm.tmdbClient.GetMovieDetailsByID(*media.TMDBId)
	if err != nil {
		mm.logger.Error("Failed to get movie details from TMDB:", err)
		return
	}

	// Create and save NFO file
	nfo := MovieNFO{
		Title:         details.Title,
		OriginalTitle: details.OriginalTitle,
		Year:          details.Year,
		Plot:          details.Overview,
		Tagline:       details.Tagline,
		IMDBId:        details.IMDBID,
		TMDBId:        strconv.Itoa(*media.TMDBId),
	}
	nfoPath := filepath.Join(destinationPath, "movie.nfo")
	mm.createNFOFile(nfoPath, nfo)

	// Download images
	mm.downloadImage(details.PosterURL, filepath.Join(destinationPath, "poster.jpg"))
	mm.downloadImage(details.BackdropURL, filepath.Join(destinationPath, "fanart.jpg"))
	mm.downloadImage(details.LogoURL, filepath.Join(destinationPath, "logo.png"))
}

func (mm *MetadataManager) processTVShowMetadata(media *models.Media, destinationPath string, seasonNumber, episodeNumber int) {
	// TV Show level NFO and images (only once per show)
	showFolder := filepath.Dir(destinationPath)
	showNFOPath := filepath.Join(showFolder, "tvshow.nfo")

	var details *metadata.TVShowResult
	var err error

	if _, statErr := os.Stat(showNFOPath); os.IsNotExist(statErr) {
		mm.logger.Info("Show metadata not found, fetching for:", media.Title)

		var detailsSlice []*metadata.TVShowResult
		if media.Type == models.MediaTypeAnime {
			detailsSlice, err = mm.anilistClient.SearchAnime(media.Title)
		} else {
			detailsSlice, err = mm.tvmazeClient.SearchTVShow(media.Title)
		}

		if err != nil || len(detailsSlice) == 0 {
			mm.logger.Error("Failed to get show details from provider:", err)
			return
		}
		details = detailsSlice[0]

		nfo := TVShowNFO{
			Title:  details.Title,
			Plot:   details.Overview,
			IMDBId: details.IMDBID,
			// Note: TVmaze/AniList do not provide TMDB ID directly in the current implementation.
		}
		mm.createNFOFile(showNFOPath, nfo)
		mm.downloadImage(details.PosterURL, filepath.Join(showFolder, "poster.jpg"))
		mm.downloadImage(details.BackdropURL, filepath.Join(showFolder, "fanart.jpg"))
		mm.downloadImage(details.LogoURL, filepath.Join(showFolder, "logo.png"))
	} else {
		mm.logger.Info("Show metadata already exists, skipping download for:", media.Title)
	}

	// Episode level NFO - This will always run to create NFO for the specific episode
	if seasonNumber > 0 && episodeNumber > 0 {
		if details == nil { // If we skipped downloading show details, fetch them now for the episode
			var detailsSlice []*metadata.TVShowResult
			if media.Type == models.MediaTypeAnime {
				detailsSlice, err = mm.anilistClient.SearchAnime(media.Title)
			} else {
				detailsSlice, err = mm.tvmazeClient.SearchTVShow(media.Title)
			}
			if err != nil || len(detailsSlice) == 0 {
				mm.logger.Error("Failed to get show details for episode NFO:", err)
				return
			}
			details = detailsSlice[0]
		}

		var episodeDetails *metadata.Episode
		if seasonEpisodes, ok := details.Seasons[seasonNumber]; ok {
			for i := range seasonEpisodes {
				if seasonEpisodes[i].EpisodeNumber == episodeNumber {
					episodeDetails = &seasonEpisodes[i]
					break
				}
			}
		}

		if episodeDetails != nil {
			files, _ := os.ReadDir(destinationPath)
			for _, f := range files {
				if utils.IsVideoFile(f.Name()) {
					baseName := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
					nfo := EpisodeNFO{
						Title:   episodeDetails.Title,
						Season:  seasonNumber,
						Episode: episodeNumber,
						Plot:    episodeDetails.Overview,
					}
					episodeNFOPath := filepath.Join(destinationPath, baseName+".nfo")
					mm.createNFOFile(episodeNFOPath, nfo)
					break
				}
			}
		}
	}
}

func (mm *MetadataManager) createNFOFile(path string, content interface{}) {
	file, err := os.Create(path)
	if err != nil {
		mm.logger.Error("Failed to create NFO file:", err)
		return
	}
	defer file.Close()

	file.WriteString(xml.Header)
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	if err := encoder.Encode(content); err != nil {
		mm.logger.Error("Failed to encode NFO content:", err)
	}
}

func (mm *MetadataManager) downloadImage(url, path string) {
	if url == "" {
		return
	}
	resp, err := mm.httpClient.Get(url)
	if err != nil {
		mm.logger.Error("Failed to download image:", err)
		return
	}
	defer resp.Body.Close()

	file, err := os.Create(path)
	if err != nil {
		mm.logger.Error("Failed to create image file:", err)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		mm.logger.Error("Failed to save image:", err)
	}
}
