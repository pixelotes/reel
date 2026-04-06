package metadata

import (
	"testing"
	"time"
)

func TestGoogleBooks_SearchBook(t *testing.T) {
	srv := newFakeGoogleBooksServer(t)
	defer srv.Close()

	client := NewGoogleBooksClient("", 5*time.Second)
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
	if results[0].ISBN != "9780441013593" {
		t.Errorf("isbn = %q, want '9780441013593'", results[0].ISBN)
	}
	if results[0].Rating != 4.5 {
		t.Errorf("rating = %f, want 4.5", results[0].Rating)
	}
	if results[0].PosterURL == "" {
		t.Error("expected poster URL")
	}
	if results[0].Overview == "" {
		t.Error("expected overview")
	}
}

func TestGoogleBooks_SearchBookWithAuthor(t *testing.T) {
	srv := newFakeGoogleBooksServer(t)
	defer srv.Close()

	client := NewGoogleBooksClient("test-key", 5*time.Second)
	client.baseURL = srv.URL

	results, err := client.SearchBook("Dune", "Frank Herbert")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestGoogleBooks_ImplementsInterfaces(t *testing.T) {
	client := NewGoogleBooksClient("key", 5*time.Second)

	var _ BookClient = client
	var _ Client = client

	_, err := client.SearchMovie("test", 2024)
	if err == nil {
		t.Error("SearchMovie should return error")
	}
	_, err = client.SearchTVShow("test")
	if err == nil {
		t.Error("SearchTVShow should return error")
	}
}
