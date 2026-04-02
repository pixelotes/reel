package indexers

import (
	"testing"
	"time"
)

func TestScarf_SearchMovies(t *testing.T) {
	srv := newFakeTorznabServer(t)
	defer srv.Close()

	client := NewScarfClient(srv.URL+"/torznab/test", "testkey", 5*time.Second)
	results, err := client.SearchMovies("Test Movie", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Indexer != "Scarf" {
		t.Errorf("indexer = %q, want 'Scarf'", results[0].Indexer)
	}
	if results[0].Seeders != 42 {
		t.Errorf("seeders = %d, want 42", results[0].Seeders)
	}
}

func TestScarf_SearchTVShows(t *testing.T) {
	srv := newFakeTorznabServer(t)
	defer srv.Close()

	client := NewScarfClient(srv.URL+"/torznab/test", "testkey", 5*time.Second)
	results, err := client.SearchTVShows("Test Show", 1, 5, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestScarf_HealthCheck(t *testing.T) {
	srv := newFakeTorznabServer(t)
	defer srv.Close()

	// Scarf health check hits /health on the base host
	// Our test server doesn't have /health, so it should return false
	client := NewScarfClient(srv.URL+"/torznab/test", "testkey", 5*time.Second)
	ok, _ := client.HealthCheck()
	// 404 from test server = not healthy
	if ok {
		t.Error("health check should fail (no /health endpoint)")
	}
}

func TestScarf_ServerDown(t *testing.T) {
	client := NewScarfClient("http://127.0.0.1:1/torznab/test", "testkey", 1*time.Second)
	_, err := client.SearchMovies("Test", "", "")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
