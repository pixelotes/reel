package indexers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// GutendexIndexer implements indexers.Client for Project Gutenberg via the Gutendex API.
// Returns direct download URLs for public domain ebooks.
type GutendexIndexer struct {
	baseURL    string
	httpClient *http.Client
	language   string
}

type gutendexIndexerResponse struct {
	Count   int `json:"count"`
	Results []struct {
		ID      int    `json:"id"`
		Title   string `json:"title"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
		Formats       map[string]string `json:"formats"`
		DownloadCount int               `json:"download_count"`
	} `json:"results"`
}

// Preferred ebook formats in order of priority.
var ebookFormatPriority = []struct {
	mime string
	ext  string
}{
	{"application/epub+zip", ".epub"},
	{"application/x-mobipocket-ebook", ".mobi"},
	{"application/pdf", ".pdf"},
	{"text/html", ".html"},
}

func NewGutendexIndexer(timeout time.Duration, language string) *GutendexIndexer {
	return &GutendexIndexer{
		baseURL:    "https://gutendex.com",
		httpClient: &http.Client{Timeout: timeout},
		language:   language,
	}
}

func (g *GutendexIndexer) SearchMovies(query string, _ string, _ string) ([]IndexerResult, error) {
	return g.search(query)
}

func (g *GutendexIndexer) SearchTVShows(query string, _ int, _ int, _ string) ([]IndexerResult, error) {
	return g.search(query)
}

func (g *GutendexIndexer) search(query string) ([]IndexerResult, error) {
	params := url.Values{}
	params.Set("search", query)
	if g.language != "" {
		params.Set("languages", g.language)
	}
	reqURL := fmt.Sprintf("%s/books?%s", g.baseURL, params.Encode())

	resp, err := g.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("Gutendex search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gutendex API error: %s", resp.Status)
	}

	var gResp gutendexIndexerResponse
	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return nil, fmt.Errorf("failed to decode Gutendex response: %w", err)
	}

	var results []IndexerResult
	limit := 10
	if len(gResp.Results) < limit {
		limit = len(gResp.Results)
	}

	for _, item := range gResp.Results[:limit] {
		// Find the best available format
		downloadURL := ""
		for _, pref := range ebookFormatPriority {
			if u, ok := item.Formats[pref.mime]; ok {
				downloadURL = u
				break
			}
		}
		if downloadURL == "" {
			continue // No usable format
		}

		author := ""
		if len(item.Authors) > 0 {
			author = item.Authors[0].Name
		}

		title := item.Title
		if author != "" {
			title = fmt.Sprintf("%s - %s", author, item.Title)
		}

		results = append(results, IndexerResult{
			Title:       title,
			DownloadURL: downloadURL,
			Seeders:     item.DownloadCount, // Use download count as "seeders" for scoring
			Size:        0,                  // Unknown until downloaded
			Indexer:     "Gutendex",
			PublishDate: time.Time{},
		})
	}

	return results, nil
}

func (g *GutendexIndexer) HealthCheck() (bool, error) {
	resp, err := g.httpClient.Get(g.baseURL + "/books?search=test")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}
