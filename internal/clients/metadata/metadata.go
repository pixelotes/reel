package metadata

// Client is the interface for all metadata providers.
type Client interface {
	SearchMovie(title string, year int) (*MovieResult, error)
	// In the future, you could add:
	// SearchTVShow(title string, year int) (*TVShowResult, error)
}

// MovieResult is a standardized struct for movie metadata.
type MovieResult struct {
	ID        string // Use string to accommodate different providers (e.g., tt12345)
	Title     string
	Year      int
	Overview  string
	PosterURL string
	Rating    float64
}
