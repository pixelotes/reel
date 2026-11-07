package services

import (
	"errors"
	"strings"
	"testing"

	"reel/internal/clients/metadata"
	"reel/internal/database/models"
)

type failingMetaClient struct{ err error }

func (f failingMetaClient) SearchMovie(string, int) ([]*metadata.MovieResult, error) {
	return nil, f.err
}
func (f failingMetaClient) SearchTVShow(string) ([]*metadata.TVShowResult, error) {
	return nil, f.err
}
func (f failingMetaClient) GetTVShowDetailsByID(int) (*metadata.TVShowResult, error) {
	return nil, f.err
}

type okMetaClient struct{}

func (okMetaClient) SearchMovie(title string, _ int) ([]*metadata.MovieResult, error) {
	return []*metadata.MovieResult{{Title: title}}, nil
}
func (okMetaClient) SearchTVShow(string) ([]*metadata.TVShowResult, error) { return nil, nil }
func (okMetaClient) GetTVShowDetailsByID(int) (*metadata.TVShowResult, error) {
	return nil, nil
}

func libraryWith(t *testing.T, clients ...metadata.Client) *LibraryService {
	return NewLibraryService(testConfig(t.TempDir(), t.TempDir()), nil,
		map[models.MediaType][]metadata.Client{models.MediaTypeMovie: clients}, nil, testLogger())
}

func TestSearchMetadata_ReportsEveryProviderFailure(t *testing.T) {
	l := libraryWith(t,
		failingMetaClient{errors.New("401 invalid API key")},
		failingMetaClient{errors.New("timeout")},
	)
	_, err := l.SearchMetadata("Sintel", string(models.MediaTypeMovie))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"401 invalid API key", "timeout"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestSearchMetadata_FallsBackToNextProvider(t *testing.T) {
	l := libraryWith(t, failingMetaClient{errors.New("down")}, okMetaClient{})
	res, err := l.SearchMetadata("Sintel", string(models.MediaTypeMovie))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result from the second provider, got %d", len(res))
	}
}

func TestSearchMetadata_NoProviderConfigured(t *testing.T) {
	l := libraryWith(t)
	if _, err := l.SearchMetadata("Sintel", string(models.MediaTypeMovie)); err == nil ||
		!strings.Contains(err.Error(), "no metadata provider configured") {
		t.Fatalf("want a 'no metadata provider configured' error, got %v", err)
	}
}
