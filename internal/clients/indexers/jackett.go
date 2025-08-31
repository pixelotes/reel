package indexers

import (
	"fmt"
	"time"
)

// JackettClient is a placeholder for a real Jackett client.
type JackettClient struct {
	url    string
	apiKey string
}

func NewJackettClient(url, apiKey string) *JackettClient {
	return &JackettClient{url: url, apiKey: apiKey}
}

// SearchMovies is a mock implementation for Jackett.
func (c *JackettClient) SearchMovies(query string, imdbID string, searchMode string) ([]IndexerResult, error) {
	fmt.Printf("Searching Jackett for: %s\n", query)
	// Return a single mock result for demonstration purposes.
	return []IndexerResult{
		{
			Title:       fmt.Sprintf("[MOCK] %s - 1080p", query),
			Size:        1234567890,
			Seeders:     100,
			Leechers:    10,
			DownloadURL: "magnet:?xt=urn:btih:mockjackettmagnet",
			PublishDate: time.Now(),
			Indexer:     "Jackett (Mock)",
		},
	}, nil
}

// SearchTVShows is a mock implementation for Jackett.
func (c *JackettClient) SearchTVShows(query string, season int, episode int, searchMode string) ([]IndexerResult, error) {
	fmt.Printf("Searching Jackett for TV Show: %s S%02dE%02d\n", query, season, episode)
	// Return a single mock result for demonstration purposes.
	return []IndexerResult{
		{
			Title:       fmt.Sprintf("[MOCK] %s S%02dE%02d - 1080p", query, season, episode),
			Size:        1234567890,
			Seeders:     100,
			Leechers:    10,
			DownloadURL: "magnet:?xt=urn:btih:mockjackettmagnet",
			PublishDate: time.Now(),
			Indexer:     "Jackett (Mock)",
		},
	}, nil
}

// HealthCheck is a mock implementation for Jackett.
func (c *JackettClient) HealthCheck() (bool, error) {
	// A real implementation would ping the Jackett API.
	fmt.Println("Performing mock Jackett health check.")
	return true, nil
}
