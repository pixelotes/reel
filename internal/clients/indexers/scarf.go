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

type TorznabAttribute struct {
	XMLName xml.Name `xml:"attr"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:"value,attr"`
}

type TorznabItem struct {
	Title       string             `xml:"title"`
	Link        string             `xml:"link"`
	Comments    string             `xml:"comments"`
	PubDate     string             `xml:"pubDate"`
	Size        int64              `xml:"size"`
	Description string             `xml:"description"`
	GUID        string             `xml:"guid"`
	Attributes  []TorznabAttribute `xml:"attr"`
}

func (item *TorznabItem) GetIntAttr(name string) int {
	for _, attr := range item.Attributes {
		if attr.Name == name {
			val, _ := strconv.Atoi(attr.Value)
			return val
		}
	}
	return 0
}

type TorznabChannel struct {
	Title       string        `xml:"title"`
	Description string        `xml:"description"`
	Link        string        `xml:"link"`
	Language    string        `xml:"language"`
	WebMaster   string        `xml:"webMaster"`
	Items       []TorznabItem `xml:"item"`
}

type TorznabFeed struct {
	XMLName xml.Name       `xml:"rss"`
	Channel TorznabChannel `xml:"channel"`
}

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

func (s *ScarfClient) SearchMovies(query string, tmdbID string) ([]IndexerResult, error) {
	params := url.Values{}
	params.Add("t", "movie-search") // Use t=movie-search for movie-specific search
	params.Add("q", query)
	params.Add("apikey", s.apiKey)
	if tmdbID != "" {
		params.Add("tmdbid", tmdbID)
	}

	// It now uses the base URL directly as the endpoint
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

func (s *ScarfClient) HealthCheck() (bool, error) {
	// A proper health check would ping a status endpoint.
	// For now, we assume it's healthy if the config is present.
	return true, nil
}
