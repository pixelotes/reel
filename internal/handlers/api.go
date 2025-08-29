package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"reel/internal/core"
	"reel/internal/database/models"
	"reel/internal/utils"

	"github.com/gorilla/mux"
)

type APIHandler struct {
	manager *core.Manager
	logger  *utils.Logger
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

func NewAPIHandler(manager *core.Manager, logger *utils.Logger) *APIHandler {
	return &APIHandler{manager: manager, logger: logger}
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

	// In a real implementation, validate password against config
	// For now, just generate a simple JWT token
	token := generateJWTToken(req.Password) // Implement JWT generation

	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

// Get all media
func (h *APIHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	media, err := h.manager.GetAllMedia()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch media")
		return
	}

	respondJSON(w, http.StatusOK, media)
}

// Add new media
func (h *APIHandler) AddMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type         string `json:"type"`
		Title        string `json:"title"`
		Year         int    `json:"year"`
		TMDBId       int    `json:"tmdb_id"`
		Language     string `json:"language"`
		MinQuality   string `json:"min_quality"`
		MaxQuality   string `json:"max_quality"`
		AutoDownload bool   `json:"auto_download"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	mediaType := models.MediaType(req.Type)
	media, err := h.manager.AddMedia(mediaType, req.TMDBId, req.Title, req.Year,
		req.Language, req.MinQuality, req.MaxQuality, req.AutoDownload)
	if err != nil {
		h.logger.Error("Failed to add media:", err)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

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
	status := h.manager.GetSystemStatus()
	respondJSON(w, http.StatusOK, status)
}

// Test connections
func (h *APIHandler) TestIndexer(w http.ResponseWriter, r *http.Request) {
	ok := h.manager.TestIndexerConnection()
	respondJSON(w, http.StatusOK, map[string]bool{"ok": ok})
}

func (h *APIHandler) TestTorrent(w http.ResponseWriter, r *http.Request) {
	ok := h.manager.TestTorrentConnection()
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

func generateJWTToken(password string) string {
	// Simple token generation - implement proper JWT in production
	return "simple-token-" + password
}
