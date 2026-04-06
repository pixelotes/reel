package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"reel/internal/clients/metadata"
	"reel/internal/clients/subtitles"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

type LibraryService struct {
	config          *config.Config
	mediaRepo       *models.MediaRepository
	metadataClients map[models.MediaType][]metadata.Client
	subtitleClient  *subtitles.Client
	logger          *utils.Logger
}

func NewLibraryService(cfg *config.Config, repo *models.MediaRepository, clients map[models.MediaType][]metadata.Client, subClient *subtitles.Client, logger *utils.Logger) *LibraryService {
	return &LibraryService{
		config:          cfg,
		mediaRepo:       repo,
		metadataClients: clients,
		subtitleClient:  subClient,
		logger:          logger,
	}
}

func (l *LibraryService) AddMedia(mediaType models.MediaType, id string, title string, year int, language, minQuality, maxQuality string, autoDownload bool, startSeason, startEpisode int) (*models.Media, error) {
	l.logger.Info("Parameters - Type:", mediaType, "ID:", id, "Title:", title, "Year:", year, "StartSeason:", startSeason, "StartEpisode:", startEpisode)

	var overview, posterURL *string
	var rating *float64
	var tvShowData *metadata.TVShowResult
	var metadataID *int

	l.logger.Info("Looking for metadata providers for type:", mediaType)
	providers := l.metadataClients[mediaType]
	l.logger.Info("Found", len(providers), "metadata providers")

	if len(providers) > 0 {
		client := providers[0]
		l.logger.Info("Using first metadata provider")

		switch mediaType {
		case models.MediaTypeMovie:
			l.logger.Info("Processing movie metadata...")
			movieData, err := client.SearchMovie(title, year)
			if err != nil {
				l.logger.Error("Movie metadata search failed:", err)
			} else if len(movieData) > 0 {
				l.logger.Info("Movie metadata found - ID:", movieData[0].ID, "Title:", movieData[0].Title)
				if tmdbID, parseErr := strconv.Atoi(movieData[0].ID); parseErr == nil {
					metadataID = &tmdbID
					l.logger.Info("Parsed TMDB ID:", *metadataID)
				} else {
					l.logger.Error("Failed to parse TMDB ID:", movieData[0].ID, "Error:", parseErr)
				}
				overview = &movieData[0].Overview
				posterURL = &movieData[0].PosterURL
				rating = &movieData[0].Rating
				if title == "" {
					title = movieData[0].Title
				}
				if year == 0 {
					year = movieData[0].Year
				}
				l.logger.Info("Movie data processed successfully")
			} else {
				l.logger.Warn("No movie metadata found")
			}
		case models.MediaTypeManga:
			l.logger.Info("Processing manga metadata...")
			if mangaClient, ok := client.(metadata.MangaClient); ok {
				mangas, err := mangaClient.SearchManga(title)
				if err != nil {
					l.logger.Error("Manga metadata search failed:", err)
				} else if len(mangas) > 0 {
					manga := mangas[0]
					l.logger.Info("Manga metadata found - Title:", manga.Title)
					overview = &manga.Description
					posterURL = &manga.CoverURL
					if title == "" {
						title = manga.Title
					}
					if year == 0 {
						year = manga.Year
					}

					// Get chapters and build TVShow structure
					langs := l.config.Manga.Languages
					if len(langs) == 0 {
						langs = []string{l.config.Metadata.Language}
					}
					chapters, chErr := mangaClient.GetMangaChapters(manga.ID, langs)
					if chErr == nil && len(chapters) > 0 {
						// Build seasons from volumes, chapters as episodes
						tvShowData = &metadata.TVShowResult{
							ID:      manga.ID,
							Title:   manga.Title,
							Year:    manga.Year,
							Status:  manga.Status,
							Seasons: make(map[int][]metadata.Episode),
						}
						for _, ch := range chapters {
							vol := 1
							if ch.Volume != "" {
								fmt.Sscanf(ch.Volume, "%d", &vol)
								if vol == 0 {
									vol = 1
								}
							}
							var epNum int
							fmt.Sscanf(ch.Chapter, "%d", &epNum)
							if epNum == 0 {
								continue
							}
							tvShowData.Seasons[vol] = append(tvShowData.Seasons[vol], metadata.Episode{
								EpisodeNumber: epNum,
								Title:         ch.Title,
								AirDate:       ch.CreatedAt,
							})
						}
					}
					l.logger.Info("Manga data processed successfully")
				}
			}
		case models.MediaTypeEbook:
			l.logger.Info("Processing ebook metadata...")
			if bookClient, ok := client.(metadata.BookClient); ok {
				bookData, err := bookClient.SearchBook(title, "")
				if err != nil {
					l.logger.Error("Ebook metadata search failed:", err)
				} else if len(bookData) > 0 {
					l.logger.Info("Ebook metadata found - Title:", bookData[0].Title)
					overview = &bookData[0].Overview
					posterURL = &bookData[0].PosterURL
					rating = &bookData[0].Rating
					if title == "" {
						title = bookData[0].Title
					}
					if year == 0 {
						year = bookData[0].Year
					}
					l.logger.Info("Ebook data processed successfully")
				} else {
					l.logger.Warn("No ebook metadata found")
				}
			}
		case models.MediaTypeTVShow, models.MediaTypeAnime:
			l.logger.Info("Processing TV show/anime metadata...")
			tvShowDataSlice, err := client.SearchTVShow(title)
			if err != nil {
				l.logger.Error("TV show/anime metadata search failed:", err)
			} else if len(tvShowDataSlice) > 0 {
				tvShowData = tvShowDataSlice[0]
				l.logger.Info("TV show/anime metadata found - ID:", tvShowData.ID, "Title:", tvShowData.Title)
				overview = &tvShowData.Overview
				posterURL = &tvShowData.PosterURL
				rating = &tvShowData.Rating
				if title == "" {
					title = tvShowData.Title
				}
				if year == 0 {
					year = tvShowData.Year
				}
				l.logger.Info("TV show/anime data processed successfully")
			} else {
				l.logger.Warn("No TV show/anime metadata found")
			}
		}
	}

	var tvShowID *int
	if (mediaType == models.MediaTypeTVShow || mediaType == models.MediaTypeAnime || mediaType == models.MediaTypeManga) && tvShowData != nil {
		l.logger.Info("Creating TV show/anime database entries...")
		show := &models.TVShow{
			Status:   tvShowData.Status,
			TVmazeID: tvShowData.ID, // Using TVmazeID for both for now
		}

		l.logger.Info("Creating TV show/anime record...")
		if err := l.mediaRepo.CreateTVShow(show); err != nil {
			l.logger.Error("CRITICAL: Failed to create TV show/anime:", err)
			return nil, fmt.Errorf("failed to create tv show/anime: %w", err)
		}
		l.logger.Info("TV show/anime created with ID:", show.ID)
		tvShowID = &show.ID

		l.logger.Info("Creating", len(tvShowData.Seasons), "seasons...")
		for seasonNum, episodes := range tvShowData.Seasons {
			l.logger.Info("Creating season", seasonNum, "with", len(episodes), "episodes")
			season := &models.Season{ShowID: show.ID, SeasonNumber: seasonNum}
			if err := l.mediaRepo.CreateSeason(season); err != nil {
				l.logger.Error("CRITICAL: Failed to create season:", seasonNum, "Error:", err)
				return nil, fmt.Errorf("failed to create season: %w", err)
			}
			l.logger.Info("Season", seasonNum, "created with ID:", season.ID)

			for _, ep := range episodes {
				status := models.StatusPending
				if ep.AirDate != "" {
					airDate, _ := time.Parse("2006-01-02", ep.AirDate)
					if airDate.After(time.Now()) {
						status = models.StatusTBA
					}
				}
				if seasonNum < startSeason || (seasonNum == startSeason && ep.EpisodeNumber < startEpisode) {
					status = models.StatusSkipped
				}
				episode := &models.Episode{
					SeasonID:      season.ID,
					EpisodeNumber: ep.EpisodeNumber,
					Title:         ep.Title,
					AirDate:       ep.AirDate,
					Status:        status,
				}
				if err := l.mediaRepo.CreateEpisode(episode); err != nil {
					l.logger.Error("CRITICAL: Failed to create episode:", ep.EpisodeNumber, "Error:", err)
					return nil, fmt.Errorf("failed to create episode: %w", err)
				}
			}
		}
		l.logger.Info("All TV show/anime data created successfully")
	}

	l.logger.Info("Creating main media record...")
	media := &models.Media{
		Type:         mediaType,
		TMDBId:       metadataID,
		TVShowID:     tvShowID,
		Title:        title,
		Year:         year,
		Language:     language,
		MinQuality:   minQuality,
		MaxQuality:   maxQuality,
		Status:       models.StatusPending,
		Overview:     overview,
		PosterURL:    posterURL,
		Rating:       rating,
		AutoDownload: autoDownload,
	}

	l.logger.Info("About to create media record - TMDB ID:", metadataID, "TV Show ID:", tvShowID)

	if err := l.mediaRepo.Create(media); err != nil {
		l.logger.Error("CRITICAL: Failed to create media entry:", err)
		l.logger.Error("Media details - Title:", media.Title, "Type:", media.Type, "TMDB ID:", media.TMDBId, "TV Show ID:", media.TVShowID)
		return nil, fmt.Errorf("failed to create media: %w", err)
	}

	l.logger.Info("Media ID:", media.ID, "Title:", media.Title, "Type:", media.Type)

	return media, nil
}

