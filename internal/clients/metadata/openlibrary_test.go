package metadata

import (
	"strings"
	"testing"
	"time"
)

func TestOpenLibrary_SearchBook(t *testing.T) {
	srv := newFakeOpenLibraryServer(t)
	defer srv.Close()

	client := NewOpenLibraryClient(5 * time.Second)
	client.baseURL = srv.URL

	results, err := client.SearchBook("Dune", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Title != "Dune" {
		t.Errorf("title = %q, want 'Dune'", results[0].Title)
	}
	if results[0].Year != 1965 {
		t.Errorf("year = %d, want 1965", results[0].Year)
	}
	if len(results[0].Authors) != 1 || results[0].Authors[0] != "Frank Herbert" {
		t.Errorf("authors = %v, want [Frank Herbert]", results[0].Authors)
	}

	// First result has cover
	if !strings.Contains(results[0].PosterURL, "covers.openlibrary.org") {
		t.Errorf("expected cover URL, got %q", results[0].PosterURL)
	}

	// Second result has no cover
	if results[1].PosterURL != "" {
		t.Errorf("expected empty poster for result without cover_i, got %q", results[1].PosterURL)
	}
}

func TestOpenLibrary_ImplementsInterfaces(t *testing.T) {
	client := NewOpenLibraryClient(5 * time.Second)

	var _ BookClient = client
	var _ Client = client

	_, err := client.SearchMovie("test", 2024)
	if err == nil {
		t.Error("SearchMovie should return error")
	}
}
