package services

import (
	"errors"
	"testing"
	"time"

	"reel/internal/clients/indexers"
	"reel/internal/database/models"
)

// fakeIndexer answers after delay with one result titled name, or with err.
type fakeIndexer struct {
	name  string
	delay time.Duration
	err   error
}

func (f fakeIndexer) answer() ([]indexers.IndexerResult, error) {
	time.Sleep(f.delay)
	if f.err != nil {
		return nil, f.err
	}
	return []indexers.IndexerResult{{Title: f.name, DownloadURL: "magnet:?xt=urn:btih:" + f.name}}, nil
}

func (f fakeIndexer) SearchMovies(string, string, string) ([]indexers.IndexerResult, error) {
	return f.answer()
}

func (f fakeIndexer) SearchTVShows(string, int, int, string) ([]indexers.IndexerResult, error) {
	return f.answer()
}

func (f fakeIndexer) HealthCheck() (bool, error) { return true, nil }

func searcherWith(budget time.Duration, clients ...fakeIndexer) *SearcherService {
	var wrapped []IndexerClientWithMode
	for _, c := range clients {
		wrapped = append(wrapped, IndexerClientWithMode{Client: c})
	}
	return NewSearcherService(
		map[models.MediaType][]IndexerClientWithMode{models.MediaTypeMovie: wrapped},
		nil, nil, testLogger(), budget,
	)
}

func titles(results []indexers.IndexerResult) map[string]bool {
	out := map[string]bool{}
	for _, r := range results {
		out[r.Title] = true
	}
	return out
}

func TestPerformSearch_ReturnsPartialResultsWhenBudgetRunsOut(t *testing.T) {
	s := searcherWith(300*time.Millisecond,
		fakeIndexer{name: "fast", delay: 10 * time.Millisecond},
		fakeIndexer{name: "slow", delay: 3 * time.Second},
	)
	media := &models.Media{Type: models.MediaTypeMovie, Title: "Sintel"}

	start := time.Now()
	results, err := s.performSearch(media, 0, 0)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := titles(results)
	if !got["fast"] || got["slow"] {
		t.Fatalf("want only the fast indexer's result, got %v", got)
	}
	if elapsed > time.Second {
		t.Fatalf("search should stop at the budget (300ms), took %s", elapsed)
	}
}

func TestPerformSearch_QueriesIndexersConcurrently(t *testing.T) {
	s := searcherWith(5*time.Second,
		fakeIndexer{name: "a", delay: 400 * time.Millisecond},
		fakeIndexer{name: "b", delay: 400 * time.Millisecond},
		fakeIndexer{name: "c", delay: 400 * time.Millisecond},
	)
	media := &models.Media{Type: models.MediaTypeMovie, Title: "Sintel"}

	start := time.Now()
	results, _ := s.performSearch(media, 0, 0)
	elapsed := time.Since(start)

	if len(results) != 3 {
		t.Fatalf("want 3 results, got %d", len(results))
	}
	// Sequentially this would take 1.2s, plus the old 5s pause after the term.
	if elapsed > 900*time.Millisecond {
		t.Fatalf("indexers should run in parallel with no trailing pause, took %s", elapsed)
	}
}

func TestPerformSearch_FailingIndexerDoesNotHideOthers(t *testing.T) {
	s := searcherWith(2*time.Second,
		fakeIndexer{name: "broken", err: errors.New("503 Service Unavailable")},
		fakeIndexer{name: "ok", delay: 20 * time.Millisecond},
	)
	media := &models.Media{Type: models.MediaTypeMovie, Title: "Sintel"}

	results, err := s.performSearch(media, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := titles(results); !got["ok"] || len(got) != 1 {
		t.Fatalf("want only the healthy indexer's result, got %v", got)
	}
}
