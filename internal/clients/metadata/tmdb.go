package metadata

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TMDBClient struct {
	apiKey     string
	language   string
	httpClient *http.Client
}

type tmdbTVDetails struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Overview   string `json:"overview"`
	PosterPath string `json:"poster_path"`
}

// Define a struct that matches the TMDB API's JSON response
type tmdbSearchResponse struct {
	Page    int `json:"page"`
	Results []struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		ReleaseDate string  `json:"release_date"`
		Overview    string  `json:"overview"`
		PosterPath  string  `json:"poster_path"`
		VoteAverage float64 `json:"vote_average"`
	} `json:"results"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
}

func NewTMDBClient(apiKey, language string) *TMDBClient {
	return &TMDBClient{
		apiKey:   apiKey,
		language: language,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TMDBClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	params := url.Values{}
	params.Add("api_key", t.apiKey)
	params.Add("language", t.language)
	params.Add("query", title)
	if year > 0 {
		params.Add("year", strconv.Itoa(year))
	}

	searchURL := fmt.Sprintf("https://api.themoviedb.org/3/search/movie?%s", params.Encode())

	// --- Start Logging ---
	maskedKey := t.apiKey
	if len(maskedKey) > 8 {
		maskedKey = maskedKey[:4] + "..." + maskedKey[len(maskedKey)-4:]
	}
	log.Printf("TMDB Request URL: %s", searchURL)
	log.Printf("TMDB API Key: %s", maskedKey)
	// --- End Logging ---

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create TMDB request: %w", err)
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search TMDB: %w", err)
	}
	defer resp.Body.Close()

	// --- Start Logging ---
	log.Printf("TMDB Response Status Code: %d", resp.StatusCode)
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read TMDB response body: %w", err)
	}
	log.Printf("TMDB Response Body: %s", string(bodyBytes))
	// --- End Logging ---

	// Re-create a reader for the JSON decoder since the original has been consumed
	resp.Body = ioutil.NopCloser(strings.NewReader(string(bodyBytes)))

	var searchResp tmdbSearchResponse

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode TMDB response: %w", err)
	}

	if len(searchResp.Results) == 0 {
		return nil, fmt.Errorf("no results found on TMDB for '%s'", title)
	}

	var results []*MovieResult
	for i, result := range searchResp.Results {
		if i >= 5 {
			break
		}
		movieYear := 0
		if result.ReleaseDate != "" {
			if releaseTime, err := time.Parse("2006-01-02", result.ReleaseDate); err == nil {
				movieYear = releaseTime.Year()
			}
		}

		posterURL := ""
		if result.PosterPath != "" {
			posterURL = "https://image.tmdb.org/t/p/w500" + result.PosterPath
		}

		results = append(results, &MovieResult{
			ID:        strconv.Itoa(result.ID),
			Title:     result.Title,
			Year:      movieYear,
			Overview:  result.Overview,
			PosterURL: posterURL,
			Rating:    result.VoteAverage,
		})
	}

	return results, nil
}

func (t *TMDBClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	detailsURL := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?api_key=%s&language=%s", tmdbID, t.apiKey, t.language)

	req, err := http.NewRequest("GET", detailsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create TMDB details request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get TMDB details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB details request failed with status: %d", resp.StatusCode)
	}

	var details tmdbTVDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("failed to decode TMDB details: %w", err)
	}

	posterURL := ""
	if details.PosterPath != "" {
		posterURL = "https://image.tmdb.org/t/p/w500" + details.PosterPath
	}

	return &TVShowResult{
		PosterURL: posterURL,
	}, nil
}

func (t *TMDBClient) SearchTVShow(title string) ([]*TVShowResult, error) { // Add this empty function
	return nil, fmt.Errorf("TMDB TV show search not implemented")
}
