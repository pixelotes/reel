package services

import (
	"testing"

	"reel/internal/config"
)

func TestNewMatcherService_PrecompilesRejectPatterns(t *testing.T) {
	cfg := &config.Config{}
	cfg.Automation.RejectCommon = []string{`\bvostfr\b`, `\bhdcam\b`, `\bscreener\b`}

	matcher := NewMatcherService(cfg, testLogger())

	if len(matcher.rejectPatterns) != 3 {
		t.Errorf("expected 3 pre-compiled patterns, got %d", len(matcher.rejectPatterns))
	}
}

func TestNewMatcherService_SkipsInvalidPatterns(t *testing.T) {
	cfg := &config.Config{}
	cfg.Automation.RejectCommon = []string{`\bvalid\b`, `[invalid`, `another\bvalid\b`}

	matcher := NewMatcherService(cfg, testLogger())

	// "[invalid" is bad regex, should be skipped
	if len(matcher.rejectPatterns) != 2 {
		t.Errorf("expected 2 valid patterns (1 invalid skipped), got %d", len(matcher.rejectPatterns))
	}
}

func TestNewMatcherService_EmptyRejectList(t *testing.T) {
	cfg := &config.Config{}
	matcher := NewMatcherService(cfg, testLogger())

	if len(matcher.rejectPatterns) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(matcher.rejectPatterns))
	}

	// Should not reject anything
	if !matcher.Matches("test", "test S01E01") {
		t.Error("expected match with no reject patterns")
	}
}

func TestIsIgnorable_AllCategories(t *testing.T) {
	cfg := &config.Config{}
	matcher := NewMatcherService(cfg, testLogger())

	tests := []struct {
		token string
		want  bool
	}{
		// Season/Episode patterns
		{"s01e01", true},
		{"S02E03", true},
		{"1x01", true},
		{"s01", true},
		{"e05", true},
		// Years
		{"2024", true},
		{"1999", true},
		{"2100", false},
		{"1899", false},
		// Resolutions
		{"1080p", true},
		{"720p", true},
		{"4k", true},
		{"8k", true},
		// Tech terms
		{"hevc", true},
		{"x264", true},
		{"bluray", true},
		{"remux", true},
		{"atmos", true},
		// Stop words
		{"the", true},
		{"and", true},
		{"season", true},
		// Non-ignorable
		{"randomword", false},
		{"batman", false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			got := matcher.IsIgnorable(tt.token)
			if got != tt.want {
				t.Errorf("IsIgnorable(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestMatchesLoose(t *testing.T) {
	cfg := &config.Config{}
	cfg.Automation.RejectCommon = []string{`\bhdcam\b`, `\bscreener\b`}
	matcher := NewMatcherService(cfg, testLogger())

	tests := []struct {
		name      string
		query     string
		candidate string
		want      bool
	}{
		{"Exact match", "The Other Bennet Sister", "The.Other.Bennet.Sister.S01E01.1080p.HEVC.x265-MeGusta", true},
		{"Release group CBFM", "The Other Bennet Sister", "The Other Bennet Sister S01E01 1080p WEBRip x264-CBFM", true},
		{"With EZTV tag", "The Other Bennet Sister", "The Other Bennet Sister S01E01 1080p HEVC x265 MeGusta EZTV", true},
		{"Missing significant word", "The Other Bennet Sister", "The Other Sister S01E01 1080p", false},
		{"Completely different", "The Other Bennet Sister", "Flashpoint S01E05 1080p HDTV", false},
		{"Reject pattern", "The Other Bennet Sister", "The Other Bennet Sister S01E01 HDCAM", false},
		{"Ignores stop words in query", "The Flash", "Flash S01E01 1080p HDTV x264 FLUX", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matcher.MatchesLoose(tt.query, tt.candidate)
			if got != tt.want {
				t.Errorf("MatchesLoose(%q, %q) = %v, want %v", tt.query, tt.candidate, got, tt.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	cfg := &config.Config{}
	matcher := NewMatcherService(cfg, testLogger())

	tests := []struct {
		input string
		want  string
	}{
		{"Mr.Robot.S01E01", "mr robot s01e01"},
		{"The_Flash-S02E03", "the flash s02e03"},
		{"Hello   World", "hello world"},
		{"Special!@#$%Characters", "specialcharacters"},
		{"UPPER.case_MIX-ed", "upper case mix ed"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := matcher.Normalize(tt.input)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
