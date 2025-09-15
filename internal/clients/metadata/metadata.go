package metadata

// Client is the interface for all metadata providers.
type Client interface {
	SearchMovie(title string, year int) ([]*MovieResult, error)
	SearchTVShow(title string) ([]*TVShowResult, error)
	GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error)
	GetMovieDetailsByID(tmdbID int) (*MovieResult, error)
}

// MovieResult is a standardized struct for movie metadata.
type MovieResult struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Year          int     `json:"year"`
	Overview      string  `json:"overview"`
	Tagline       string  `json:"tagline"`
	PosterURL     string  `json:"poster_url"`
	BackdropURL   string  `json:"backdrop_url"`
	LogoURL       string  `json:"logo_url"`
	Rating        float64 `json:"rating"`
	IMDBID        string  `json:"imdb_id"`
}

type Episode struct {
	EpisodeNumber int    `json:"episode_number"`
	Title         string `json:"title"`
	AirDate       string `json:"air_date"`
	Overview      string `json:"overview"`
}

// TVShowResult is a standardized struct for TV show metadata.
type TVShowResult struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Year        int               `json:"year"`
	Overview    string            `json:"overview"`
	PosterURL   string            `json:"poster_url"`
	BackdropURL string            `json:"backdrop_url"`
	LogoURL     string            `json:"logo_url"`
	Rating      float64           `json:"rating"`
	Status      string            `json:"status"`
	IMDBID      string            `json:"imdb_id"`
	Seasons     map[int][]Episode `json:"seasons"`
}
