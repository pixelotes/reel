package metadata

// Client is the interface for all metadata providers.
type Client interface {
	SearchMovie(title string, year int) (*MovieResult, error)
	SearchTVShow(title string) (*TVShowResult, error)
}

// MovieResult is a standardized struct for movie metadata.
type MovieResult struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Year      int     `json:"year"`
	Overview  string  `json:"overview"`
	PosterURL string  `json:"poster_url"`
	Rating    float64 `json:"rating"`
}

// TVShowResult is a standardized struct for TV show metadata.
type TVShowResult struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Year      int     `json:"year"`
	Overview  string  `json:"overview"`
	PosterURL string  `json:"poster_url"`
	Rating    float64 `json:"rating"`
}
