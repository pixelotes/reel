package indexers

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

// RSSItem mirrors the <item> structure in a standard RSS feed.
type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

// RSSChannel mirrors the <channel> structure.
type RSSChannel struct {
	Items []RSSItem `xml:"item"`
}

// RSSFeed is the top-level structure for the XML document.
type RSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel RSSChannel `xml:"channel"`
}

// RSSClient implements the indexer.Client for RSS feeds.
type RSSClient struct {
	httpClient *http.Client
}

func NewRSSClient(timeout time.Duration) *RSSClient {
	return &RSSClient{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// fetchFeed fetches and parses the content of a given RSS feed URL.
func (r *RSSClient) fetchFeed(url string) ([]IndexerResult, error) {
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RSS feed request failed with status: %d", resp.StatusCode)
	}

	var rssFeed RSSFeed
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&rssFeed); err != nil {
		return nil, fmt.Errorf("failed to decode RSS feed: %w", err)
	}

	results := make([]IndexerResult, len(rssFeed.Channel.Items))
	for i, item := range rssFeed.Channel.Items {
		pubDate, _ := time.Parse(time.RFC1123Z, item.PubDate)
		results[i] = IndexerResult{
			Title:       item.Title,
			DownloadURL: item.Link,
			PublishDate: pubDate,
			Indexer:     "RSS",
			// Seeders/Leechers are typically not available in basic RSS feeds
		}
	}
	return results, nil
}

// SearchMovies for RSS client filters the feed items by the query.
func (r *RSSClient) SearchMovies(query, tmdbID, url string) ([]IndexerResult, error) {
	// For RSS, we fetch the whole feed and then filter it. The 'query' is used as a filter.
	allItems, err := r.fetchFeed(url)
	if err != nil {
		return nil, err
	}

	var filteredResults []IndexerResult
	lowerQuery := strings.ToLower(query)
	for _, item := range allItems {
		if strings.Contains(strings.ToLower(item.Title), lowerQuery) {
			filteredResults = append(filteredResults, item)
		}
	}
	return filteredResults, nil
}

// SearchTVShows for RSS client filters the feed items by the query.
func (r *RSSClient) SearchTVShows(query string, season, episode int, url string) ([]IndexerResult, error) {
	return r.SearchMovies(query, "", url)
}

func (r *RSSClient) HealthCheck() (bool, error) {
	// A basic health check for RSS could try to fetch a known valid feed.
	// For now, we'll assume it's always healthy if it's configured.
	return true, nil
}
