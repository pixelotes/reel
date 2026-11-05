package services

import (
	"os"
	"testing"

	"reel/internal/clients/indexers"
	"reel/internal/database/models"
)

func TestFilterByQuality_Ranges(t *testing.T) {
	results := []indexers.IndexerResult{
		{Title: "A 480p"}, {Title: "A 720p"}, {Title: "A 1080p"},
		{Title: "A 2160p"}, {Title: "A unknown"},
	}
	cases := []struct {
		name, min, max string
		want           int
	}{
		{"no range accepts everything incl. unknown", "", "", 5},
		{"no min", "", "1080p", 4},
		{"no max", "720p", "", 3},
		{"explicit 720p-1080p", "720p", "1080p", 2},
		{"explicit range rejects unknown", "360p", "2160p", 4},
	}
	ts := testSelector()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ts.filterByQuality(results, c.min, c.max, &FilterStats{})
			if len(got) != c.want {
				t.Errorf("min=%q max=%q: got %d results, want %d", c.min, c.max, len(got), c.want)
			}
		})
	}
}

func TestQualityRankBounds_Explicit(t *testing.T) {
	lo, hi := qualityRankBounds("720p", "1080p")
	if lo != 2 || hi != 3 {
		t.Errorf("got [%d,%d], want [2,3]", lo, hi)
	}
}

func TestSaveMetadata_SkipsNFOForEbooks(t *testing.T) {
	for _, tc := range []struct {
		typ     models.MediaType
		wantNFO string
	}{
		{models.MediaTypeEbook, ""},
		{models.MediaTypeMovie, "movie.nfo"},
	} {
		dest := t.TempDir()
		ctx := testContext(t, tc.typ)
		ctx.Media.PosterURL = nil
		ctx.DestinationDir = dest
		ctx.SeasonNumber = 0
		if err := NewSaveMetadataStage(testConfig("", ""), testLogger()).Execute(ctx); err != nil {
			t.Fatal(err)
		}
		var found []string
		entries, _ := os.ReadDir(dest)
		for _, e := range entries {
			found = append(found, e.Name())
		}
		if tc.wantNFO == "" && len(found) != 0 {
			t.Errorf("%s: expected no files, got %v", tc.typ, found)
		}
		if tc.wantNFO != "" && (len(found) != 1 || found[0] != tc.wantNFO) {
			t.Errorf("%s: expected [%s], got %v", tc.typ, tc.wantNFO, found)
		}
	}
}
