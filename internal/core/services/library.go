package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pixelotes/scout"

	"reel/internal/clients/metadata"
	"reel/internal/clients/scoutclient"
	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

type LibraryService struct {
	config      *config.Config
	mediaRepo   *models.MediaRepository
	scout       scout.Backend
	bookClients []metadata.BookClient
	logger      *utils.Logger
}

// NewLibraryService wires the library. Movies, shows and anime go through
// scout; ebooks keep Reel's own book providers.
func NewLibraryService(cfg *config.Config, repo *models.MediaRepository, backend scout.Backend, bookClients []metadata.BookClient, logger *utils.Logger) *LibraryService {
	return &LibraryService{
		config:      cfg,
		mediaRepo:   repo,
		scout:       backend,
		bookClients: bookClients,
		logger:      logger,
	}
}

// metadataBudget bounds one metadata operation; a full TMDB show lookup makes
// one call per season.
const metadataBudget = 60 * time.Second

// lookupItem resolves full details, episodes included: by provider and ID
// when the UI sent them, otherwise by searching the title and taking the best
// match.
func (l *LibraryService) lookupItem(ctx context.Context, mediaType models.MediaType, provider, id, title string, year int) (*scout.Item, error) {
	if l.scout == nil {
		return nil, errors.New("metadata is unavailable (see the startup log)")
	}
	kind, ok := scoutclient.Kind(mediaType)
	if !ok {
		return nil, fmt.Errorf("no metadata kind for %s", mediaType)
	}
	var searchIDs scout.IDs
	if provider == "" || id == "" {
		items, err := l.scout.Search(ctx, scout.Query{
			Kind: kind, Title: title, Year: year, Limit: 1,
			Providers: scoutclient.Providers(l.config, mediaType),
		})
		if err != nil {
			return nil, err
		}
		provider, id, searchIDs = items[0].Provider, items[0].ID, items[0].IDs
	}
	item, err := l.scout.Get(ctx, scout.Lookup{Kind: kind, Provider: provider, ID: id})
	if err != nil {
		return nil, err
	}
	// Keep IDs only the search result carried (e.g. Cinemeta's TMDB ID).
	if item.IDs == nil {
		item.IDs = scout.IDs{}
	}
	for k, v := range searchIDs {
		if _, ok := item.IDs[k]; !ok {
			item.IDs[k] = v
		}
	}
	return item, nil
}

func (l *LibraryService) AddMedia(mediaType models.MediaType, provider, id string, title string, year int, language, minQuality, maxQuality string, autoDownload bool, startSeason, startEpisode int) (*models.Media, error) {
	l.logger.Info("Adding media - Type:", mediaType, "Provider:", provider, "ID:", id, "Title:", title, "Year:", year, "StartSeason:", startSeason, "StartEpisode:", startEpisode)

	var overview, posterURL *string
	var rating *float64
	var item *scout.Item
	var tmdbID *int
	var imdbID string

	if mediaType == models.MediaTypeEbook {
		for _, bc := range l.bookClients {
			books, err := bc.SearchBook(title, "")
			if err != nil || len(books) == 0 {
				continue
			}
			b := books[0]
			overview, posterURL, rating = &b.Overview, &b.PosterURL, &b.Rating
			if year == 0 {
				year = b.Year
			}
			break
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), metadataBudget)
		defer cancel()
		var err error
		item, err = l.lookupItem(ctx, mediaType, provider, id, title, year)
		if err != nil {
			l.logger.Warn("No metadata for", title, ":", err)
			if mediaType != models.MediaTypeMovie {
				// A show without episodes cannot be tracked.
				return nil, fmt.Errorf("could not find %q: %w", title, err)
			}
		} else {
			l.logger.Info("Metadata from", item.Provider, "- ID:", item.ID, "Title:", item.Title, "IDs:", item.IDs)
			overview, posterURL, rating = &item.Overview, &item.PosterURL, &item.Rating
			if title == "" {
				title = item.Title
			}
			if year == 0 {
				year = item.Year
			}
			imdbID = item.IDs["imdb"]
			if n, err := strconv.Atoi(item.IDs["tmdb"]); err == nil {
				tmdbID = &n
			}
		}
	}

	var tvShowID *int
	if (mediaType == models.MediaTypeTVShow || mediaType == models.MediaTypeAnime) && item != nil {
		show := &models.TVShow{Status: item.Status, TVmazeID: item.ID, Provider: item.Provider}
		if err := l.mediaRepo.CreateTVShow(show); err != nil {
			return nil, fmt.Errorf("failed to create tv show/anime: %w", err)
		}
		tvShowID = &show.ID

		for _, seasonNum := range sortedSeasons(item.Seasons) {
			season := &models.Season{ShowID: show.ID, SeasonNumber: seasonNum}
			if err := l.mediaRepo.CreateSeason(season); err != nil {
				return nil, fmt.Errorf("failed to create season %d: %w", seasonNum, err)
			}
			for _, ep := range item.Seasons[seasonNum] {
				status := models.StatusPending
				if ep.AirDate != "" {
					if airDate, err := time.Parse("2006-01-02", ep.AirDate); err == nil && airDate.After(time.Now()) {
						status = models.StatusTBA
					}
				}
				if seasonNum < startSeason || (seasonNum == startSeason && ep.Number < startEpisode) {
					status = models.StatusSkipped
				}
				episode := &models.Episode{
					SeasonID:      season.ID,
					EpisodeNumber: ep.Number,
					Title:         ep.Title,
					AirDate:       ep.AirDate,
					Status:        status,
				}
				if err := l.mediaRepo.CreateEpisode(episode); err != nil {
					return nil, fmt.Errorf("failed to create episode %d: %w", ep.Number, err)
				}
			}
		}
		l.logger.Info("Created show with", len(item.Seasons), "seasons")
	}

	media := &models.Media{
		Type:         mediaType,
		IMDBId:       imdbID,
		TMDBId:       tmdbID,
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
	if err := l.mediaRepo.Create(media); err != nil {
		l.logger.Error("Failed to create media entry:", err, "- Title:", media.Title, "Type:", media.Type, "TMDB ID:", media.TMDBId)
		return nil, fmt.Errorf("failed to create media: %w", err)
	}
	l.logger.Info("Media ID:", media.ID, "Title:", media.Title, "Type:", media.Type)
	return media, nil
}

