package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const gutendexBaseURL = "https://gutendex.com"

type GutendexClient struct {
	baseURL    string
	httpClient *http.Client
}

type gutendexResponse struct {
	Count   int `json:"count"`
	Results []struct {
		ID      int    `json:"id"`
		Title   string `json:"title"`
		Authors []struct {
			Name      string `json:"name"`
			BirthYear *int   `json:"birth_year"`
			DeathYear *int   `json:"death_year"`
		} `json:"authors"`
		Subjects  []string          `json:"subjects"`
		Languages []string          `json:"languages"`
		Formats   map[string]string `json:"formats"`
	} `json:"results"`
}

func NewGutendexClient(timeout time.Duration) *GutendexClient {
	return &GutendexClient{
		baseURL:    gutendexBaseURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *GutendexClient) SearchBook(title string, author string) ([]*BookResult, error) {
	params := url.Values{}
	query := title
	if author != "" {
		query = title + " " + author
	}
	params.Set("search", query)

	reqURL := fmt.Sprintf("%s/books?%s", c.baseURL, params.Encode())

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("Gutendex search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gutendex API error: %s", resp.Status)
	}

	var gResp gutendexResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return nil, fmt.Errorf("failed to decode Gutendex response: %w", err)
	}

	var results []*BookResult
	limit := 5
	if len(gResp.Results) < limit {
		limit = len(gResp.Results)
	}

	for _, item := range gResp.Results[:limit] {
		var authors []string
		for _, a := range item.Authors {
			authors = append(authors, a.Name)
		}

		overview := ""
		if len(item.Subjects) > 0 {
			overview = fmt.Sprintf("Subjects: %s", item.Subjects[0])
			for _, s := range item.Subjects[1:] {
				overview += ", " + s
			}
		}

		results = append(results, &BookResult{
			ID:      fmt.Sprintf("gutenberg-%d", item.ID),
			Title:   item.Title,
			Authors: authors,
			Overview: overview,
		})
	}

	return results, nil
}

// Implement metadata.Client interface (not supported for books provider)
func (c *GutendexClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	return nil, fmt.Errorf("Gutendex does not support movie search")
}

func (c *GutendexClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	return nil, fmt.Errorf("Gutendex does not support TV show search")
}

func (c *GutendexClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	return nil, fmt.Errorf("Gutendex does not support TV show details")
}
