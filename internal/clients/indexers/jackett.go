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

// JackettClient implements a real Jackett client.
type JackettClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewJackettClient(baseURL, apiKey string, timeout time.Duration) *JackettClient {
	return &JackettClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// searchTorznab is a generic function to handle Torznab searches.
func (c *JackettClient) searchTorznab(params url.Values) ([]IndexerResult, error) {
	searchURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	resp, err := c.httpClient.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search Jackett: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jackett search failed with status: %d", resp.StatusCode)
	}

	var torznabResp TorznabFeed
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&torznabResp); err != nil {
		return nil, fmt.Errorf("failed to decode Jackett Torznab response: %w", err)
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
			Indexer:     "Jackett",
		}
	}
	return results, nil
}

// SearchMovies performs a movie search on Jackett.
func (c *JackettClient) SearchMovies(query string, imdbID string, searchMode string) ([]IndexerResult, error) {
	params := url.Values{}
	params.Add("t", "movie")
	params.Add("q", query)
	params.Add("apikey", c.apiKey)
	if imdbID != "" {
		params.Add("imdbid", imdbID)
	}

	return c.searchTorznab(params)
}

// SearchTVShows performs a TV show search on Jackett.
func (c *JackettClient) SearchTVShows(query string, season int, episode int, searchMode string) ([]IndexerResult, error) {
	params := url.Values{}
	params.Add("t", "tvsearch")
	params.Add("q", query)
	params.Add("apikey", c.apiKey)
	if season > 0 {
		params.Add("season", strconv.Itoa(season))
	}
	if episode > 0 {
		params.Add("ep", strconv.Itoa(episode))
	}

	return c.searchTorznab(params)
}

// HealthCheck verifies the connection to Jackett.
func (c *JackettClient) HealthCheck() (bool, error) {
	params := url.Values{}
	params.Add("t", "caps")
	params.Add("apikey", c.apiKey)

	searchURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())
	resp, err := c.httpClient.Get(searchURL)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
