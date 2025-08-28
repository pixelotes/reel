package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type TMDBClient struct {
	apiKey     string
	language   string
	httpClient *http.Client
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

func (t *TMDBClient) SearchMovie(title string, year int) (*MovieResult, error) {
	params := url.Values{}
	params.Add("api_key", t.apiKey)
	params.Add("language", t.language)
	params.Add("query", title)
	if year > 0 {
		params.Add("year", strconv.Itoa(year))
	}

	searchURL := fmt.Sprintf("https://api.themoviedb.org/3/search/movie?%s", params.Encode())

	resp, err := t.httpClient.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search TMDB: %w", err)
	}
	defer resp.Body.Close()

	var searchResp struct {
		Results []struct {
			ID          int     `json:"id"`
			Title       string  `json:"title"`
			ReleaseDate string  `json:"release_date"`
			Overview    string  `json:"overview"`
			PosterPath  string  `json:"poster_path"`
			VoteAverage float64 `json:"vote_average"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode TMDB response: %w", err)
	}

	if len(searchResp.Results) == 0 {
		return nil, fmt.Errorf("no results found on TMDB for '%s'", title)
	}

	result := searchResp.Results[0]
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

	return &MovieResult{
		ID:        strconv.Itoa(result.ID), // Convert int ID to string
		Title:     result.Title,
		Year:      movieYear,
		Overview:  result.Overview,
		PosterURL: posterURL,
		Rating:    result.VoteAverage,
	}, nil
}
