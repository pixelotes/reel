package indexers

import "time"

// Client is the interface for all indexer providers.
type Client interface {
	SearchMovies(query string, tmdbID string) ([]IndexerResult, error)
	HealthCheck() (bool, error)
}

// IndexerResult is a standardized struct for search results from any indexer.
type IndexerResult struct {
	Title       string
	Size        int64
	Seeders     int
	Leechers    int
	DownloadURL string
	PublishDate time.Time
	Indexer     string
	Score       int
}