func (l *LibraryService) CheckForNewEpisodes() {
	l.logger.Info("Checking for new episodes...")
	media, err := l.mediaRepo.GetAll()
	if err != nil {
		l.logger.Error("Failed to get all media for new episode check:", err)
		return
	}

	for _, item := range media {
		if item.Type == models.MediaTypeTVShow || item.Type == models.MediaTypeAnime || item.Type == models.MediaTypeManga {
			if item.Status == models.StatusMonitoring || item.Status == models.StatusPending {
				provider := l.metadataClients[item.Type][0] // Assuming first provider
				l.updateShowMetadata(&item, provider)
			}
		}
	}
}

func (l *LibraryService) updateShowMetadata(media *models.Media, provider metadata.Client) {
	l.logger.Info("Updating metadata for show:", media.Title)
	remoteShowSlice, err := provider.SearchTVShow(media.Title)
	if err != nil {
		l.logger.Error("Failed to fetch remote show data for", media.Title, ":", err)
		return
	}

	if len(remoteShowSlice) == 0 {
		l.logger.Error("No remote show data found for", media.Title)
		return
	}
	remoteShow := remoteShowSlice[0]

	localShow, err := l.mediaRepo.GetTVShowByMediaID(media.ID)
	if err != nil {
		l.logger.Error("Failed to get local show data for", media.Title, ":", err)
		return
	}

	// Logic to compare and update seasons and episodes
	for seasonNum, episodes := range remoteShow.Seasons {
		var localSeason *models.Season
		for i := range localShow.Seasons {
			if localShow.Seasons[i].SeasonNumber == seasonNum {
				localSeason = &localShow.Seasons[i]
				break
			}
		}

		if localSeason == nil {
			// New season
			newSeason := &models.Season{ShowID: localShow.ID, SeasonNumber: seasonNum}
			l.mediaRepo.CreateSeason(newSeason)
			localShow.Seasons = append(localShow.Seasons, *newSeason)
			localSeason = newSeason
		}

		for _, remoteEpisode := range episodes {
			var localEpisode *models.Episode
			for i := range localSeason.Episodes {
				if localSeason.Episodes[i].EpisodeNumber == remoteEpisode.EpisodeNumber {
					localEpisode = &localSeason.Episodes[i]
					break
				}
			}

			if localEpisode == nil {
				// New episode
				status := models.StatusPending
				if remoteEpisode.AirDate != "" {
					airDate, _ := time.Parse("2006-01-02", remoteEpisode.AirDate)
					if airDate.After(time.Now()) {
						status = models.StatusTBA
					}
				}
				newEpisode := &models.Episode{
					SeasonID:      localSeason.ID,
					EpisodeNumber: remoteEpisode.EpisodeNumber,
					Title:         remoteEpisode.Title,
					AirDate:       remoteEpisode.AirDate,
					Status:        status,
				}
				l.mediaRepo.CreateEpisode(newEpisode)
				// If a new episode is found, set the media status to pending
				if media.Status == models.StatusMonitoring {
					l.mediaRepo.UpdateStatus(media.ID, models.StatusPending)
				}
			} else if localEpisode.Status == models.StatusTBA && remoteEpisode.AirDate != "" {
				airDate, _ := time.Parse("2006-01-02", remoteEpisode.AirDate)
				downloadDelay := time.Duration(l.config.Automation.EpisodeDownloadDelayHours) * time.Hour
				if airDate.Add(downloadDelay).Before(time.Now()) {
					l.mediaRepo.UpdateEpisodeDownloadInfo(media.ID, seasonNum, localEpisode.EpisodeNumber, models.StatusPending, nil, nil)
					// If a TBA episode becomes available, set the media status to pending
					if media.Status == models.StatusMonitoring {
						l.mediaRepo.UpdateStatus(media.ID, models.StatusPending)
					}
				}
			}
		}
	}
	l.UpdateShowProgress(media.ID)
}

