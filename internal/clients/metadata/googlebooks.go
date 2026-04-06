package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const googleBooksBaseURL = "https://www.googleapis.com/books/v1"

type GoogleBooksClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type googleBooksResponse struct {
	TotalItems int `json:"totalItems"`
	Items      []struct {
		ID         string `json:"id"`
		VolumeInfo struct {
			Title               string   `json:"title"`
			Authors             []string `json:"authors"`
			PublishedDate       string   `json:"publishedDate"`
			Description         string   `json:"description"`
			IndustryIdentifiers []struct {
				Type       string `json:"type"`
				Identifier string `json:"identifier"`
			} `json:"industryIdentifiers"`
			PageCount     int     `json:"pageCount"`
			AverageRating float64 `json:"averageRating"`
			ImageLinks    struct {
				Thumbnail string `json:"thumbnail"`
			} `json:"imageLinks"`
		} `json:"volumeInfo"`
	} `json:"items"`
}

func NewGoogleBooksClient(apiKey string, timeout time.Duration) *GoogleBooksClient {
	return &GoogleBooksClient{
		apiKey:     apiKey,
		baseURL:    googleBooksBaseURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *GoogleBooksClient) SearchBook(title string, author string) ([]*BookResult, error) {
	q := title
	if author != "" {
		q = fmt.Sprintf("intitle:%s+inauthor:%s", url.QueryEscape(title), url.QueryEscape(author))
	}

	reqURL := fmt.Sprintf("%s/volumes?q=%s&maxResults=5", c.baseURL, url.QueryEscape(q))
	if c.apiKey != "" {
		reqURL += "&key=" + url.QueryEscape(c.apiKey)
	}

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("Google Books search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Books API error: %s", resp.Status)
	}

	var gbResp googleBooksResponse
	if err := json.NewDecoder(resp.Body).Decode(&gbResp); err != nil {
		return nil, fmt.Errorf("failed to decode Google Books response: %w", err)
	}

	var results []*BookResult
	for _, item := range gbResp.Items {
		vi := item.VolumeInfo

		year := 0
		if len(vi.PublishedDate) >= 4 {
			fmt.Sscanf(vi.PublishedDate[:4], "%d", &year)
		}

		isbn := ""
		for _, id := range vi.IndustryIdentifiers {
			if id.Type == "ISBN_13" {
				isbn = id.Identifier
				break
			}
			if id.Type == "ISBN_10" && isbn == "" {
				isbn = id.Identifier
			}
		}

		poster := vi.ImageLinks.Thumbnail
		if poster != "" {
			poster = strings.Replace(poster, "http://", "https://", 1)
		}

		results = append(results, &BookResult{
			ID:        item.ID,
			Title:     vi.Title,
			Authors:   vi.Authors,
			Year:      year,
			Overview:  vi.Description,
			PosterURL: poster,
			Rating:    vi.AverageRating,
			ISBN:      isbn,
		})
	}

	return results, nil
}

// Implement metadata.Client interface (not supported for books provider)
func (c *GoogleBooksClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	return nil, fmt.Errorf("Google Books does not support movie search")
}

func (c *GoogleBooksClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	return nil, fmt.Errorf("Google Books does not support TV show search")
}

func (c *GoogleBooksClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	return nil, fmt.Errorf("Google Books does not support TV show details")
}
