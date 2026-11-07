package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"reel/internal/utils"
)

// IMDBClient finds movie metadata without an API key.
//
// IMDb has no public API, so this uses two unofficial but long-lived,
// keyless JSON endpoints:
//   - IMDb's own search suggestions (what imdb.com's search box calls) to find
//     titles: IMDb ID, title, year, kind and poster.
//   - Cinemeta, Stremio's metadata service, to fill in the plot, a better
//     poster and the IMDb rating for each IMDb ID.
//
// Neither is covered by a stability promise, so TMDB (with an API key) stays
// the preferred movie provider; this is the fallback that works out of the box.
// Results carry the IMDb ID ("tt...") in MovieResult.ID.
type IMDBClient struct {
	httpClient  *http.Client
	logger      *utils.Logger
	suggestURL  string
	cinemetaURL string
}

const (
	imdbSuggestURL     = "https://v2.sg.media-imdb.com/suggestion"
	cinemetaURL        = "https://v3-cinemeta.strem.io"
	imdbUserAgent      = "Mozilla/5.0 (compatible; Reel media automator)"
	imdbMaxResults     = 8
	imdbEnrichParallel = 4
)

// movieKinds are the IMDb suggestion "qid" values that are movies. Series,
// episodes, people (nm...) and games are dropped.
var movieKinds = map[string]bool{"movie": true, "short": true, "tvMovie": true, "video": true}

// NewIMDBClient creates the keyless IMDb client. apiKey is ignored; it is kept
// so existing configs with `metadata.imdb.api_key` still load.
func NewIMDBClient(apiKey string, timeout time.Duration, logger *utils.Logger) *IMDBClient {
	return &IMDBClient{
		httpClient:  &http.Client{Timeout: timeout},
		logger:      logger,
		suggestURL:  imdbSuggestURL,
		cinemetaURL: cinemetaURL,
	}
}

type imdbSuggestion struct {
	D []struct {
		ID    string `json:"id"`
		Title string `json:"l"`
		Year  int    `json:"y"`
		Kind  string `json:"qid"`
		Image struct {
			URL string `json:"imageUrl"`
		} `json:"i"`
	} `json:"d"`
}

type cinemetaMeta struct {
	Meta struct {
		Description string `json:"description"`
		Poster      string `json:"poster"`
		IMDbRating  string `json:"imdbRating"`
	} `json:"meta"`
}

func (c *IMDBClient) getJSON(rawURL string, v interface{}) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", imdbUserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// SearchMovie looks a title up in IMDb's suggestions, keeps movies, ranks the
// requested year first, and enriches each match with Cinemeta.
func (c *IMDBClient) SearchMovie(title string, year int) ([]*MovieResult, error) {
	query := strings.ToLower(strings.TrimSpace(title))
	first := firstAlnum(query)
	if first == 0 {
		return nil, fmt.Errorf("IMDb search needs a title with letters or digits")
	}

	var sug imdbSuggestion
	suggestURL := fmt.Sprintf("%s/%c/%s.json", c.suggestURL, first, url.PathEscape(query))
	if err := c.getJSON(suggestURL, &sug); err != nil {
		return nil, fmt.Errorf("IMDb search failed: %w", err)
	}

	var results []*MovieResult
	for _, d := range sug.D {
		if !strings.HasPrefix(d.ID, "tt") || !movieKinds[d.Kind] {
			continue
		}
		results = append(results, &MovieResult{
			ID:        d.ID,
			Title:     d.Title,
			Year:      d.Year,
			PosterURL: d.Image.URL,
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no results found on IMDb for '%s'", title)
	}

	// Exact year first (if one was asked for), otherwise keep IMDb's order,
	// which already ranks by relevance and popularity.
	if year > 0 {
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].Year == year && results[j].Year != year
		})
	}
	if len(results) > imdbMaxResults {
		results = results[:imdbMaxResults]
	}

	c.enrich(results)
	return results, nil
}

// enrich adds plot, poster and rating from Cinemeta. A failure only leaves
// that result with what IMDb's suggestions gave.
func (c *IMDBClient) enrich(results []*MovieResult) {
	sem := make(chan struct{}, imdbEnrichParallel)
	var wg sync.WaitGroup
	for _, r := range results {
		wg.Add(1)
		go func(r *MovieResult) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var meta cinemetaMeta
			if err := c.getJSON(fmt.Sprintf("%s/meta/movie/%s.json", c.cinemetaURL, r.ID), &meta); err != nil {
				if c.logger != nil {
					c.logger.Debug("Cinemeta lookup failed for", r.ID, err)
				}
				return
			}
			r.Overview = meta.Meta.Description
			if meta.Meta.Poster != "" {
				r.PosterURL = meta.Meta.Poster
			}
			if rating, err := strconv.ParseFloat(meta.Meta.IMDbRating, 64); err == nil {
				r.Rating = rating
			}
		}(r)
	}
	wg.Wait()
}

func firstAlnum(s string) rune {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
	}
	return 0
}

// SearchTVShow is not offered: TV metadata comes from TVmaze, AniList or TMDB.
func (c *IMDBClient) SearchTVShow(title string) ([]*TVShowResult, error) {
	return nil, fmt.Errorf("the imdb provider only supports movies; use tvmaze, anilist or tmdb for series")
}

func (c *IMDBClient) GetTVShowDetailsByID(tmdbID int) (*TVShowResult, error) {
	return nil, fmt.Errorf("the imdb provider only supports movies")
}
