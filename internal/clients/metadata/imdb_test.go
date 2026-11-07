package metadata

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeIMDb serves IMDb-style suggestions under /suggestion/ and Cinemeta-style
// metadata under /meta/movie/. IDs listed in brokenMeta answer 500.
func fakeIMDb(t *testing.T, brokenMeta map[string]bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/suggestion/s/sintel.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"d":[
			{"id":"nm0000001","l":"Sintel Person","qid":null},
			{"id":"tt9999999","l":"Sintel: The Series","y":2015,"qid":"tvSeries"},
			{"id":"tt5555555","l":"Sintel Remake","y":2024,"qid":"movie","i":{"imageUrl":"https://img/remake.jpg"}},
			{"id":"tt1727587","l":"Sintel","y":2010,"qid":"short","i":{"imageUrl":"https://img/sintel.jpg"}}
		]}`)
	})
	mux.HandleFunc("/meta/movie/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/meta/movie/"), ".json")
		if brokenMeta[id] {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `{"meta":{"description":"plot of %s","poster":"https://cinemeta/%s.jpg","imdbRating":"7.6"}}`, id, id)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testIMDBClient(srv *httptest.Server) *IMDBClient {
	c := NewIMDBClient("", 5*time.Second, nil)
	c.suggestURL = srv.URL + "/suggestion"
	c.cinemetaURL = srv.URL
	return c
}

func TestIMDB_SearchMovie_KeepsMoviesAndRanksYear(t *testing.T) {
	c := testIMDBClient(fakeIMDb(t, nil))

	res, err := c.SearchMovie("Sintel", 2010)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 movie results (person and series dropped), got %d: %+v", len(res), res)
	}
	got := res[0]
	if got.ID != "tt1727587" || got.Title != "Sintel" || got.Year != 2010 {
		t.Fatalf("year 2010 should rank first, got %+v", got)
	}
	if got.Overview != "plot of tt1727587" || got.PosterURL != "https://cinemeta/tt1727587.jpg" || got.Rating != 7.6 {
		t.Errorf("Cinemeta enrichment missing: %+v", got)
	}
}

func TestIMDB_SearchMovie_KeepsSuggestionDataWhenCinemetaFails(t *testing.T) {
	c := testIMDBClient(fakeIMDb(t, map[string]bool{"tt1727587": true}))

	res, err := c.SearchMovie("sintel", 2010)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := res[0]
	if got.ID != "tt1727587" || got.PosterURL != "https://img/sintel.jpg" || got.Overview != "" {
		t.Errorf("want IMDb suggestion data only, got %+v", got)
	}
}

func TestIMDB_SearchMovie_NoMovies(t *testing.T) {
	c := testIMDBClient(fakeIMDb(t, nil))
	if _, err := c.SearchMovie("zzz", 0); err == nil {
		t.Fatal("expected an error for a query with no suggestions")
	}
	if _, err := c.SearchMovie("   ", 0); err == nil {
		t.Fatal("expected an error for an empty title")
	}
}

func TestIMDB_SeriesNotSupported(t *testing.T) {
	c := NewIMDBClient("", time.Second, nil)
	if _, err := c.SearchTVShow("Severance"); err == nil {
		t.Fatal("imdb provider should refuse TV searches")
	}
}