func (l *LibraryService) UpdateShowProgress(mediaID int) {
	show, err := l.mediaRepo.GetTVShowByMediaID(mediaID)
	if err != nil {
		l.logger.Error("Failed to get show for progress update:", err)
		return
	}
	if show == nil {
		return // Not a show, nothing to do
	}

	var downloadableEpisodes, downloadedEpisodes, pendingEpisodes, downloadingEpisodes, tbaEpisodes, failedEpisodes int

	for _, season := range show.Seasons {
		for _, episode := range season.Episodes {
			// Count episodes for progress calculation
			if episode.Status != models.StatusSkipped && episode.Status != models.StatusTBA {
				downloadableEpisodes++
				if episode.Status == models.StatusDownloaded {
					downloadedEpisodes++
				}
			}
			// Count episodes for status determination
			switch episode.Status {
			case models.StatusPending:
				pendingEpisodes++
			case models.StatusDownloading:
				downloadingEpisodes++
			case models.StatusTBA:
				tbaEpisodes++
			case models.StatusFailed:
				failedEpisodes++
			}
		}
	}

	var progress float64
	if downloadableEpisodes > 0 {
		progress = float64(downloadedEpisodes) / float64(downloadableEpisodes)
	}

	// Determine the new overall status for the media item
	var newStatus models.MediaStatus
	if downloadingEpisodes > 0 {
		newStatus = models.StatusDownloading
	} else if pendingEpisodes > 0 {
		newStatus = models.StatusPending
	} else if failedEpisodes > 0 && downloadedEpisodes == 0 {
		newStatus = models.StatusFailed
	} else {
		if tbaEpisodes > 0 || strings.ToLower(show.Status) == "running" {
			newStatus = models.StatusMonitoring
		} else {
			newStatus = models.StatusDownloaded
		}
	}

	// Use the generic UpdateProgress which now handles status correctly
	l.mediaRepo.UpdateProgress(mediaID, newStatus, progress, nil)
	l.logger.Info("Updated show progress for Media ID", mediaID, "New Status:", newStatus, "Progress:", progress)
}

