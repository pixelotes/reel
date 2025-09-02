package indexers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ProwlarrClient implements the indexer.Client interface for Prowlarr.
type ProwlarrClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// prowlarrSearchResult defines the structure of a single search result from the Prowlarr API.
type prowlarrSearchResult struct {
	Title       string    `json:"title"`
	Size        int64     `json:"size"`
	Seeders     int       `json:"seeders"`
	Leechers    int       `json:"leechers"`
	DownloadURL string    `json:"downloadUrl"`
	PublishDate time.Time `json:"publishDate"`
	Indexer     string    `json:"indexer"`
}

// NewProwlarrClient creates a new client for interacting with the Prowlarr API.
func NewProwlarrClient(baseURL, apiKey string) *ProwlarrClient {
	return &ProwlarrClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// search sends a request to the Prowlarr API and returns the results.
func (p *ProwlarrClient) search(params url.Values) ([]IndexerResult, error) {
	// Prowlarr's API endpoint is at the root of the URL provided.
	searchURL := fmt.Sprintf("%s/api/v1/search?%s", p.baseURL, params.Encode())

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Prowlarr request: %w", err)
	}
	req.Header.Set("X-Api-Key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search Prowlarr: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Prowlarr search failed with status: %d", resp.StatusCode)
	}

	var searchResults []prowlarrSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResults); err != nil {
		return nil, fmt.Errorf("failed to decode Prowlarr response: %w", err)
	}

	results := make([]IndexerResult, len(searchResults))
	for i, item := range searchResults {
		results[i] = IndexerResult{
			Title:       item.Title,
			Size:        item.Size,
			Seeders:     item.Seeders,
			Leechers:    item.Leechers,
			DownloadURL: item.DownloadURL,
			PublishDate: item.PublishDate,
			Indexer:     item.Indexer,
		}
	}
	return results, nil
}

// SearchMovies searches for movies using the Prowlarr API.
func (p *ProwlarrClient) SearchMovies(query string, tmdbID string, searchMode string) ([]IndexerResult, error) {
	params := url.Values{}
	params.Add("query", query)
	params.Add("type", "search")
	params.Add("categories", "2000") // Movie categories

	return p.search(params)
}

// SearchTVShows searches for TV shows using the Prowlarr API.
func (p *ProwlarrClient) SearchTVShows(query string, season int, episode int, searchMode string) ([]IndexerResult, error) {
	params := url.Values{}
	params.Add("query", query)
	params.Add("type", "search")
	params.Add("categories", "5000") // TV categories
	if season > 0 {
		params.Add("season", strconv.Itoa(season))
	}
	if episode > 0 {
		params.Add("episode", strconv.Itoa(episode))
	}

	return p.search(params)
}

// HealthCheck verifies the connection to the Prowlarr API.
func (p *ProwlarrClient) HealthCheck() (bool, error) {
	healthURL := fmt.Sprintf("%s/api/v1/health", p.baseURL)
	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Api-Key", p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
