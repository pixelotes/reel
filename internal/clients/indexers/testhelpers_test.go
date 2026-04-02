package indexers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newFakeTorznabServer creates a deterministic Torznab server for testing.
// Returns items with known titles, seeders, and sizes.
func newFakeTorznabServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/torznab/", func(w http.ResponseWriter, r *http.Request) {
		tParam := r.URL.Query().Get("t")

		if tParam == "caps" {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<caps>
				<server version="1.1" title="Fake Indexer"/>
				<searching>
					<search available="yes" supportedParams="q"/>
					<tv-search available="yes" supportedParams="q,season,ep"/>
					<movie-search available="yes" supportedParams="q,imdbid,tmdbid"/>
				</searching>
			</caps>`)
			return
		}

		w.Header().Set("Content-Type", "application/xml")
		pubDate := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC1123Z)

		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">
<channel>
  <title>Fake Indexer</title>
  <item>
    <title>Test Movie 2024 1080p WEB-DL</title>
    <link>magnet:?xt=urn:btih:fakehash1&amp;dn=Test+Movie</link>
    <pubDate>%s</pubDate>
    <size>1500000000</size>
    <torznab:attr name="seeders" value="42"/>
    <torznab:attr name="leechers" value="5"/>
  </item>
  <item>
    <title>Test Movie 2024 720p BluRay</title>
    <link>magnet:?xt=urn:btih:fakehash2&amp;dn=Test+Movie+720p</link>
    <pubDate>%s</pubDate>
    <size>800000000</size>
    <torznab:attr name="seeders" value="15"/>
    <torznab:attr name="leechers" value="2"/>
  </item>
  <item>
    <title>Test Show S01E05 1080p HDTV</title>
    <link>magnet:?xt=urn:btih:fakehash3&amp;dn=Test+Show</link>
    <pubDate>%s</pubDate>
    <size>500000000</size>
    <torznab:attr name="seeders" value="30"/>
    <torznab:attr name="leechers" value="3"/>
  </item>
</channel>
</rss>`, pubDate, pubDate, pubDate)
	})
	return httptest.NewServer(mux)
}

// newFakeProwlarrServer creates a deterministic Prowlarr JSON API server.
func newFakeProwlarrServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		// Verify API key header
		if r.Header.Get("X-Api-Key") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		results := []prowlarrSearchResult{
			{
				Title:       "Test Movie 2024 1080p",
				Size:        1500000000,
				Seeders:     42,
				Leechers:    5,
				DownloadURL: "magnet:?xt=urn:btih:fakehash1",
				PublishDate: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
				Indexer:     "FakeIndexer",
			},
			{
				Title:       "Test Movie 2024 720p",
				Size:        800000000,
				Seeders:     15,
				Leechers:    2,
				DownloadURL: "magnet:?xt=urn:btih:fakehash2",
				PublishDate: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
				Indexer:     "FakeIndexer",
			},
		}
		json.NewEncoder(w).Encode(results)
	})

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})

	return httptest.NewServer(mux)
}

// newEmptyTorznabServer creates a Torznab server that returns 0 items.
func newEmptyTorznabServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed">
<channel><title>Empty</title></channel></rss>`)
	})
	return httptest.NewServer(mux)
}

// newFakeRSSServer creates a deterministic RSS feed server.
func newFakeRSSServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		pubDate := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC1123Z)
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <item>
    <title>The Flash S01E01 720p</title>
    <link>magnet:?xt=urn:btih:rsshash1</link>
    <pubDate>%s</pubDate>
  </item>
  <item>
    <title>Breaking Bad S05E16 1080p</title>
    <link>magnet:?xt=urn:btih:rsshash2</link>
    <pubDate>%s</pubDate>
  </item>
  <item>
    <title>The Flash S01E02 1080p</title>
    <link>magnet:?xt=urn:btih:rsshash3</link>
    <pubDate>%s</pubDate>
  </item>
</channel>
</rss>`, pubDate, pubDate, pubDate)
	})

	mux.HandleFunc("/empty", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`)
	})

	mux.HandleFunc("/malformed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `this is not xml`)
	})

	return httptest.NewServer(mux)
}
