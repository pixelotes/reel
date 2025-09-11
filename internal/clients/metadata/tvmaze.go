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

type tvmazeShowSearch struct {
	Score float64    `json:"score"`
	Show  tvmazeShow `json:"show"`
}

type tvmazeShow struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Premiered string `json:"premiered"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	Image     struct {
		Original string `json:"original"`
	} `json:"image"`
	Rating struct {
		Average float64 `json:"average"`
	} `json:"rating"`
	Embedded struct {
		Episodes []tvmazeEpisode `json:"episodes"`
	} `json:"_embedded"`
}

type tvmazeEpisode struct {
	ID      int    `json:"id"`
	Season  int    `json:"season"`
	Number  int    `json:"number"`
	Name    string `json:"name"`
	Airdate string `json:"airdate"`
}

func NewTVmazeClient(timeout time.Duration) *TVmazeClient {
	return &TVmazeClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (t *TVmazeClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	return nil, fmt.Errorf("TVmaze does not support movie searches")
}

func (t *TVmazeClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	searchURL := fmt.Sprintf("https://api.tvmaze.com/search/shows?q=%s", url.QueryEscape(title))

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
		return nil, fmt.Errorf("TVmaze search failed with status: %d", resp.StatusCode)
	}

	var searchData []tvmazeShowSearch
	if err := json.NewDecoder(resp.Body).Decode(&searchData); err != nil {
		return nil, fmt.Errorf("failed to decode TVmaze response: %w", err)
	}

	if len(searchData) == 0 {
		return nil, fmt.Errorf("no TV show results found on TVmaze for '%s'", title)
	}

	var results []*TVShowResult
	numResults := len(searchData)
	if numResults > 5 {
		numResults = 5
	}

	for i := 0; i < numResults; i++ {
		showID := searchData[i].Show.ID
		infoURL := fmt.Sprintf("https://api.tvmaze.com/shows/%d?embed=episodes", showID)

		req, err := http.NewRequest("GET", infoURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create TVmaze info request: %w", err)
		}

		resp, err := t.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get TVmaze show info: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var showData tvmazeShow
		if err := json.NewDecoder(resp.Body).Decode(&showData); err != nil {
			return nil, fmt.Errorf("failed to decode TVmaze show info response: %w", err)
		}

		showYear := 0
		if showData.Premiered != "" {
			if premiereTime, err := time.Parse("2006-01-02", showData.Premiered); err == nil {
				showYear = premiereTime.Year()
			}
		}

		posterURL := ""
		if showData.Image.Original != "" {
			posterURL = showData.Image.Original
		}

		result := &TVShowResult{
			ID:        fmt.Sprintf("%d", showData.ID),
			Title:     showData.Name,
			Year:      showYear,
			Overview:  showData.Summary,
			PosterURL: posterURL,
			Rating:    showData.Rating.Average,
			Status:    showData.Status,
			Seasons:   make(map[int][]Episode),
		}

		for _, ep := range showData.Embedded.Episodes {
			result.Seasons[ep.Season] = append(result.Seasons[ep.Season], Episode{
				EpisodeNumber: ep.Number,
				Title:         ep.Name,
				AirDate:       ep.Airdate,
			})
		}
		results = append(results, result)
	}

	return results, nil
}

func (c *TVmazeClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	return nil, fmt.Errorf("GetTVShowDetailsByID not implemented for this client")
}
