package metadata

// Client is the interface for all metadata providers.
type Client interface {
	SearchMovie(title string, year int) ([]*MovieResult, error)
	SearchTVShow(title string) ([]*TVShowResult, error)
	GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error)
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

// BookResult is a standardized struct for book metadata.
type BookResult struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Authors   []string `json:"authors"`
	Year      int      `json:"year"`
	Overview  string   `json:"overview"`
	PosterURL string   `json:"poster_url"`
	Rating    float64  `json:"rating"`
	ISBN      string   `json:"isbn"`
}

// MangaResult is a standardized struct for manga metadata.
type MangaResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Year        int    `json:"year"`
	Description string `json:"description"`
	CoverURL    string `json:"cover_url"`
	Status      string `json:"status"` // ongoing, completed, hiatus, cancelled
}

// MangaChapter represents a single manga chapter.
type MangaChapter struct {
	ID        string `json:"id"`
	Chapter   string `json:"chapter"` // "1", "2.5", etc
	Volume    string `json:"volume"`
	Title     string `json:"title"`
	Language  string `json:"language"`
	Pages     int    `json:"pages"`
	CreatedAt string `json:"created_at"`
}

// MangaClient is the interface for manga metadata providers.
type MangaClient interface {
	SearchManga(title string) ([]*MangaResult, error)
	GetMangaChapters(mangaID string, languages []string) ([]MangaChapter, error)
	GetChapterPages(chapterID string) (baseURL string, filenames []string, err error)
}

// BookClient is the interface for book metadata providers.
type BookClient interface {
	SearchBook(title string, author string) ([]*BookResult, error)
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
