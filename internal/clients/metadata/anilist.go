package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type AniListClient struct {
	httpClient *http.Client
}

type aniListGraphQLQuery struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type aniListSearchResponse struct {
	Data struct {
		Page struct {
			Media []struct {
				ID    int `json:"id"`
				Title struct {
					English string `json:"english"`
					Romaji  string `json:"romaji"`
				} `json:"title"`
				Description string `json:"description"`
				BannerImage string `json:"bannerImage"`
				Episodes    int    `json:"episodes"`
				StartDate   struct {
					Year int `json:"year"`
				} `json:"startDate"`
			} `json:"media"`
		} `json:"page"`
	} `json:"data"`
}

func NewAniListClient(timeout time.Duration) *AniListClient {
	return &AniListClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (a *AniListClient) SearchAnime(title string) ([]*TVShowResult, error) {
	query := `
query ($search: String) {
  Page(perPage: 5) {
    media(search: $search, type: ANIME, sort: POPULARITY_DESC) {
      id
      title {
        romaji
        english
      }
      description(asHtml: false)
      bannerImage
      episodes
      startDate {
        year
      }
    }
  }
}
`
	variables := map[string]interface{}{
		"search": title,
	}

	jsonData, err := json.Marshal(aniListGraphQLQuery{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal graphQL query: %w", err)
	}

	req, err := http.NewRequest("POST", "https://graphql.anilist.co", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create anilist request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search anilist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anilist search failed with status: %d", resp.StatusCode)
	}

	var searchResp aniListSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode anilist response: %w", err)
	}

	if len(searchResp.Data.Page.Media) == 0 {
		return nil, fmt.Errorf("no anime results found for '%s'", title)
	}

	var results []*TVShowResult
	for _, anime := range searchResp.Data.Page.Media {
		animeTitle := anime.Title.English
		if animeTitle == "" {
			animeTitle = anime.Title.Romaji
		}

		result := &TVShowResult{
			ID:        strconv.Itoa(anime.ID),
			Title:     animeTitle,
			Year:      anime.StartDate.Year,
			Overview:  anime.Description,
			PosterURL: anime.BannerImage,
			Seasons:   make(map[int][]Episode),
		}

		for i := 1; i <= anime.Episodes; i++ {
			result.Seasons[1] = append(result.Seasons[1], Episode{
				EpisodeNumber: i,
				Title:         fmt.Sprintf("Episode %d", i),
			})
		}
		results = append(results, result)
	}

	return results, nil
}

func (a *AniListClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	return nil, fmt.Errorf("anilist client does not support movie searches")
}

func (a *AniListClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	return a.SearchAnime(title)
}

func (c *AniListClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	return nil, fmt.Errorf("GetTVShowDetailsByID not implemented for this client")
}
