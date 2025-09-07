package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"reel/internal/clients/indexers"
	"reel/internal/config"
	"reel/internal/core"
	"reel/internal/database/models"
	"reel/internal/utils"

	"github.com/gorilla/mux"
)

type APIHandler struct {
	manager *core.Manager
	logger  *utils.Logger
	config  *config.Config
}

// A helper function to respond with JSON
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

// A helper function to respond with a JSON error
func respondError(w http.ResponseWriter, code int, message string) {
	respondJSON(w, code, map[string]string{"error": message})
}

func NewAPIHandler(manager *core.Manager, logger *utils.Logger, config *config.Config) *APIHandler {
	return &APIHandler{manager: manager, logger: logger, config: config}
}

// Login endpoint
func (h *APIHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.Password != h.config.App.UIPassword {
		respondError(w, http.StatusUnauthorized, "Incorrect password")
		return
	}

	// In a real implementation, validate password against config
	// For now, just generate a simple JWT token
	token := generateJWTToken(req.Password) // Implement JWT generation

	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

// Get all media
func (h *APIHandler) GetMedia(w http.ResponseWriter, r *http.Request) {

	media, err := h.manager.GetAllMedia()
	if err != nil {
		h.logger.Error("CRITICAL: Failed to fetch media from manager:", err)
		respondError(w, http.StatusInternalServerError, "Failed to fetch media")
		return
	}

	//h.logger.Info("GetMedia: Retrieved", len(media), "media items from manager")

	// Log each media item for debugging
	//for i, m := range media {
	//h.logger.Info("Media", i, "- ID:", m.ID, "Title:", m.Title, "Type:", m.Type, "TV Show ID:", m.TVShowID)
	//}

	respondJSON(w, http.StatusOK, media)
	//h.logger.Info("GetMedia: Response sent successfully")
}

// Add new media
func (h *APIHandler) AddMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type         string `json:"type"`
		Title        string `json:"title"`
		Year         int    `json:"year"`
		ID           string `json:"id"`
		Language     string `json:"language"`
		MinQuality   string `json:"min_quality"`
		MaxQuality   string `json:"max_quality"`
		AutoDownload bool   `json:"auto_download"`
		StartSeason  int    `json:"start_season"`
		StartEpisode int    `json:"start_episode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode add media request:", err)
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Log the request for debugging
	h.logger.Info("Adding media request:", req.Type, req.Title, req.Year)

	// Validate required fields
	if req.Type == "" || req.Title == "" {
		h.logger.Error("Missing required fields - Type:", req.Type, "Title:", req.Title)
		respondError(w, http.StatusBadRequest, "Type and Title are required")
		return
	}

	mediaType := models.MediaType(req.Type)

	// Add detailed logging before the database operation
	h.logger.Info("Creating media with type:", mediaType, "title:", req.Title)

	media, err := h.manager.AddMedia(mediaType, req.ID, req.Title, req.Year,
		req.Language, req.MinQuality, req.MaxQuality, req.AutoDownload, req.StartSeason, req.StartEpisode)

	if err != nil {
		// Log the full error details
		h.logger.Error("Failed to add media - Title:", req.Title, "Error:", err)

		// Check if it's a database constraint error
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "unique constraint") {
			respondError(w, http.StatusConflict, "Media already exists in library")
			return
		}

		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.logger.Info("Successfully added media:", media.Title, "ID:", media.ID)
	respondJSON(w, http.StatusCreated, media)
}

// Delete media
func (h *APIHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	if err := h.manager.DeleteMedia(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete media")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Retry failed media
func (h *APIHandler) RetryMedia(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	if err := h.manager.RetryMedia(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retry media")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Search metadata (TMDB/OMDB)
func (h *APIHandler) SearchMetadata(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	mediaType := r.URL.Query().Get("type")

	if query == "" {
		respondError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	results, err := h.manager.SearchMetadata(query, mediaType)
	if err != nil {
		h.logger.Error("Metadata search failed:", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, results)
}

// System status
func (h *APIHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.manager.GetSystemStatus()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get system status")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

// Test connections
func (h *APIHandler) TestIndexer(w http.ResponseWriter, r *http.Request) {
	indexerKey := r.URL.Query().Get("indexer")
	if indexerKey == "" {
		respondError(w, http.StatusBadRequest, "indexer parameter is required")
		return
	}

	ok, err := h.manager.TestIndexerConnection(indexerKey)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": ok})
}

func (h *APIHandler) TestTorrent(w http.ResponseWriter, r *http.Request) {
	ok, err := h.manager.TestTorrentConnection()
	if err != nil {
		h.logger.Error("Torrent connection test failed:", err)
		// Even if there's an error, we can still return ok: false
		respondJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": ok})
}

// Clear failed media
func (h *APIHandler) ClearFailed(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.ClearFailedMedia(); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to clear failed media")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *APIHandler) ManualSearch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	results, err := h.manager.PerformSearch(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, results)
}

func (h *APIHandler) ManualDownload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	var req indexers.IndexerResult
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.manager.StartDownload(id, req); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *APIHandler) GetTVShowDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	show, err := h.manager.GetTVShowDetails(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, show)
}

func generateJWTToken(password string) string {
	// Simple token generation - implement proper JWT in production
	return "simple-token-" + password
}

func (h *APIHandler) EpisodeSearch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mediaID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	seasonStr := vars["season"]
	episodeStr := vars["episode"]

	season, err := strconv.Atoi(seasonStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid season number")
		return
	}

	episode, err := strconv.Atoi(episodeStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid episode number")
		return
	}

	h.logger.Info(fmt.Sprintf("Manual episode search requested for media %d S%02dE%02d", mediaID, season, episode))

	results, err := h.manager.PerformEpisodeSearch(mediaID, season, episode)
	if err != nil {
		h.logger.Error("Episode search failed:", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.logger.Info(fmt.Sprintf("Episode search completed: found %d results", len(results)))
	respondJSON(w, http.StatusOK, results)
}

// Manual download for a specific episode
func (h *APIHandler) EpisodeDownload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mediaID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	seasonStr := vars["season"]
	episodeStr := vars["episode"]

	season, err := strconv.Atoi(seasonStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid season number")
		return
	}

	episode, err := strconv.Atoi(episodeStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid episode number")
		return
	}

	var req indexers.IndexerResult
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.logger.Info(fmt.Sprintf("Manual episode download requested for media %d S%02dE%02d: %s",
		mediaID, season, episode, req.Title))

	if err := h.manager.StartEpisodeDownload(mediaID, season, episode, req); err != nil {
		h.logger.Error("Episode download failed:", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.logger.Info("Episode download started successfully")
	respondJSON(w, http.StatusOK, map[string]string{"status": "download started"})
}

// Get episode details
func (h *APIHandler) GetEpisodeDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mediaID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	seasonStr := vars["season"]
	episodeStr := vars["episode"]

	season, err := strconv.Atoi(seasonStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid season number")
		return
	}

	episode, err := strconv.Atoi(episodeStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid episode number")
		return
	}

	// You'll need to implement this method in MediaRepository if you want episode details
	// For now, we'll just return the episode info from the TV show details
	show, err := h.manager.GetTVShowDetails(mediaID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if show == nil {
		respondError(w, http.StatusNotFound, "TV show not found")
		return
	}

	// Find the specific episode
	for _, s := range show.Seasons {
		if s.SeasonNumber == season {
			for _, e := range s.Episodes {
				if e.EpisodeNumber == episode {
					respondJSON(w, http.StatusOK, e)
					return
				}
			}
		}
	}

	respondError(w, http.StatusNotFound, "Episode not found")
}

// StreamVideo handles serving the video file for playback.
func (h *APIHandler) StreamVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mediaID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	seasonNumber, _ := strconv.Atoi(r.URL.Query().Get("season"))
	episodeNumber, _ := strconv.Atoi(r.URL.Query().Get("episode"))

	filePath, err := h.manager.GetMediaFilePath(mediaID, seasonNumber, episodeNumber)
	if err != nil {
		h.logger.Error("Could not get media file path:", err)
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	http.ServeFile(w, r, filePath)
}

// GetSubtitles handles finding, converting, and serving the subtitle file.
func (h *APIHandler) GetSubtitles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mediaID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	seasonNumber, _ := strconv.Atoi(r.URL.Query().Get("season"))
	episodeNumber, _ := strconv.Atoi(r.URL.Query().Get("episode"))
	requestedLang := r.URL.Query().Get("lang")
	if requestedLang == "" {
		requestedLang = "en" // Default to English
	}

	// Get all available subtitles
	subtitles, err := h.manager.GetAllSubtitleFiles(mediaID, seasonNumber, episodeNumber)
	if err != nil {
		h.logger.Debug("No subtitle files found:", err)
		respondError(w, http.StatusNotFound, "No subtitle files found")
		return
	}

	if len(subtitles) == 0 {
		respondError(w, http.StatusNotFound, "No subtitle files found")
		return
	}

	// Find the requested language
	var selectedSubtitle *core.SubtitleTrack
	for _, sub := range subtitles {
		if sub.Language == requestedLang {
			selectedSubtitle = &sub
			break
		}
	}

	// If requested language not found, use the first available (which should be English or default)
	if selectedSubtitle == nil {
		selectedSubtitle = &subtitles[0]
		h.logger.Debug("Requested language not found, using:", selectedSubtitle.Language)
	}

	// Convert SRT to VTT
	vttContent, err := utils.ConvertSRTToVTT(selectedSubtitle.FilePath)
	if err != nil {
		h.logger.Error("Failed to convert SRT to VTT:", err)
		respondError(w, http.StatusInternalServerError, "Failed to process subtitles")
		return
	}

	// Set headers for proper caching and content type
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	w.Header().Set("Access-Control-Allow-Origin", "*")      // Allow CORS for subtitles

	// Serve the converted content
	http.ServeContent(w, r, "subtitles.vtt", time.Now(), vttContent)
}

// Add a new endpoint to get all available subtitle languages
func (h *APIHandler) GetAvailableSubtitles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mediaID, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	seasonNumber, _ := strconv.Atoi(r.URL.Query().Get("season"))
	episodeNumber, _ := strconv.Atoi(r.URL.Query().Get("episode"))

	// Get all available subtitles
	subtitles, err := h.manager.GetAllSubtitleFiles(mediaID, seasonNumber, episodeNumber)
	if err != nil {
		h.logger.Debug("No subtitle files found:", err)
		t := []string{}
		respondJSON(w, http.StatusOK, t) // Return empty array instead of error
		return
	}

	// Return the list of available subtitles (without file paths)
	respondJSON(w, http.StatusOK, subtitles)
}

// UpdateMediaSettings handles updating a media item's settings.
func (h *APIHandler) UpdateMediaSettings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	var req struct {
		MinQuality   string `json:"min_quality"`
		MaxQuality   string `json:"max_quality"`
		AutoDownload bool   `json:"auto_download"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.manager.UpdateMediaSettings(id, req.MinQuality, req.MaxQuality, req.AutoDownload); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "settings updated successfully"})
}

// This handler gets the config
func (h *APIHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	configContent, err := h.manager.GetConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to read config file")
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(configContent))
}

func (h *APIHandler) GetAnimeSearchTerms(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	terms, err := h.manager.GetAnimeSearchTerms(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get search terms")
		return
	}
	if terms == nil {
		respondJSON(w, http.StatusOK, []models.AnimeSearchTerm{})
		return
	}

	respondJSON(w, http.StatusOK, terms)
}

func (h *APIHandler) AddAnimeSearchTerm(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	var req struct {
		Term string `json:"term"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	term, err := h.manager.AddAnimeSearchTerm(id, req.Term)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to add search term")
		return
	}

	respondJSON(w, http.StatusCreated, term)
}

func (h *APIHandler) DeleteAnimeSearchTerm(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	termID, err := strconv.Atoi(vars["term_id"])
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid term ID")
		return
	}

	if err := h.manager.DeleteAnimeSearchTerm(termID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete search term")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