func sortedSeasons(seasons map[int][]scout.Episode) []int {
	nums := make([]int, 0, len(seasons))
	for n := range seasons {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	return nums
}

func (l *LibraryService) CheckForNewEpisodes() {
	l.logger.Info("Checking for new episodes...")
	media, err := l.mediaRepo.GetAll()
	if err != nil {
		l.logger.Error("Failed to get all media for new episode check:", err)
		return
	}

	for _, item := range media {
		if item.Type == models.MediaTypeTVShow || item.Type == models.MediaTypeAnime {
			if item.Status == models.StatusMonitoring || item.Status == models.StatusPending {
				l.updateShowMetadata(&item)
			}
		}
	}
}

func (l *LibraryService) updateShowMetadata(media *models.Media) {
	l.logger.Info("Updating metadata for show:", media.Title)

	localShow, err := l.mediaRepo.GetTVShowByMediaID(media.ID)
	if err != nil || localShow == nil {
		l.logger.Error("Failed to get local show data for", media.Title, ":", err)
		return
	}

	// Shows added before scout have no provider stored: search by title, as
	// Reel always did.
	ctx, cancel := context.WithTimeout(context.Background(), metadataBudget)
	defer cancel()
	remoteShow, err := l.lookupItem(ctx, media.Type, localShow.Provider, localShow.TVmazeID, media.Title, 0)
	if err != nil {
		l.logger.Error("Failed to fetch remote show data for", media.Title, ":", err)
		return
	}

	// Logic to compare and update seasons and episodes
	for _, seasonNum := range sortedSeasons(remoteShow.Seasons) {
		episodes := remoteShow.Seasons[seasonNum]
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
			if err := l.mediaRepo.CreateSeason(newSeason); err != nil {
				l.logger.Error("Failed to create season", seasonNum, "for", media.Title, ":", err)
				continue
			}
			localShow.Seasons = append(localShow.Seasons, *newSeason)
			localSeason = newSeason
		}

		for _, remoteEpisode := range episodes {
			var localEpisode *models.Episode
			for i := range localSeason.Episodes {
				if localSeason.Episodes[i].EpisodeNumber == remoteEpisode.Number {
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
					EpisodeNumber: remoteEpisode.Number,
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

// SearchMetadata returns candidates for the add-media dialog. Movies, shows
// and anime come from scout (no episode lists; see GetMetadataDetails);
// ebooks from Reel's book providers.
func (l *LibraryService) SearchMetadata(query string, mediaType string) ([]interface{}, error) {
	t := models.MediaType(mediaType)
	var results []interface{}

	if t == models.MediaTypeEbook {
		if len(l.bookClients) == 0 {
			return nil, fmt.Errorf("no ebook metadata provider configured")
		}
		var failures []string
		for _, bc := range l.bookClients {
			books, err := bc.SearchBook(query, "")
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", providerName(bc), err))
				continue
			}
			for _, b := range books {
				results = append(results, b)
			}
			return results, nil
		}
		return nil, fmt.Errorf("all ebook providers failed: %s", strings.Join(failures, "; "))
	}

	kind, ok := scoutclient.Kind(t)
	if !ok {
		return nil, fmt.Errorf("unsupported media type for metadata search: %s", mediaType)
	}
	if l.scout == nil {
		return nil, errors.New("metadata is unavailable (see the startup log)")
	}
	providers := scoutclient.Providers(l.config, t)
	if len(providers) == 0 {
		return nil, fmt.Errorf("no metadata provider configured for '%s'", mediaType)
	}
	ctx, cancel := context.WithTimeout(context.Background(), metadataBudget)
	defer cancel()
	items, err := l.scout.Search(ctx, scout.Query{Kind: kind, Title: query, Limit: 10, Providers: providers})
	if errors.Is(err, scout.ErrNotFound) {
		return []interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		results = append(results, it)
	}
	return results, nil
}

// GetMetadataDetails returns one item with its episode list, for the season
// and episode pickers in the add-media dialog.
func (l *LibraryService) GetMetadataDetails(mediaType, provider, id string) (*scout.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataBudget)
	defer cancel()
	return l.lookupItem(ctx, models.MediaType(mediaType), provider, id, "", 0)
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
	default:
		return "", fmt.Errorf("unknown media type: %s", media.Type)
	}

	safeTitle := utils.SanitizeFilename(media.Title)
	mediaFolderName := fmt.Sprintf("%s (%d)", safeTitle, media.Year)
	fullPath := filepath.Join(baseDestPath, mediaFolderName)

	isSeries := media.Type == models.MediaTypeTVShow || media.Type == models.MediaTypeAnime
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
	if !l.config.Subtitles.Enabled || l.scout == nil {
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
		if media.Type == models.MediaTypeEbook {
			continue
		}

		if media.Type == models.MediaTypeMovie {
			if l.scanSubtitlesForFile(&media, 0, 0, configuredLangs) != nil {
				l.logger.Warn("Subtitle scan: download quota used up, stopping until the next scan")
				return
			}
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
				if l.scanSubtitlesForFile(&media, season.SeasonNumber, ep.EpisodeNumber, configuredLangs) != nil {
					l.logger.Warn("Subtitle scan: download quota used up, stopping until the next scan")
					return
				}
			}
		}
	}

	l.logger.Info("Subtitle scan: complete")
}

// scanSubtitlesForFile returns scout.ErrQuotaExceeded when the scan should
// stop for today.
func (l *LibraryService) scanSubtitlesForFile(media *models.Media, season, episode int, configuredLangs []string) error {
	videoPath, err := l.GetMediaFilePath(media.ID, season, episode)
	if err != nil {
		return nil // file not found on disk, skip silently
	}

	// Languages already on disk, in any of the names Reel recognises.
	existing, _ := l.GetAllSubtitleFiles(media.ID, season, episode)
	have := make(map[string]bool)
	for _, sub := range existing {
		have[sub.Language] = true
	}
	var missing []string
	for _, lang := range configuredLangs {
		if !have[lang] {
			missing = append(missing, lang)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	label := media.Title
	if season > 0 {
		label = fmt.Sprintf("%s S%02dE%02d", media.Title, season, episode)
	}
	l.logger.Info(fmt.Sprintf("Subtitle scan: %s missing [%s]", label, strings.Join(missing, ", ")))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := scoutclient.FetchSubtitles(ctx, l.scout, media, videoPath, season, episode, missing, l.logger); err != nil {
		l.logger.Warn(fmt.Sprintf("Subtitle scan: %s: %v", label, err))
		if errors.Is(err, scout.ErrQuotaExceeded) {
			return err
		}
	}
	return nil
}

// providerName gives a short, readable name for a book client in errors,
// e.g. *metadata.GutendexClient -> "Gutendex".
func providerName(client interface{}) string {
	name := fmt.Sprintf("%T", client)
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSuffix(name, "Client")
}
