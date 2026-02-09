package subtitles

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
	"reel/internal/version"
)

const (
	BaseURL     = "https://api.opensubtitles.com/api/v1"
	DownloadURL = "https://api.opensubtitles.com/api/v1/download"
)

type Client struct {
	apiKey      string
	languages   []string
	logger      *utils.Logger
	config      *config.Config
	client      *http.Client
	rateLimiter *RateLimiter
}

type Subtitle struct {
	ID       string
	Language string
	Format   string
	Download string // URL or ID to download
	FileName string
	Score    float64
}

// OSResponse is a simplified structure for OpenSubtitles API response
type OSResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Language string `json:"language"`
			Format   string `json:"format"`
			Files    []struct {
				FileID   int    `json:"file_id"`
				FileName string `json:"file_name"`
			} `json:"files"`
			Ratings float64 `json:"ratings"` // Note: API might change, but this is approx
		} `json:"attributes"`
	} `json:"data"`
}

type DownloadResponse struct {
	Link string `json:"link"`
}

func NewClient(cfg *config.Config, logger *utils.Logger) *Client {
	langs := cfg.Subtitles.Languages
	if len(langs) == 0 {
		langs = []string{"en"}
	}

	// Rate limit: 5 requests per minute (conservative for free tier)
	// OpenSubtitles allows ~200 requests/day
	rateLimiter := NewRateLimiter(5, 1*time.Minute)

	return &Client{
		apiKey:      cfg.Subtitles.APIKey,
		languages:   langs,
		logger:      logger,
		config:      cfg,
		client:      &http.Client{Timeout: 15 * time.Second},
		rateLimiter: rateLimiter,
	}
}

func (c *Client) Search(filePath string, media *models.Media, season, episode int) ([]Subtitle, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OpenSubtitles API key not configured")
	}

	// 1. Try Hash Search
	hash, err := utils.ComputeMovieHash(filePath)
	if err == nil {
		c.logger.Info("Searching subtitles by hash:", hash)
		subs, err := c.searchByHash(hash)
		if err == nil && len(subs) > 0 {
			c.logger.Info("Found", len(subs), "subtitles by hash")
			return subs, nil
		}
	} else {
		c.logger.Warn("Failed to compute hash for subtitle search:", err)
	}

	// 2. Fallback to Text/Metadata Search
	c.logger.Info("Fallback: Searching subtitles by metadata for", media.Title)
	subs, err := c.searchByMetadata(media, season, episode)
	if err != nil {
		return nil, err
	}

	if len(subs) > 0 {
		c.logger.Info("Found", len(subs), "subtitles by metadata")
		return subs, nil
	}

	return nil, nil
}

func (c *Client) searchByHash(hash string) ([]Subtitle, error) {
	// API endpoint: /subtitles?moviehash={hash}&languages={langs}
	params := url.Values{}
	params.Add("moviehash", hash)
	params.Add("languages", strings.Join(c.languages, ","))

	return c.performRequest(params)
}

func (c *Client) searchByMetadata(media *models.Media, season, episode int) ([]Subtitle, error) {
	params := url.Values{}
	params.Add("languages", strings.Join(c.languages, ","))

	if media.Type == models.MediaTypeMovie {
		params.Add("query", media.Title)
		if media.Year > 0 {
			params.Add("year", strconv.Itoa(media.Year))
		}
		if media.TMDBId != nil {
			params.Add("tmdb_id", strconv.Itoa(*media.TMDBId))
		}
	} else { // TV Show / Anime
		params.Add("query", media.Title)
		params.Add("season_number", strconv.Itoa(season))
		params.Add("episode_number", strconv.Itoa(episode))
		// Can add tmdb_id if needed, but risky if TMDB ID is for show and not episode?
		// Actually tmdb_id parameter usually refers to the movie or show, then season/ep narrows it.
		if media.TMDBId != nil {
			params.Add("tmdb_id", strconv.Itoa(*media.TMDBId))
		}
	}

	return c.performRequest(params)
}

func (c *Client) performRequest(params url.Values) ([]Subtitle, error) {
	// Rate limit requests
	c.rateLimiter.Wait()

	reqURL := fmt.Sprintf("%s/subtitles?%s", BaseURL, params.Encode())
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	c.addHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenSubtitles API error: %s - Body: %s", resp.Status, string(body))
	}

	var osResp OSResponse
	if err := json.NewDecoder(resp.Body).Decode(&osResp); err != nil {
		return nil, err
	}

	var results []Subtitle
	for _, item := range osResp.Data {
		fileID := 0
		fileName := ""
		if len(item.Attributes.Files) > 0 {
			fileID = item.Attributes.Files[0].FileID
			fileName = item.Attributes.Files[0].FileName
		}

		results = append(results, Subtitle{
			ID:       item.ID,
			Language: item.Attributes.Language,
			Format:   item.Attributes.Format,
			Download: strconv.Itoa(fileID), // We need File ID to request download
			FileName: fileName,
			Score:    item.Attributes.Ratings,
		})
	}
	return results, nil
}

// Download requests the download link and saves the file with retry logic.
func (c *Client) Download(sub Subtitle, destPath string) error {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2^attempt seconds
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			c.logger.Info(fmt.Sprintf("Retrying subtitle download (attempt %d/%d) after %v", attempt+1, maxRetries, backoff))
			time.Sleep(backoff)
		}

		err := c.downloadAttempt(sub, destPath)
		if err == nil {
			return nil // Success!
		}

		lastErr = err
		c.logger.Warn(fmt.Sprintf("Subtitle download attempt %d failed: %v", attempt+1, err))
	}

	return fmt.Errorf("failed to download subtitle after %d attempts: %w", maxRetries, lastErr)
}

// downloadAttempt performs a single download attempt
func (c *Client) downloadAttempt(sub Subtitle, destPath string) error {
	// Rate limit
	c.rateLimiter.Wait()

	// 1. Request Download Link
	payload := map[string]int{"file_id": 0}
	if id, err := strconv.Atoi(sub.Download); err == nil {
		payload["file_id"] = id
	} else {
		return fmt.Errorf("invalid file id for download: %s", sub.Download)
	}

	jsonBody, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", DownloadURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}
	c.addHeaders(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get download link: %s - %s", resp.Status, string(body))
	}

	var dlResp DownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&dlResp); err != nil {
		return err
	}

	// 2. Download the file from the link
	dlReq, err := http.NewRequest("GET", dlResp.Link, nil)
	if err != nil {
		return err
	}

	dlRespObj, err := c.client.Do(dlReq)
	if err != nil {
		return err
	}
	defer dlRespObj.Body.Close()

	if dlRespObj.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", dlRespObj.Status)
	}

	// 3. Save to file
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, dlRespObj.Body)
	return err
}

func (c *Client) addHeaders(req *http.Request) {
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// Use dynamic version from build
	req.Header.Set("User-Agent", version.GetUserAgent())
}
