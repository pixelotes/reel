package services

import (
	"context"
	"fmt"
	"testing"

	"github.com/pixelotes/scout"

	"reel/internal/database/models"
)

// fakeScout answers searches with one candidate and lookups with a two-season
// show, recording what was asked.
type fakeScout struct {
	scout.Backend
	queries []scout.Query
	lookups []scout.Lookup
}

func (f *fakeScout) Search(_ context.Context, q scout.Query) ([]scout.Item, error) {
	f.queries = append(f.queries, q)
	if q.Title == "nothing" {
		return nil, fmt.Errorf("no results: %w", scout.ErrNotFound)
	}
	return []scout.Item{{Kind: q.Kind, Provider: "tvmaze", ID: "44933", Title: "Severance", Year: 2022,
		IDs: scout.IDs{"tvmaze": "44933", "tmdb": "95396"}}}, nil
}

func (f *fakeScout) Get(_ context.Context, l scout.Lookup) (*scout.Item, error) {
	f.lookups = append(f.lookups, l)
	return &scout.Item{Kind: l.Kind, Provider: l.Provider, ID: l.ID, Title: "Severance", Year: 2022, Status: "Running",
		IDs: scout.IDs{"tvmaze": l.ID, "imdb": "tt11280740"},
		Seasons: map[int][]scout.Episode{
			1: {{Number: 1, Title: "Good News About Hell", AirDate: "2022-02-18"}, {Number: 2, AirDate: "2022-02-18"}},
			2: {{Number: 1, AirDate: "2099-01-01"}},
		}}, nil
}

func TestSearchMetadataPassesProviderOrder(t *testing.T) {
	cfg := testConfig(t.TempDir(), t.TempDir())
	cfg.TVShows.Providers = []string{"tvmaze", "tmdb"}
	f := &fakeScout{}
	l := NewLibraryService(cfg, nil, f, nil, testLogger())
	res, err := l.SearchMetadata("severance", string(models.MediaTypeTVShow))
	if err != nil || len(res) != 1 {
		t.Fatalf("res=%v err=%v", res, err)
	}
	q := f.queries[0]
	if q.Kind != scout.Show || fmt.Sprint(q.Providers) != "[tvmaze tmdb]" {
		t.Fatalf("query %+v", q)
	}
	if res, err := l.SearchMetadata("nothing", string(models.MediaTypeTVShow)); err != nil || len(res) != 0 {
		t.Fatalf("not found should be an empty list, got %v %v", res, err)
	}
}

func TestSearchMetadataNoProvider(t *testing.T) {
	l := NewLibraryService(testConfig(t.TempDir(), t.TempDir()), nil, &fakeScout{}, nil, testLogger())
	if _, err := l.SearchMetadata("x", string(models.MediaTypeMovie)); err == nil {
		t.Fatal("want an error with no movie providers configured")
	}
}

func TestAddShowUsesProviderAndStoresIt(t *testing.T) {
	repo := newTestRepo(t)
	f := &fakeScout{}
	l := NewLibraryService(testConfig(t.TempDir(), t.TempDir()), repo, f, nil, testLogger())

	media, err := l.AddMedia(models.MediaTypeTVShow, "tvmaze", "44933", "Severance", 2022, "en", "720p", "1080p", false, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.queries) != 0 || len(f.lookups) != 1 || f.lookups[0].ID != "44933" {
		t.Fatalf("with provider+id it should look up directly: queries=%v lookups=%v", f.queries, f.lookups)
	}
	if media.IMDBId != "tt11280740" {
		t.Fatalf("imdb id not stored: %+v", media)
	}
	show, err := repo.GetTVShowByMediaID(media.ID)
	if err != nil || show == nil {
		t.Fatal(err)
	}
	if show.Provider != "tvmaze" || show.TVmazeID != "44933" || len(show.Seasons) != 2 {
		t.Fatalf("show %+v", show)
	}
	st := map[string]models.MediaStatus{}
	for _, s := range show.Seasons {
		for _, e := range s.Episodes {
			st[fmt.Sprintf("%dx%d", s.SeasonNumber, e.EpisodeNumber)] = e.Status
		}
	}
	if st["1x1"] != models.StatusSkipped || st["1x2"] != models.StatusPending || st["2x1"] != models.StatusTBA {
		t.Fatalf("statuses %v", st)
	}

	// The episode refresh looks the show up by its stored provider and ID.
	l.updateShowMetadata(media)
	if len(f.queries) != 0 || f.lookups[1].Provider != "tvmaze" {
		t.Fatalf("refresh should use the stored ID: queries=%v lookups=%v", f.queries, f.lookups)
	}
}

func TestAddShowWithoutIDSearchesFirst(t *testing.T) {
	repo := newTestRepo(t)
	f := &fakeScout{}
	l := NewLibraryService(testConfig(t.TempDir(), t.TempDir()), repo, f, nil, testLogger())
	media, err := l.AddMedia(models.MediaTypeTVShow, "", "", "Severance", 0, "en", "720p", "1080p", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.queries) != 1 || f.lookups[0].ID != "44933" {
		t.Fatalf("queries=%v lookups=%v", f.queries, f.lookups)
	}
	// The TMDB ID only the search knew is kept.
	if media.TMDBId == nil || *media.TMDBId != 95396 || media.Year != 2022 {
		t.Fatalf("media %+v", media)
	}
}