// GetPendingMediaForProcessing gathers all media items that need processing (Pending, Failed, Series with failures)
func (l *LibraryService) GetPendingMediaForProcessing() []models.Media {
	pendingMedia, err := l.mediaRepo.GetByStatus(models.StatusPending)
	if err != nil {
		l.logger.Error("Failed to get pending media:", err)
	}

	failedMedia, err := l.mediaRepo.GetByStatus(models.StatusFailed)
	if err != nil {
		l.logger.Error("Failed to get failed media:", err)
	}

	seriesWithFailedEpisodes, err := l.mediaRepo.GetSeriesWithFailedEpisodes()
	if err != nil {
		l.logger.Error("Failed to get series with failed episodes:", err)
	}

	seriesWithPendingEpisodes, err := l.mediaRepo.GetSeriesWithPendingEpisodes()
	if err != nil {
		l.logger.Error("Failed to get series with pending episodes:", err)
	}

	// Use a map to collect and de-duplicate all media items that need processing.
	mediaMap := make(map[int]models.Media)
	for _, item := range pendingMedia {
		mediaMap[item.ID] = item
	}
	for _, item := range failedMedia {
		mediaMap[item.ID] = item
	}
	for _, item := range seriesWithFailedEpisodes {
		mediaMap[item.ID] = item
	}
	for _, item := range seriesWithPendingEpisodes {
		mediaMap[item.ID] = item
	}

	var result []models.Media
	if len(mediaMap) > 0 {
		l.logger.Info(fmt.Sprintf("Processing %d media items (pending, failed series, and series with failed episodes).", len(mediaMap)))
		for _, media := range mediaMap {
			if media.AutoDownload {
				result = append(result, media)
			}
		}
	}
	return result
}

