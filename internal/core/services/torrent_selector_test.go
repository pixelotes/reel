package services

import (
	"testing"

	"reel/internal/clients/indexers"
	"reel/internal/database/models"
)

func testSelector() *TorrentSelector {
	cfg := testConfig("", "")
	cfg.Automation.MinSeeders = 3
	cfg.Automation.RejectCommon = []string{
		`\bvostfr\b`,
		`\bhdcam\b`,
		`\bscreener\b`,
		`\bfrench\b`,
		`\brus\b`,
		`\bmd\b`,
		`\btc\b`,
		`\bdvdscr\b`,
	}
	cfg.App.FilterLogLevel = "none"
	matcher := NewMatcherService(cfg, testLogger())
	return NewTorrentSelector(cfg, matcher, testLogger())
}

// fakeTorznabResults simulates the kind of results faketorznab generates:
// - Some with wrong episode numbers
// - Some with rejection terms
// - Some correct and good quality
func fakeTorznabResults(seriesName string, season, episode int) []indexers.IndexerResult {
	return []indexers.IndexerResult{
		// Correct results - should PASS
		{Title: seriesName + " S01E05 1080p HDTV x264", Seeders: 50, Size: 1500000000},
		{Title: seriesName + " S01E05 720p BluRay x265 DTS", Seeders: 30, Size: 800000000},
		{Title: seriesName + " S01E05 2160p BluRay REMUX DTS-HD", Seeders: 10, Size: 5000000000},

		// Wrong episode - should be REJECTED by episode filter
		{Title: seriesName + " S01E06 1080p HDTV", Seeders: 80, Size: 1500000000},
		{Title: seriesName + " S01E07 720p HDTV", Seeders: 60, Size: 700000000},
		{Title: seriesName + " S01E08 1080p BluRay", Seeders: 40, Size: 1200000000},
		{Title: seriesName + " S01E03 2160p HDTV", Seeders: 20, Size: 3000000000},
		{Title: seriesName + " S01E10 1080p HDTV", Seeders: 15, Size: 1400000000},

		// Rejection terms - should be REJECTED by reject filter
		{Title: seriesName + " S01E05 1080p VOSTFR", Seeders: 90, Size: 1500000000},
		{Title: seriesName + " S01E05 720p HDCAM", Seeders: 70, Size: 800000000},
		{Title: seriesName + " S01E05 1080p SCREENER", Seeders: 55, Size: 1300000000},
		{Title: seriesName + " S01E05 720p FRENCH", Seeders: 45, Size: 900000000},
		{Title: seriesName + " S01E05 1080p RUS", Seeders: 35, Size: 1100000000},
		{Title: seriesName + " S01E05 720p DVDSCR", Seeders: 25, Size: 750000000},

		// Too few seeders - should be REJECTED by seeder filter
		{Title: seriesName + " S01E05 1080p HDTV x264", Seeders: 1, Size: 1500000000},
		{Title: seriesName + " S01E05 720p HDTV", Seeders: 0, Size: 700000000},

		// Wrong series name - should be REJECTED by name filter
		{Title: "Totally Different Show S01E05 1080p HDTV", Seeders: 50, Size: 1500000000},
		{Title: "Another Series S01E05 720p BluRay", Seeders: 40, Size: 800000000},
	}
}

func TestFilterAndScore_RejectsPatterns(t *testing.T) {
	ts := testSelector()
	media := testMedia(models.MediaTypeTVShow)
	media.Title = "Mr Robot"
	media.MinQuality = "360p"
	media.MaxQuality = "2160p"

	results := []indexers.IndexerResult{
		{Title: "Mr Robot S01E05 1080p x264", Seeders: 50},
		{Title: "Mr Robot S01E05 1080p VOSTFR", Seeders: 90},
		{Title: "Mr Robot S01E05 720p HDCAM", Seeders: 70},
		{Title: "Mr Robot S01E05 1080p SCREENER", Seeders: 55},
	}

	filtered := ts.FilterAndScoreTorrents(media, results, 1, 5, []string{"Mr Robot"})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 result after reject filter, got %d", len(filtered))
	}
	if filtered[0].Title != "Mr Robot S01E05 1080p x264" {
		t.Errorf("wrong torrent passed: %q", filtered[0].Title)
	}
}

