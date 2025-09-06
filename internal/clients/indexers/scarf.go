package indexers

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/html/charset"
)

// --- Structs for Torznab XML Parsing ---
type ScarfClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewScarfClient(baseURL, apiKey string, timeout time.Duration) *ScarfClient {
	return &ScarfClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (s *ScarfClient) SearchMovies(query string, tmdbID string, searchMode string) ([]IndexerResult, error) {
	params := url.Values{}
	if searchMode == "" {
		searchMode = "movie-search"
	}
	params.Add("t", searchMode)
	params.Add("q", query)
	params.Add("apikey", s.apiKey)
	if tmdbID != "" {
		params.Add("tmdbid", tmdbID)
	}

	searchURL := fmt.Sprintf("%s?%s", s.baseURL, params.Encode())

	resp, err := s.httpClient.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search Scarf: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Scarf search failed with status: %d", resp.StatusCode)
	}

	var torznabResp TorznabFeed
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&torznabResp); err != nil {
		return nil, fmt.Errorf("failed to decode Scarf Torznab response: %w", err)
	}

	results := make([]IndexerResult, len(torznabResp.Channel.Items))
	for i, item := range torznabResp.Channel.Items {
		pubDate, _ := time.Parse(time.RFC1123Z, item.PubDate)
		results[i] = IndexerResult{
			Title:       item.Title,
			Size:        item.Size,
			Seeders:     item.GetIntAttr("seeders"),
			Leechers:    item.GetIntAttr("leechers"),
			DownloadURL: item.Link,
			PublishDate: pubDate,
			Indexer:     "Scarf",
		}
	}
	return results, nil
}

func (s *ScarfClient) SearchTVShows(query string, season int, episode int, searchMode string) ([]IndexerResult, error) {
	params := url.Values{}
	effectiveSearchMode := searchMode
	if effectiveSearchMode == "" {
		effectiveSearchMode = "tv-search"
	}
	params.Add("t", effectiveSearchMode)

	if effectiveSearchMode != "search" {
		params.Add("q", query)
		if season > 0 {
			params.Add("season", strconv.Itoa(season))
		}
		if episode > 0 {
			params.Add("ep", strconv.Itoa(episode))
		}
	} else {
		params.Add("q", query)
	}

	params.Add("apikey", s.apiKey)

	searchURL := fmt.Sprintf("%s?%s", s.baseURL, params.Encode())

	resp, err := s.httpClient.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search Scarf for TV shows: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Scarf TV show search failed with status: %d", resp.StatusCode)
	}

	var torznabResp TorznabFeed
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&torznabResp); err != nil {
		return nil, fmt.Errorf("failed to decode Scarf Torznab response: %w", err)
	}

	results := make([]IndexerResult, len(torznabResp.Channel.Items))
	for i, item := range torznabResp.Channel.Items {
		pubDate, _ := time.Parse(time.RFC1123Z, item.PubDate)
		results[i] = IndexerResult{
			Title:       item.Title,
			Size:        item.Size,
			Seeders:     item.GetIntAttr("seeders"),
			Leechers:    item.GetIntAttr("leechers"),
			DownloadURL: item.Link,
			PublishDate: pubDate,
			Indexer:     "Scarf",
		}
	}
	return results, nil
}

func (s *ScarfClient) HealthCheck() (bool, error) {
	// Parse the full Torznab URL to extract the base scheme and host.
	parsedURL, err := url.Parse(s.baseURL)
	if err != nil {
		return false, fmt.Errorf("could not parse scarf base url: %w", err)
	}

	// Construct the correct health check URL (e.g., http://localhost:8080/health).
	healthURL := fmt.Sprintf("%s://%s/health", parsedURL.Scheme, parsedURL.Host)

	resp, err := s.httpClient.Get(healthURL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