// SearchMetadata searches for metadata using configured providers
func (l *LibraryService) SearchMetadata(query string, mediaType string) ([]interface{}, error) {
	providers := l.metadataClients[models.MediaType(mediaType)]
	if len(providers) == 0 {
		return nil, fmt.Errorf("no metadata provider configured for '%s'", mediaType)
	}

	// Try each provider in order, fall back to next on error
	var lastErr error
	for _, client := range providers {
		var results []interface{}
		var err error

		switch mediaType {
		case string(models.MediaTypeMovie):
			var res []*metadata.MovieResult
			res, err = client.SearchMovie(query, 0)
			if err == nil {
				for _, r := range res {
					results = append(results, r)
				}
			}
		case string(models.MediaTypeTVShow), string(models.MediaTypeAnime):
			var res []*metadata.TVShowResult
			res, err = client.SearchTVShow(query)
			if err == nil {
				for _, r := range res {
					results = append(results, r)
				}
			}
		case string(models.MediaTypeManga):
			if mangaClient, ok := client.(metadata.MangaClient); ok {
				var res []*metadata.MangaResult
				res, err = mangaClient.SearchManga(query)
				if err == nil {
					for _, r := range res {
						results = append(results, r)
					}
				}
			} else {
				continue
			}
		case string(models.MediaTypeEbook):
			if bookClient, ok := client.(metadata.BookClient); ok {
				var res []*metadata.BookResult
				res, err = bookClient.SearchBook(query, "")
				if err == nil {
					for _, r := range res {
						results = append(results, r)
					}
				}
			} else {
				continue // This provider doesn't support books, try next
			}
		default:
			return nil, fmt.Errorf("unsupported media type for metadata search: %s", mediaType)
		}

		if err != nil {
			l.logger.Warn("Metadata provider failed, trying next:", err)
			lastErr = err
			continue
		}

		return results, nil
	}

	return nil, fmt.Errorf("all metadata providers failed, last error: %w", lastErr)
}

