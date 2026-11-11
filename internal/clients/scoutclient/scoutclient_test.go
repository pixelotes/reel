package scoutclient

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pixelotes/scout"

	"reel/internal/config"
	"reel/internal/database/models"
	"reel/internal/utils"
)

func TestKeyMissing(t *testing.T) {
	for key, want := range map[string]bool{
		"":                          true,
		"   ":                       true,
		"REPLACE_WITH_TMDB_API_KEY": true,
		"0123456789abcdef":          false,
	} {
		if got := KeyMissing(key); got != want {
			t.Errorf("KeyMissing(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestProvidersDropsBookProviders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Movies.Providers = []string{"tmdb", "gutendex", "imdb"}
	if got := Providers(cfg, models.MediaTypeMovie); !reflect.DeepEqual(got, []string{"tmdb", "imdb"}) {
		t.Fatalf("got %v", got)
	}
}

// subBackend records the subtitle query and serves fixed candidates.
type subBackend struct {
	scout.Backend
	got       scout.SubtitleQuery
	downloads []string
}

func (b *subBackend) SearchSubtitles(_ context.Context, q scout.SubtitleQuery) ([]scout.Subtitle, error) {
	b.got = q
	return []scout.Subtitle{
		{Provider: "opensubtitles", FileID: "1", Language: "es", Downloads: 5},
		{Provider: "opensubtitles", FileID: "2", Language: "es", HashMatch: true},
		{Provider: "opensubtitles", FileID: "3", Language: "en"},
	}, nil
}

func (b *subBackend) DownloadSubtitle(_ context.Context, _, id string) (*scout.SubtitleFile, error) {
	b.downloads = append(b.downloads, id)
	return &scout.SubtitleFile{FileName: id + ".srt", Data: []byte("sub " + id)}, nil
}

func TestFetchSubtitles(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Show - S01E02.mkv")
	if err := os.WriteFile(video, make([]byte, 200_000), 0o644); err != nil {
		t.Fatal(err)
	}
	// English is already there: only Spanish should be fetched.
	if err := os.WriteFile(filepath.Join(dir, "Show - S01E02.en.srt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tmdb := 1396
	media := &models.Media{Title: "Show", IMDBId: "tt0903747", TMDBId: &tmdb}
	b := &subBackend{}
	n, err := FetchSubtitles(context.Background(), b, media, video, 1, 2, []string{"en", "es"}, utils.NewLogger(false, io.Discard))
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if b.got.Hash == "" || b.got.Size != 200_000 || b.got.IMDbID != "tt0903747" || b.got.TMDbID != "1396" ||
		b.got.Season != 1 || b.got.Episode != 2 || !reflect.DeepEqual(b.got.Languages, []string{"es"}) {
		t.Fatalf("query %+v", b.got)
	}
	if !reflect.DeepEqual(b.downloads, []string{"2"}) {
		t.Fatalf("want the hash match downloaded, got %v", b.downloads)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "Show - S01E02.es.srt"))
	if string(data) != "sub 2" {
		t.Fatalf("es.srt = %q", data)
	}
}