func TestFilterAndScore_RejectsWrongEpisode(t *testing.T) {
	ts := testSelector()
	media := testMedia(models.MediaTypeTVShow)
	media.Title = "The Flash"
	media.MinQuality = "360p"
	media.MaxQuality = "2160p"

	results := []indexers.IndexerResult{
		{Title: "The Flash S02E03 1080p HDTV", Seeders: 50},
		{Title: "The Flash S02E04 1080p HDTV", Seeders: 80},
		{Title: "The Flash S02E05 720p HDTV", Seeders: 40},
	}

	filtered := ts.FilterAndScoreTorrents(media, results, 2, 3, []string{"The Flash"})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 result for S02E03, got %d", len(filtered))
	}
	if filtered[0].Title != "The Flash S02E03 1080p HDTV" {
		t.Errorf("wrong episode passed: %q", filtered[0].Title)
	}
}

func TestFilterAndScore_RejectsWrongSeriesName(t *testing.T) {
	ts := testSelector()
	media := testMedia(models.MediaTypeTVShow)
	media.Title = "The 100"
	media.MinQuality = "360p"
	media.MaxQuality = "2160p"

	results := []indexers.IndexerResult{
		{Title: "The 100 S01E05 1080p HDTV", Seeders: 50},
		{Title: "The 100 Humans S01E05 720p HDTV", Seeders: 40},   // passes: contains "100" (significant token)
		{Title: "Flashpoint S01E05 1080p HDTV", Seeders: 60},       // rejected: missing "100"
		{Title: "Totally Different S01E05 1080p HDTV", Seeders: 30}, // rejected: missing "100"
	}

	filtered := ts.FilterAndScoreTorrents(media, results, 1, 5, []string{"The 100"})

	if len(filtered) != 2 {
		t.Fatalf("expected 2 results (The 100 + The 100 Humans), got %d", len(filtered))
	}
	for _, f := range filtered {
		if f.Title == "Flashpoint S01E05 1080p HDTV" {
			t.Errorf("Flashpoint should have been rejected (missing significant query tokens)")
		}
	}
}

func TestFilterAndScore_RejectsLowSeeders(t *testing.T) {
	ts := testSelector()
	media := testMedia(models.MediaTypeMovie)
	media.MinQuality = "360p"
	media.MaxQuality = "2160p"

	results := []indexers.IndexerResult{
		{Title: "Test Movie 2024 1080p HDTV", Seeders: 50},
		{Title: "Test Movie 2024 720p BluRay", Seeders: 1},
		{Title: "Test Movie 2024 1080p HDTV", Seeders: 0},
	}

	filtered := ts.FilterAndScoreTorrents(media, results, 0, 0, nil)

	if len(filtered) != 1 {
		t.Fatalf("expected 1 result above min_seeders=3, got %d", len(filtered))
	}
}

func TestFilterAndScore_FiltersQualityRange(t *testing.T) {
	ts := testSelector()
	media := testMedia(models.MediaTypeMovie)
	media.MinQuality = "720p"
	media.MaxQuality = "1080p"

	results := []indexers.IndexerResult{
		{Title: "Test Movie 2024 2160p BluRay", Seeders: 50},
		{Title: "Test Movie 2024 1080p HDTV", Seeders: 40},
		{Title: "Test Movie 2024 720p HDTV", Seeders: 30},
		{Title: "Test Movie 2024 480p DVDRip", Seeders: 20},
	}

	filtered := ts.FilterAndScoreTorrents(media, results, 0, 0, nil)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 results in 720p-1080p range, got %d", len(filtered))
	}
}

func TestFilterAndScore_SortsByScore(t *testing.T) {
	ts := testSelector()
	media := testMedia(models.MediaTypeMovie)
	media.MinQuality = "360p"
	media.MaxQuality = "2160p"

	results := []indexers.IndexerResult{
		{Title: "Test Movie 2024 720p HDTV x264", Seeders: 80},
		{Title: "Test Movie 2024 1080p BluRay x265 DTS", Seeders: 10},
		{Title: "Test Movie 2024 2160p BluRay REMUX DTS-HD x265", Seeders: 5},
	}

	filtered := ts.FilterAndScoreTorrents(media, results, 0, 0, nil)

	if len(filtered) != 3 {
		t.Fatalf("expected 3 results, got %d", len(filtered))
	}

	// 2160p REMUX should win despite fewer seeders
	if filtered[0].Title != "Test Movie 2024 2160p BluRay REMUX DTS-HD x265" {
		t.Errorf("expected 2160p REMUX first (highest quality), got %q", filtered[0].Title)
	}
	// 1080p BluRay should be second
	if filtered[1].Title != "Test Movie 2024 1080p BluRay x265 DTS" {
		t.Errorf("expected 1080p BluRay second, got %q", filtered[1].Title)
	}
	// Scores should be in descending order
	for i := 1; i < len(filtered); i++ {
		if filtered[i].Score > filtered[i-1].Score {
			t.Errorf("results not sorted by score: [%d]=%d > [%d]=%d",
				i, filtered[i].Score, i-1, filtered[i-1].Score)
		}
	}
}

