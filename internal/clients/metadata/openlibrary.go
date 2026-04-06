package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const openLibraryBaseURL = "https://openlibrary.org"

type OpenLibraryClient struct {
	baseURL    string
	httpClient *http.Client
}

type openLibraryResponse struct {
	NumFound int `json:"num_found"`
	Docs     []struct {
		Key              string   `json:"key"`
		Title            string   `json:"title"`
		AuthorName       []string `json:"author_name"`
		FirstPublishYear int      `json:"first_publish_year"`
		CoverI           int      `json:"cover_i"`
	} `json:"docs"`
}

func NewOpenLibraryClient(timeout time.Duration) *OpenLibraryClient {
	return &OpenLibraryClient{
		baseURL:    openLibraryBaseURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *OpenLibraryClient) SearchBook(title string, author string) ([]*BookResult, error) {
	params := url.Values{}
	if author != "" {
		params.Set("title", title)
		params.Set("author", author)
	} else {
		params.Set("q", title)
	}
	params.Set("limit", "5")
	params.Set("fields", "key,title,author_name,first_publish_year,cover_i")

	reqURL := fmt.Sprintf("%s/search.json?%s", c.baseURL, params.Encode())

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("Open Library search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Open Library API error: %s", resp.Status)
	}

	var olResp openLibraryResponse
	if err := json.NewDecoder(resp.Body).Decode(&olResp); err != nil {
		return nil, fmt.Errorf("failed to decode Open Library response: %w", err)
	}

	var results []*BookResult
	for _, doc := range olResp.Docs {
		posterURL := ""
		if doc.CoverI > 0 {
			posterURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", doc.CoverI)
		}

		results = append(results, &BookResult{
			ID:        doc.Key,
			Title:     doc.Title,
			Authors:   doc.AuthorName,
			Year:      doc.FirstPublishYear,
			PosterURL: posterURL,
		})
	}

	return results, nil
}

// Implement metadata.Client interface (not supported for books provider)
func (c *OpenLibraryClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	return nil, fmt.Errorf("Open Library does not support movie search")
}

func (c *OpenLibraryClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	return nil, fmt.Errorf("Open Library does not support TV show search")
}

func (c *OpenLibraryClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	return nil, fmt.Errorf("Open Library does not support TV show details")
}
