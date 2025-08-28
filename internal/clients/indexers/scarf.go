package indexers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type ScarfClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Scarf specific response structs
type scarfResult struct {
	Title       string    `json:"Title"`
	Size        int64     `json:"Size"`
	Seeders     int       `json:"Seeders"`
	Leechers    int       `json:"Leechers"`
	DownloadURL string    `json:"DownloadURL"`
	PublishDate time.Time `json:"PublishDate"`
	Indexer     string    `json:"Indexer"`
}
type scarfResponse struct {
	Results []scarfResult `json:"results"`
	Total   int           `json:"total"`
}
type healthCheckResponse struct {
	Status string `json:"status"`
}

func NewScarfClient(baseURL, apiKey string, timeout time.Duration) *ScarfClient {
	return &ScarfClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (s *ScarfClient) SearchMovies(query string, imdbID string) ([]IndexerResult, error) {
	params := url.Values{}
	params.Add("indexer", "all")
	params.Add("q", query)
	params.Add("apikey", s.apiKey)
	if imdbID != "" {
		params.Add("imdbid", imdbID)
	}

	searchURL := fmt.Sprintf("%s/api/v1/search?%s", s.baseURL, params.Encode())
	resp, err := s.httpClient.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search Scarf: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Scarf search failed with status: %d", resp.StatusCode)
	}

	var scarfResp scarfResponse
	if err := json.NewDecoder(resp.Body).Decode(&scarfResp); err != nil {
		return nil, fmt.Errorf("failed to decode Scarf response: %w", err)
	}

	// Convert Scarf-specific results to the generic IndexerResult
	results := make([]IndexerResult, len(scarfResp.Results))
	for i, r := range scarfResp.Results {
		results[i] = IndexerResult{
			Title:       r.Title,
			Size:        r.Size,
			Seeders:     r.Seeders,
			Leechers:    r.Leechers,
			DownloadURL: r.DownloadURL,
			PublishDate: r.PublishDate,
			Indexer:     r.Indexer,
		}
	}
	return results, nil
}

func (s *ScarfClient) HealthCheck() (bool, error) {
	healthURL := fmt.Sprintf("%s/api/health", s.baseURL)
	resp, err := s.httpClient.Get(healthURL)
	if err != nil {
		return false, fmt.Errorf("failed to reach Scarf health endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	var healthResp healthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return false, fmt.Errorf("failed to decode health check response: %w", err)
	}

	return healthResp.Status == "healthy", nil
}
