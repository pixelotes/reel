package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type TraktClient struct {
	httpClient *http.Client
	clientID   string
	tmdbClient *TMDBClient // Add this
}

// Trakt search result structs
type traktSearchResult struct {
	Show traktShow `json:"show"`
}

type traktShow struct {
	Title    string                 `json:"title"`
	Year     int                    `json:"year"`
	Overview string                 `json:"overview"`
	IDs      map[string]interface{} `json:"ids"` // Correctly handle mixed types to prevent JSON error
}

// Trakt episode structs
type traktEpisode struct {
	Season     int    `json:"season"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	FirstAired string `json:"first_aired"`
}

func NewTraktClient(clientID string, tmdbClient *TMDBClient) *TraktClient {
	return &TraktClient{
		clientID:   clientID,
		tmdbClient: tmdbClient,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (t *TraktClient) sendRequest(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("trakt-api-version", "2")
	req.Header.Set("trakt-api-key", t.clientID)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("trakt API request failed with status: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func (t *TraktClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	searchURL := fmt.Sprintf("https://api.trakt.tv/search/show?query=%s&limit=5&extended=full", url.QueryEscape(title))

	var searchResults []traktSearchResult
	if err := t.sendRequest(searchURL, &searchResults); err != nil {
		return nil, fmt.Errorf("failed to search Trakt: %w", err)
	}

	var results []*TVShowResult
	for _, res := range searchResults {
		// Safely extract the trakt ID
		var traktID int
		if id, ok := res.Show.IDs["trakt"].(float64); ok {
			traktID = int(id)
		} else {
			continue // Skip if we can't get a valid Trakt ID
		}

		// Get episode list for the show
		episodesURL := fmt.Sprintf("https://api.trakt.tv/shows/%d/seasons?extended=episodes", traktID)
		var seasonsData []struct {
			Number   int            `json:"number"`
			Episodes []traktEpisode `json:"episodes"`
		}
		if err := t.sendRequest(episodesURL, &seasonsData); err != nil {
			fmt.Printf("Could not get episode data for %s: %v\n", res.Show.Title, err)
		}

		result := &TVShowResult{
			ID:        strconv.Itoa(traktID),
			Title:     res.Show.Title,
			Year:      res.Show.Year,
			Overview:  res.Show.Overview,
			PosterURL: "",
			Seasons:   make(map[int][]Episode),
		}

		for _, season := range seasonsData {
			if season.Number == 0 { // Skip specials
				continue
			}
			for _, ep := range season.Episodes {
				parsedTime, err := time.Parse(time.RFC3339, ep.FirstAired)
				airDate := ""
				if err == nil {
					airDate = parsedTime.Format("2006-01-02")
				}

				result.Seasons[season.Number] = append(result.Seasons[season.Number], Episode{
					EpisodeNumber: ep.Number,
					Title:         ep.Title,
					AirDate:       airDate,
				})
			}
		}
		results = append(results, result)
	}

	return results, nil
}

func (t *TraktClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	return nil, fmt.Errorf("Trakt movie search not implemented")
}

func (c *TraktClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	return nil, fmt.Errorf("GetTVShowDetailsByID not implemented for this client")
}
