package indexers

import (
	"testing"
	"time"
)

func TestJackett_SearchMovies(t *testing.T) {
	srv := newFakeTorznabServer(t)
	defer srv.Close()

	client := NewJackettClient(srv.URL+"/torznab/test", "testkey", 5*time.Second)
	results, err := client.SearchMovies("Test Movie", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Title != "Test Movie 2024 1080p WEB-DL" {
		t.Errorf("first result title = %q", results[0].Title)
	}
	if results[0].Seeders != 42 {
		t.Errorf("first result seeders = %d, want 42", results[0].Seeders)
	}
	if results[0].Size != 1500000000 {
		t.Errorf("first result size = %d", results[0].Size)
	}
	if results[0].Indexer != "Jackett" {
		t.Errorf("indexer = %q, want 'Jackett'", results[0].Indexer)
	}
}

func TestJackett_SearchTVShows(t *testing.T) {
	srv := newFakeTorznabServer(t)
	defer srv.Close()

	client := NewJackettClient(srv.URL+"/torznab/test", "testkey", 5*time.Second)
	results, err := client.SearchTVShows("Test Show", 1, 5, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestJackett_HealthCheck_Success(t *testing.T) {
	srv := newFakeTorznabServer(t)
	defer srv.Close()

	client := NewJackettClient(srv.URL+"/torznab/test", "testkey", 5*time.Second)
	ok, err := client.HealthCheck()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("health check should pass")
	}
}

func TestJackett_ServerDown(t *testing.T) {
	client := NewJackettClient("http://127.0.0.1:1", "testkey", 1*time.Second)
	_, err := client.SearchMovies("Test", "", "")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestJackett_EmptyResults(t *testing.T) {
	srv := newFakeRSSServer(t)
	defer srv.Close()

	// Use the empty endpoint which returns valid XML with 0 items
	// But Jackett expects Torznab format, so create a specific empty torznab server
	emptySrv := newEmptyTorznabServer(t)
	defer emptySrv.Close()

	client := NewJackettClient(emptySrv.URL+"/torznab/test", "testkey", 5*time.Second)
	results, err := client.SearchMovies("Nonexistent", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
