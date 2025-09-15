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
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	Images       struct {
		Logos []struct {
			FilePath string `json:"file_path"`
		} `json:"logos"`
	} `json:"images"`
	ExternalIDs struct {
		IMDBID string `json:"imdb_id"`
	} `json:"external_ids"`
	Seasons []struct {
		SeasonNumber int `json:"season_number"`
	} `json:"seasons"`
}

type tmdbMovieDetails struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	Overview      string `json:"overview"`
	Tagline       string `json:"tagline"`
	PosterPath    string `json:"poster_path"`
	BackdropPath  string `json:"backdrop_path"`
	IMDBID        string `json:"imdb_id"`
	ReleaseDate   string `json:"release_date"`
	Images        struct {
		Logos []struct {
			FilePath string `json:"file_path"`
		} `json:"logos"`
	} `json:"images"`
}

type tmdbSeasonDetails struct {
	Episodes []struct {
		EpisodeNumber int    `json:"episode_number"`
		Name          string `json:"name"`
		AirDate       string `json:"air_date"`
		Overview      string `json:"overview"`
	} `json:"episodes"`
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

func NewTMDBClient(apiKey, language string, timeout time.Duration) *TMDBClient {
	return &TMDBClient{
		apiKey:   apiKey,
		language: language,
		httpClient: &http.Client{
			Timeout: timeout,
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
	detailsURL := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?api_key=%s&language=%s&append_to_response=images,external_ids", tmdbID, t.apiKey, t.language)

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

	backdropURL := ""
	if details.BackdropPath != "" {
		backdropURL = "https://image.tmdb.org/t/p/original" + details.BackdropPath
	}

	logoURL := ""
	if len(details.Images.Logos) > 0 {
		logoURL = "https://image.tmdb.org/t/p/original" + details.Images.Logos[0].FilePath
	}

	showResult := &TVShowResult{
		ID:          strconv.Itoa(details.ID),
		Title:       details.Name,
		Overview:    details.Overview,
		PosterURL:   posterURL,
		BackdropURL: backdropURL,
		LogoURL:     logoURL,
		IMDBID:      details.ExternalIDs.IMDBID,
		Seasons:     make(map[int][]Episode),
	}

	// Fetch episodes for each season
	for _, season := range details.Seasons {
		seasonDetailsURL := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d?api_key=%s&language=%s", tmdbID, season.SeasonNumber, t.apiKey, t.language)
		var seasonDetails tmdbSeasonDetails
		// You would typically create a helper for this repeated request logic
		req, _ := http.NewRequest("GET", seasonDetailsURL, nil)
		resp, _ := t.httpClient.Do(req)
		json.NewDecoder(resp.Body).Decode(&seasonDetails)
		resp.Body.Close()

		for _, ep := range seasonDetails.Episodes {
			showResult.Seasons[season.SeasonNumber] = append(showResult.Seasons[season.SeasonNumber], Episode{
				EpisodeNumber: ep.EpisodeNumber,
				Title:         ep.Name,
				AirDate:       ep.AirDate,
				Overview:      ep.Overview,
			})
		}
	}

	return showResult, nil
}

func (t *TMDBClient) GetMovieDetailsByID(tmdbID int) (*MovieResult, error) {
	detailsURL := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&language=%s&append_to_response=images", tmdbID, t.apiKey, t.language)

	var details tmdbMovieDetails
	err := t.sendRequest(detailsURL, &details)
	if err != nil {
		return nil, err
	}

	year := 0
	if parsedTime, err := time.Parse("2006-01-02", details.ReleaseDate); err == nil {
		year = parsedTime.Year()
	}

	return &MovieResult{
		ID:            strconv.Itoa(details.ID),
		Title:         details.Title,
		OriginalTitle: details.OriginalTitle,
		Year:          year,
		Overview:      details.Overview,
		Tagline:       details.Tagline,
		PosterURL:     "https://image.tmdb.org/t/p/w500" + details.PosterPath,
		BackdropURL:   "https://image.tmdb.org/t/p/original" + details.BackdropPath,
		LogoURL:       "https://image.tmdb.org/t/p/original" + details.Images.Logos[0].FilePath,
		IMDBID:        details.IMDBID,
	}, nil
}

func (t *TMDBClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	return nil, fmt.Errorf("TMDB TV show search not implemented")
}

func (t *TMDBClient) sendRequest(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}
