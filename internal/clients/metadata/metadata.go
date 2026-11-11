// Package metadata holds Reel's ebook metadata providers. Movies, shows and
// anime come from scout (github.com/pixelotes/scout) instead.
package metadata

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

// BookClient is the interface for book metadata providers.
type BookClient interface {
	SearchBook(title string, author string) ([]*BookResult, error)
}
