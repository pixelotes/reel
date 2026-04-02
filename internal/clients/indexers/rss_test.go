package indexers

import (
	"testing"
	"time"
)

func TestRSS_FetchAndFilter(t *testing.T) {
	srv := newFakeRSSServer(t)
	defer srv.Close()

	client := NewRSSClient(5 * time.Second)
	results, err := client.SearchMovies("The Flash", "", srv.URL+"/feed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should match "The Flash S01E01" and "The Flash S01E02"
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'The Flash', got %d", len(results))
	}
}

func TestRSS_CaseInsensitiveFilter(t *testing.T) {
	srv := newFakeRSSServer(t)
	defer srv.Close()

	client := NewRSSClient(5 * time.Second)
	results, err := client.SearchMovies("the flash", "", srv.URL+"/feed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results (case insensitive), got %d", len(results))
	}
}

func TestRSS_EmptyFeed(t *testing.T) {
	srv := newFakeRSSServer(t)
	defer srv.Close()

	client := NewRSSClient(5 * time.Second)
	results, err := client.SearchMovies("Nonexistent", "", srv.URL+"/empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRSS_HealthCheck_AlwaysTrue(t *testing.T) {
	client := NewRSSClient(5 * time.Second)
	ok, err := client.HealthCheck()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("RSS health check should always return true")
	}
}
