package metadata

import (
	"testing"
	"time"
)

func TestGutendex_SearchBook(t *testing.T) {
	srv := newFakeGutendexServer(t)
	defer srv.Close()

	client := NewGutendexClient(5 * time.Second)
	client.baseURL = srv.URL

	results, err := client.SearchBook("Sherlock Holmes", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("title = %q", results[0].Title)
	}
	if len(results[0].Authors) != 1 || results[0].Authors[0] != "Arthur Conan Doyle" {
		t.Errorf("authors = %v", results[0].Authors)
	}
	if results[0].Overview == "" {
		t.Error("expected subjects in overview")
	}
	// Gutendex has no covers or ratings
	if results[0].PosterURL != "" {
		t.Errorf("expected empty poster, got %q", results[0].PosterURL)
	}
}

func TestGutendex_ImplementsInterfaces(t *testing.T) {
	client := NewGutendexClient(5 * time.Second)

	var _ BookClient = client
	var _ Client = client

	_, err := client.SearchMovie("test", 2024)
	if err == nil {
		t.Error("SearchMovie should return error")
	}
}