func (l *LibraryService) GetMediaFilePath(mediaID int, seasonNumber int, episodeNumber int) (string, error) {
	media, err := l.mediaRepo.GetByID(mediaID)
	if err != nil {
		return "", err
	}
	if media == nil {
		return "", fmt.Errorf("media with ID %d not found", mediaID)
	}

	var baseDestPath string
	switch media.Type {
	case models.MediaTypeMovie:
		baseDestPath = l.config.Movies.DestinationFolder
	case models.MediaTypeTVShow:
		baseDestPath = l.config.TVShows.DestinationFolder
	case models.MediaTypeAnime:
		baseDestPath = l.config.Anime.DestinationFolder
	case models.MediaTypeEbook:
		baseDestPath = l.config.Ebooks.DestinationFolder
	case models.MediaTypeManga:
		baseDestPath = l.config.Manga.DestinationFolder
	default:
		return "", fmt.Errorf("unknown media type: %s", media.Type)
	}

	safeTitle := utils.SanitizeFilename(media.Title)
	mediaFolderName := fmt.Sprintf("%s (%d)", safeTitle, media.Year)
	fullPath := filepath.Join(baseDestPath, mediaFolderName)

	isSeries := media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime || media.Type == models.MediaTypeManga
	isSeasonFoldered := media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime
	if isSeasonFoldered {
		if seasonNumber <= 0 {
			return "", fmt.Errorf("season number must be provided for TV shows")
		}
		fullPath = filepath.Join(fullPath, fmt.Sprintf("S%02d", seasonNumber))
	}

	files, err := os.ReadDir(fullPath)
	if err != nil {
		return "", fmt.Errorf("could not read destination directory '%s': %w", fullPath, err)
	}

	videoExtensions := map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".mov": true}
	ebookExtensions := map[string]bool{".epub": true, ".pdf": true, ".mobi": true, ".azw3": true, ".fb2": true}
	mangaExtensions := map[string]bool{".cbz": true, ".cbr": true}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name()))
		switch media.Type {
		case models.MediaTypeEbook:
			if ebookExtensions[ext] {
				return filepath.Join(fullPath, file.Name()), nil
			}
		case models.MediaTypeManga:
			if mangaExtensions[ext] {
				if episodeNumber <= 0 {
					return filepath.Join(fullPath, file.Name()), nil
				}
				upperName := strings.ToUpper(file.Name())
				// Match renamed files: S01E01, S001E001, etc.
				episodePatterns := []string{
					fmt.Sprintf("S%02dE%02d", seasonNumber, episodeNumber),
					fmt.Sprintf("S%03dE%03d", seasonNumber, episodeNumber),
					fmt.Sprintf("S%dE%d", seasonNumber, episodeNumber),
				}
				for _, p := range episodePatterns {
					if strings.Contains(upperName, p) {
						return filepath.Join(fullPath, file.Name()), nil
					}
				}
				// Also match "Chapter XX" format (pre-rename or custom templates)
				chapterPattern := fmt.Sprintf("Chapter %02d", episodeNumber)
				if strings.Contains(file.Name(), chapterPattern) {
					return filepath.Join(fullPath, file.Name()), nil
				}
			}
		default:
			if videoExtensions[ext] {
				if isSeries {
					if episodeNumber <= 0 {
						return "", fmt.Errorf("episode number must be provided for TV shows")
					}
					episodePattern := fmt.Sprintf("S%02dE%02d", seasonNumber, episodeNumber)
					if strings.Contains(strings.ToUpper(file.Name()), episodePattern) {
						return filepath.Join(fullPath, file.Name()), nil
					}
				} else {
					return filepath.Join(fullPath, file.Name()), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no media file found in %s", fullPath)
}

func (l *LibraryService) GetAllSubtitleFiles(mediaID int, seasonNumber int, episodeNumber int) ([]SubtitleTrack, error) {
	videoPath, err := l.GetMediaFilePath(mediaID, seasonNumber, episodeNumber)
	if err != nil {
		return nil, err
	}

	baseName := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	videoDir := filepath.Dir(videoPath)

	// Read all files in the directory
	files, err := os.ReadDir(videoDir)
	if err != nil {
		return nil, fmt.Errorf("could not read video directory: %w", err)
	}

	var subtitles []SubtitleTrack
	foundEnglish := false

	// First pass: look for language-specific subtitle files
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		fileExt := filepath.Ext(fileName)

		// Only process .srt files
		if strings.ToLower(fileExt) != ".srt" {
			continue
		}

		// Check if this subtitle file belongs to our video
		fileBaseName := strings.TrimSuffix(fileName, fileExt)
		videoBaseName := filepath.Base(baseName)

		// Skip if this subtitle doesn't match our video file
		if !strings.HasPrefix(fileBaseName, videoBaseName) {
			continue
		}

		// Extract language code from filename
		// Expected format: videoname.lang.srt or videoname.srt
		parts := strings.Split(fileBaseName, ".")

		var langCode string
		var label string

		// Helper function getLanguageLabel needed.
		// Since it was private in Manager, I should add it here too or make it a method.

		if len(parts) >= 2 && parts[len(parts)-1] != videoBaseName {
			// Has language code: videoname.en.srt
			langCode = parts[len(parts)-1]
			label = getLanguageLabel(langCode)
		} else if fileName == videoBaseName+".srt" {
			// Default subtitle file without language code
			langCode = "default"
			label = "Default"
		} else {
			// Skip files that don't match our expected pattern
			continue
		}

		if langCode == "en" || langCode == "eng" {
			foundEnglish = true
		}

		subtitles = append(subtitles, SubtitleTrack{
			Language: langCode,
			Label:    label,
			FilePath: filepath.Join(videoDir, fileName),
		})
	}

	// Second pass: if no English subtitle found, include the default one as English
	if !foundEnglish {
		defaultSubPath := baseName + ".srt"
		if _, err := os.Stat(defaultSubPath); err == nil {
			// Check if we already added this as "default" and update it
			for i, sub := range subtitles {
				if sub.Language == "default" {
					subtitles[i].Language = "en"
					subtitles[i].Label = "English (Default)"
					foundEnglish = true
					break
				}
			}
		}
	}

	// Sort subtitles: English first, then alphabetically by label
	sort.Slice(subtitles, func(i, j int) bool {
		if subtitles[i].Language == "en" {
			return true
		}
		if subtitles[j].Language == "en" {
			return false
		}
		return subtitles[i].Label < subtitles[j].Label
	})

	return subtitles, nil
}

// Helper function to convert language codes to readable labels
func getLanguageLabel(langCode string) string {
	languageMap := map[string]string{
		"en": "English",
		"es": "Spanish",
		"fr": "French",
		"de": "German",
		"it": "Italian",
		"pt": "Portuguese",
		"ru": "Russian",
		"ja": "Japanese",
		"ko": "Korean",
		"zh": "Chinese",
		"ar": "Arabic",
		"hi": "Hindi",
		"th": "Thai",
		"tr": "Turkish",
		"pl": "Polish",
		"nl": "Dutch",
		"sv": "Swedish",
		"da": "Danish",
		"no": "Norwegian",
		"fi": "Finnish",
		"cs": "Czech",
		"hu": "Hungarian",
		"ro": "Romanian",
		"bg": "Bulgarian",
		"hr": "Croatian",
		"sk": "Slovak",
		"sl": "Slovenian",
		"et": "Estonian",
		"lv": "Latvian",
		"lt": "Lithuanian",
		"uk": "Ukrainian",
		"be": "Belarusian",
		"mk": "Macedonian",
		"sr": "Serbian",
		"bs": "Bosnian",
		"me": "Montenegrin",
		"sq": "Albanian",
		"el": "Greek",
		"he": "Hebrew",
		"fa": "Persian",
		"ur": "Urdu",
		"bn": "Bengali",
		"ta": "Tamil",
		"te": "Telugu",
		"ml": "Malayalam",
		"kn": "Kannada",
		"gu": "Gujarati",
		"pa": "Punjabi",
		"mr": "Marathi",
		"ne": "Nepali",
		"si": "Sinhala",
		"my": "Burmese",
		"km": "Khmer",
		"lo": "Lao",
		"vi": "Vietnamese",
		"id": "Indonesian",
		"ms": "Malay",
		"tl": "Filipino",
		"sw": "Swahili",
		"am": "Amharic",
		"yo": "Yoruba",
		"ig": "Igbo",
		"ha": "Hausa",
		"zu": "Zulu",
		"af": "Afrikaans",
		"ca": "Catalan",
		"eu": "Basque",
		"gl": "Galician",
		"cy": "Welsh",
		"ga": "Irish",
		"gd": "Scottish Gaelic",
		"is": "Icelandic",
		"fo": "Faroese",
		"mt": "Maltese",
		"lb": "Luxembourgish",
	}

	if label, exists := languageMap[langCode]; exists {
		return label
	}

	// If not found in map, return the code in uppercase
	return strings.ToUpper(langCode)
}

func (l *LibraryService) GetMetadataClients() []string {
	// Let's use config directly
	var clients []string
	configProviders := append([]string{}, l.config.Movies.Providers...)
	configProviders = append(configProviders, l.config.TVShows.Providers...)
	configProviders = append(configProviders, l.config.Anime.Providers...)

	// Dedup
	seen := make(map[string]bool)
	for _, p := range configProviders {
		if !seen[p] {
			seen[p] = true
			clients = append(clients, p)
		}
	}
	return clients
}

func (l *LibraryService) DeleteMedia(id int) error {
	return l.mediaRepo.Delete(id)
}

func (l *LibraryService) RetryMedia(id int) error {
	media, err := l.mediaRepo.GetByID(id)
	if err != nil {
		return err
	}
	if media == nil {
		return fmt.Errorf("media with id %d not found", id)
	}

	if media.Status == models.StatusFailed {
		if err := l.mediaRepo.UpdateStatus(media.ID, models.StatusPending); err != nil {
			return err
		}
	}
	// Note: Caller (Manager) must enqueue this if needed.
	return nil
}

func (l *LibraryService) ClearFailedMedia() error {
	failedMedia, err := l.mediaRepo.GetByStatus(models.StatusFailed)
	if err != nil {
		return err
	}
	for _, media := range failedMedia {
		if err := l.mediaRepo.Delete(media.ID); err != nil {
			l.logger.Error("failed to delete media %d: %v", media.ID, err)
		}
	}
	return nil
}

func (l *LibraryService) GetTVShowDetails(mediaID int) (*models.TVShow, error) {
	return l.mediaRepo.GetTVShowByMediaID(mediaID)
}

func (l *LibraryService) UpdateMediaSettings(id int, minQuality, maxQuality string, autoDownload bool) error {
	l.logger.Info(fmt.Sprintf("Updating settings for media ID %d: minQ=%s, maxQ=%s, auto=%t", id, minQuality, maxQuality, autoDownload))
	return l.mediaRepo.UpdateSettings(id, minQuality, maxQuality, autoDownload)
}

func (l *LibraryService) GetAnimeSearchTerms(mediaID int) ([]models.AnimeSearchTerm, error) {
	return l.mediaRepo.GetAnimeSearchTerms(mediaID)
}

func (l *LibraryService) AddAnimeSearchTerm(mediaID int, term string) (*models.AnimeSearchTerm, error) {
	return l.mediaRepo.AddAnimeSearchTerm(mediaID, term)
}

func (l *LibraryService) DeleteAnimeSearchTerm(id int) error {
	return l.mediaRepo.DeleteAnimeSearchTerm(id)
}

// ScanMissingSubtitles iterates all downloaded media and downloads subtitles
// for configured languages that are not yet present on disk.
func (l *LibraryService) ScanMissingSubtitles() {
	if !l.config.Subtitles.Enabled || l.subtitleClient == nil {
		return
	}

	configuredLangs := l.config.Subtitles.Languages
	if len(configuredLangs) == 0 {
		return
	}

	downloaded, err := l.mediaRepo.GetByStatus(models.StatusDownloaded)
	if err != nil {
		l.logger.Error("Subtitle scan: failed to get downloaded media:", err)
		return
	}

	l.logger.Info(fmt.Sprintf("Subtitle scan: checking %d media items for missing subtitles", len(downloaded)))

	for _, media := range downloaded {
		if media.Type == models.MediaTypeEbook || media.Type == models.MediaTypeManga {
			continue
		}

		if media.Type == models.MediaTypeMovie {
			l.scanSubtitlesForFile(&media, 0, 0, configuredLangs)
			continue
		}

		// TV Show / Anime — iterate episodes
		if media.TVShowID == nil {
			continue
		}
		show, err := l.mediaRepo.GetTVShowByMediaID(media.ID)
		if err != nil || show == nil {
			continue
		}
		for _, season := range show.Seasons {
			for _, ep := range season.Episodes {
				if ep.Status != models.StatusDownloaded {
					continue
				}
				l.scanSubtitlesForFile(&media, season.SeasonNumber, ep.EpisodeNumber, configuredLangs)
			}
		}
	}

	l.logger.Info("Subtitle scan: complete")
}

func (l *LibraryService) scanSubtitlesForFile(media *models.Media, season, episode int, configuredLangs []string) {
	videoPath, err := l.GetMediaFilePath(media.ID, season, episode)
	if err != nil {
		return // file not found on disk, skip silently
	}

	// Check which languages are already present
	existing, _ := l.GetAllSubtitleFiles(media.ID, season, episode)
	existingLangs := make(map[string]bool)
	for _, sub := range existing {
		existingLangs[sub.Language] = true
	}

	// Find missing languages
	var missing []string
	for _, lang := range configuredLangs {
		if !existingLangs[lang] {
			missing = append(missing, lang)
		}
	}

	if len(missing) == 0 {
		return
	}

	label := media.Title
	if season > 0 {
		label = fmt.Sprintf("%s S%02dE%02d", media.Title, season, episode)
	}
	l.logger.Info(fmt.Sprintf("Subtitle scan: %s missing [%s]", label, strings.Join(missing, ", ")))

	subs, err := l.subtitleClient.Search(videoPath, media, season, episode)
	if err != nil || len(subs) == 0 {
		return
	}

	baseName := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	for i, lang := range missing {
		if i > 0 {
			time.Sleep(20 * time.Second)
		}
		for _, sub := range subs {
			if sub.Language == lang {
				subPath := fmt.Sprintf("%s.%s.srt", baseName, lang)
				if err := l.subtitleClient.Download(sub, subPath); err != nil {
					l.logger.Warn(fmt.Sprintf("Subtitle scan: failed to download %s sub for %s: %v", lang, label, err))
				} else {
					l.logger.Info(fmt.Sprintf("Subtitle scan: downloaded %s sub for %s", lang, label))
				}
				break
			}
		}
	}
}
