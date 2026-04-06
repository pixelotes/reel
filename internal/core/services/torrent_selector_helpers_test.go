package services

import (
	"testing"

	"reel/internal/clients/indexers"
	"reel/internal/config"
)

func TestBuildEpisodePatterns_StandardPatterns(t *testing.T) {
	standard, absolute, guard := buildEpisodePatterns(1, 5)

	if len(standard) != 6 {
		t.Errorf("expected 6 standard patterns, got %d", len(standard))
	}
	if len(absolute) != 4 {
		t.Errorf("expected 4 absolute patterns, got %d", len(absolute))
	}
	if guard == nil {
		t.Fatal("expected non-nil resolution guard")
	}

	// Standard patterns should match S01E05 variants
	matches := []string{
		"Show S01E05 1080p",
		"Show s1e5 720p",
		"Show 1x05 HDTV",
		"Show s01e05 x264",
	}
	for _, title := range matches {
		if !matchesAny(standard, title) {
			t.Errorf("standard patterns should match %q", title)
		}
	}

	// Should NOT match wrong episode
	nonMatches := []string{
		"Show S01E06 1080p",
		"Show S02E05 720p",
	}
	for _, title := range nonMatches {
		if matchesAny(standard, title) {
			t.Errorf("standard patterns should NOT match %q", title)
		}
	}
}

func TestBuildEpisodePatterns_AbsolutePatterns(t *testing.T) {
	_, absolute, guard := buildEpisodePatterns(1, 5)

	// Absolute patterns should match standalone episode numbers
	if !matchesAny(absolute, "Show - 05 [720p]") {
		t.Error("absolute should match 'Show - 05 [720p]'")
	}
	if !matchesAny(absolute, "Show 05") {
		t.Error("absolute should match 'Show 05' at end")
	}

	// Resolution guard should catch false positives like "1080p" for episode 1080
	_, _, guard1080 := buildEpisodePatterns(1, 1080)
	if !guard1080.MatchString("Show 1080p") {
		t.Error("resolution guard should match '1080p' for episode 1080")
	}
	if guard.MatchString("Show 720p") {
		t.Error("resolution guard for ep5 should NOT match '720p'")
	}
}

func TestMatchesAny(t *testing.T) {
	standard, _, _ := buildEpisodePatterns(2, 3)

	if !matchesAny(standard, "Show S02E03 1080p") {
		t.Error("matchesAny should find S02E03")
	}
	if matchesAny(standard, "Show S02E04 1080p") {
		t.Error("matchesAny should not find S02E04")
	}
	if matchesAny(nil, "anything") {
		t.Error("matchesAny with nil patterns should return false")
	}
	if matchesAny(standard, "") {
		t.Error("matchesAny with empty title should return false")
	}
}

func TestMatchesRejectPattern(t *testing.T) {
	cfg := &config.Config{}
	cfg.Automation.RejectCommon = []string{`\bvostfr\b`, `\bhdcam\b`}
	cfg.Automation.MinSeeders = 0
	cfg.App.FilterLogLevel = "none"

	matcher := NewMatcherService(cfg, testLogger())
	ts := NewTorrentSelector(cfg, matcher, testLogger())

	if p := ts.matchesRejectPattern("Movie 1080p VOSTFR"); p == nil {
		t.Error("should match VOSTFR reject pattern")
	}
	if p := ts.matchesRejectPattern("Movie 1080p HDTV"); p != nil {
		t.Errorf("should NOT match clean title, but matched: %s", p.String())
	}
}

func TestNewTorrentSelector_InvalidRejectPattern(t *testing.T) {
	cfg := &config.Config{}
	cfg.Automation.RejectCommon = []string{`\bvalid\b`, `[broken`}
	cfg.App.FilterLogLevel = "none"

	matcher := NewMatcherService(cfg, testLogger())
	ts := NewTorrentSelector(cfg, matcher, testLogger())

	if len(ts.rejectPatterns) != 1 {
		t.Errorf("expected 1 valid pattern (1 invalid skipped), got %d", len(ts.rejectPatterns))
	}
}

func TestFilterByEpisodeNumber_AbsoluteNumbering(t *testing.T) {
	ts := testSelector()

	results := []indexers.IndexerResult{
		{Title: "Anime - 05 [720p]"},
		{Title: "Anime - 06 [1080p]"},
		{Title: "Anime 05"},
	}

	stats := &FilterStats{}
	filtered := ts.filterByEpisodeNumber(results, 1, 5, stats)

	if len(filtered) != 2 {
		t.Errorf("expected 2 results (ep 05), got %d", len(filtered))
		for _, r := range filtered {
			t.Logf("  passed: %s", r.Title)
		}
	}
}

func TestSeasonPackPattern(t *testing.T) {
	packs := []string{
		"Made in Abyss S01 Complete 1080p BluRay",
		"Made.in.Abyss.S01.Complete.1080p",
		"Made in Abyss S02 FULL 720p",
		"Made in Abyss S01 Pack 1080p",
		"Made in Abyss S01 Batch 1080p",
		"Made in Abyss Complete Season 1080p",
		"Made in Abyss Complete Series 1080p",
		"Made in Abyss Season 1 Complete 1080p",
		"Made in Abyss Season 2 Full 1080p",
	}
	for _, title := range packs {
		if !seasonPackPattern.MatchString(title) {
			t.Errorf("should detect as season pack: %q", title)
		}
	}

	episodes := []string{
		"Made in Abyss S01E01 1080p BluRay",
		"Made.in.Abyss.S02E05.720p.HEVC",
		"Made in Abyss - 01 [720p]",
		"Made in Abyss S01E01 Complete Nonsense Group",
	}
	for _, title := range episodes {
		if seasonPackPattern.MatchString(title) {
			t.Errorf("should NOT detect as season pack: %q", title)
		}
	}
}

func TestFilterAndScore_SkipsQualityForEbooks(t *testing.T) {
	ts := testSelector()
	media := testMedia("ebook")
	media.Title = "Test Book"
	media.MinQuality = "720p"
	media.MaxQuality = "1080p"

	results := []indexers.IndexerResult{
		{Title: "Test Book epub", Seeders: 10},
		{Title: "Test Book pdf", Seeders: 5},
	}

	filtered := ts.FilterAndScoreTorrents(media, results, 0, 0, []string{"Test Book"})

	// Ebooks should skip quality filtering entirely
	if len(filtered) != 2 {
		t.Errorf("expected 2 results (quality filter skipped for ebooks), got %d", len(filtered))
	}
}