func TestSelectBestTorrent_FullPipeline(t *testing.T) {
	ts := testSelector()
	media := testMedia(models.MediaTypeTVShow)
	media.Title = "Mr Robot"
	media.MinQuality = "360p"
	media.MaxQuality = "2160p"

	results := fakeTorznabResults("Mr Robot", 1, 5)

	best := ts.SelectBestTorrent(media, results, 1, 5, []string{"Mr Robot"})

	if best == nil {
		t.Fatal("expected a best torrent, got nil")
	}

	// Verify best torrent is one of the 3 valid results (correct episode, not rejected, enough seeders)
	validTitles := map[string]bool{
		"Mr Robot S01E05 1080p HDTV x264":           true,
		"Mr Robot S01E05 720p BluRay x265 DTS":        true,
		"Mr Robot S01E05 2160p BluRay REMUX DTS-HD":   true,
	}
	if !validTitles[best.Title] {
		t.Errorf("best torrent should be a valid result, got %q (score: %d)", best.Title, best.Score)
	}
	// Score should be positive (quality + seeders)
	if best.Score <= 0 {
		t.Errorf("best torrent score should be positive, got %d", best.Score)
	}
}

func TestSelectBestTorrent_NoResults(t *testing.T) {
	ts := testSelector()
	media := testMedia(models.MediaTypeMovie)
	media.MinQuality = "360p"
	media.MaxQuality = "2160p"

	// All results will be rejected
	results := []indexers.IndexerResult{
		{Title: "Movie SCREENER", Seeders: 0},
		{Title: "Movie HDCAM", Seeders: 1},
	}

	best := ts.SelectBestTorrent(media, results, 0, 0, nil)

	if best != nil {
		t.Errorf("expected nil when all rejected, got %q", best.Title)
	}
}

func TestGetQualityScore(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		wantMin   int
		wantMax   int
	}{
		{"2160p REMUX", "Movie.2160p.BluRay.REMUX.DTS-HD.x265", 30, 40},
		{"1080p HDTV", "Show.S01E01.1080p.HDTV.x264.AAC", 10, 14},
		{"720p HDTV", "Show.720p.HDTV.x264", 9, 12},
		{"CAM", "Movie.CAM", 0, 2},
		{"Empty", "", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := getQualityScore(tt.title)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("getQualityScore(%q) = %d, want between %d-%d", tt.title, score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestGetResolutionRank(t *testing.T) {
	tests := []struct {
		title string
		want  int
	}{
		{"Movie 2160p BluRay", 5},
		{"Movie 4K UHD", 5},
		{"Movie 1080p HDTV", 3},
		{"Movie 720p HDTV", 2},
		{"Movie 480p DVDRip", 1},
		{"Movie No Resolution", -1},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := getResolutionRank(tt.title)
			if got != tt.want {
				t.Errorf("getResolutionRank(%q) = %d, want %d", tt.title, got, tt.want)
			}
		})
	}
}

func TestFilterByRejectPatterns_WordBoundary(t *testing.T) {
	ts := testSelector()

	results := []indexers.IndexerResult{
		{Title: "Movie Trust 1080p HDTV"},        // contains "rus" but not as word
		{Title: "Movie 1080p RUS DTS"},              // "RUS" as word -> reject
		{Title: "Movie Rustic 720p"},                // contains "rus" but not as word
		{Title: "Movie 1080p HDTV TC Rip"},        // "TC" as word -> reject
		{Title: "Movie Match 1080p"},                // contains "tc" in "Match" -> should NOT reject
	}

	stats := &FilterStats{}
	filtered := ts.filterByRejectPatterns(results, stats)

	if len(filtered) != 3 {
		t.Fatalf("expected 3 results (Trust, Rustic, Match pass), got %d", len(filtered))
		for _, f := range filtered {
			t.Logf("  passed: %s", f.Title)
		}
	}
}
