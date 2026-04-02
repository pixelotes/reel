package indexers

import (
	"testing"
	"time"
)

func TestProwlarr_SearchMovies(t *testing.T) {
	srv := newFakeProwlarrServer(t)
	defer srv.Close()

	client := NewProwlarrClient(srv.URL, "test-api-key", 5*time.Second)
	results, err := client.SearchMovies("Test Movie", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Title != "Test Movie 2024 1080p" {
		t.Errorf("title = %q", results[0].Title)
	}
	if results[0].Seeders != 42 {
		t.Errorf("seeders = %d, want 42", results[0].Seeders)
	}
}

func TestProwlarr_SearchTVShows(t *testing.T) {
	srv := newFakeProwlarrServer(t)
	defer srv.Close()

	client := NewProwlarrClient(srv.URL, "test-api-key", 5*time.Second)
	results, err := client.SearchTVShows("Test Show", 1, 5, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestProwlarr_HealthCheck(t *testing.T) {
	srv := newFakeProwlarrServer(t)
	defer srv.Close()

	client := NewProwlarrClient(srv.URL, "test-api-key", 5*time.Second)
	ok, err := client.HealthCheck()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("health check should pass")
	}
}

func TestProwlarr_NoAPIKey_Fails(t *testing.T) {
	srv := newFakeProwlarrServer(t)
	defer srv.Close()

	client := NewProwlarrClient(srv.URL, "", 5*time.Second)
	_, err := client.SearchMovies("Test", "", "")
	if err == nil {
		t.Fatal("expected error without API key")
	}
}

func TestProwlarr_ServerDown(t *testing.T) {
	client := NewProwlarrClient("http://127.0.0.1:1", "key", 1*time.Second)
	_, err := client.SearchMovies("Test", "", "")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
