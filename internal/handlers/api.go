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

func NewAPIHandler(manager *core.Manager, logger *utils.Logger) *APIHandler {
	return &APIHandler{manager: manager, logger: logger}
}

// Login endpoint
func (h *APIHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// In a real implementation, validate password against config
	// For now, just generate a simple JWT token
	token := generateJWTToken(req.Password) // Implement JWT generation

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// Get all media
func (h *APIHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	media, err := h.manager.GetAllMedia()
	if err != nil {
		http.Error(w, "Failed to fetch media", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

// Add new media
func (h *APIHandler) AddMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type       string `json:"type"`
		Title      string `json:"title"`
		Year       int    `json:"year"`
		IMDBId     string `json:"imdb_id"`
		Language   string `json:"language"`
		MinQuality string `json:"min_quality"`
		MaxQuality string `json:"max_quality"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	mediaType := models.MediaType(req.Type)
	media, err := h.manager.AddMedia(mediaType, req.IMDBId, req.Title, req.Year,
		req.Language, req.MinQuality, req.MaxQuality)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

// Delete media
func (h *APIHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	if err := h.manager.DeleteMedia(id); err != nil {
		http.Error(w, "Failed to delete media", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Retry failed media
func (h *APIHandler) RetryMedia(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid media ID", http.StatusBadRequest)
		return
	}

	if err := h.manager.RetryMedia(id); err != nil {
		http.Error(w, "Failed to retry media", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Search metadata (TMDB/OMDB)
func (h *APIHandler) SearchMetadata(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	mediaType := r.URL.Query().Get("type")

	if query == "" {
		http.Error(w, "Query parameter required", http.StatusBadRequest)
		return
	}

	results, err := h.manager.SearchMetadata(query, mediaType)
	if err != nil {
		h.logger.Error("Metadata search failed:", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// System status
func (h *APIHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	status := h.manager.GetSystemStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Test connections
func (h *APIHandler) TestIndexer(w http.ResponseWriter, r *http.Request) {
	ok := h.manager.TestIndexerConnection()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": ok})
}

func (h *APIHandler) TestTorrent(w http.ResponseWriter, r *http.Request) {
	ok := h.manager.TestTorrentConnection()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": ok})
}

// Clear failed media
func (h *APIHandler) ClearFailed(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.ClearFailedMedia(); err != nil {
		http.Error(w, "Failed to clear failed media", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func generateJWTToken(password string) string {
	// Simple token generation - implement proper JWT in production
	return "simple-token-" + password
}
