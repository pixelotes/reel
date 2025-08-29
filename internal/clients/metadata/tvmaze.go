package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type TVmazeClient struct {
	httpClient *http.Client
}

type tvmazeSearchResponse struct {
	Score float64 `json:"score"`
	Show  struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Premiered string `json:"premiered"`
		Summary   string `json:"summary"`
		Image     struct {
			Medium   string `json:"medium"`
			Original string `json:"original"`
		} `json:"image"`
		Rating struct {
			Average float64 `json:"average"`
		} `json:"rating"`
	} `json:"show"`
}

func NewTVmazeClient() *TVmazeClient {
	return &TVmazeClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TVmazeClient) SearchMovie(title string, year int) (*MovieResult, error) {
	return nil, fmt.Errorf("TVmaze does not support movie searches")
}

func (t *TVmazeClient) SearchTVShow(title string) (*TVShowResult, error) {
	searchURL := fmt.Sprintf("https://api.tvmaze.com/singlesearch/shows?q=%s", url.QueryEscape(title))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create TVmaze request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search TVmaze: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// TVmaze returns 404 for not found, which is not a server error
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("no TV show results found on TVmaze for '%s'", title)
		}
		return nil, fmt.Errorf("TVmaze search failed with status: %d", resp.StatusCode)
	}

	var searchResp tvmazeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode TVmaze response: %w", err)
	}

	show := searchResp.Show
	showYear := 0
	if show.Premiered != "" {
		if premiereTime, err := time.Parse("2006-01-02", show.Premiered); err == nil {
			showYear = premiereTime.Year()
		}
	}

	posterURL := ""
	if show.Image.Original != "" {
		posterURL = show.Image.Original
	}

	return &TVShowResult{
		ID:        fmt.Sprintf("%d", show.ID),
		Title:     show.Name,
		Year:      showYear,
		Overview:  show.Summary,
		PosterURL: posterURL,
		Rating:    show.Rating.Average,
	}, nil
}
