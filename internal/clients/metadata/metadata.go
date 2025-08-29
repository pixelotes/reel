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

type Episode struct {
	EpisodeNumber int    `json:"episode_number"`
	Title         string `json:"title"`
	AirDate       string `json:"air_date"`
}

// TVShowResult is a standardized struct for TV show metadata.
type TVShowResult struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Year      int               `json:"year"`
	Overview  string            `json:"overview"`
	PosterURL string            `json:"poster_url"`
	Rating    float64           `json:"rating"`
	Status    string            `json:"status"`
	Seasons   map[int][]Episode `json:"seasons"`
}
