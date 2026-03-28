package services

import (
	"os"
	"testing"

	"reel/internal/config"
	"reel/internal/utils"
)

func TestMatcherService_Matches(t *testing.T) {
	cfg := &config.Config{}

	// Trap cases from user request
	// Case 1: "The 100 Humans" (False positive for "The 100") -> reject
	// Case 2: "Flashpoint" (False positive for "The Flash") -> reject
	// Case 3: "Mr.Robot.S01E01.1080p.HEVC" (Match with noise) -> pass

	tests := []struct {
		name      string
		query     string
		candidate string
		want      bool
	}{
		{"Exact Match", "The 100", "The 100 S01E01", true},
		{"Case Insensitive", "the 100", "THE 100 S01E01", true},
		{"Trap Case 1: The 100 Humans", "The 100", "The 100 Humans S01E01", false},
		{"Trap Case 2: Flashpoint", "The Flash", "Flashpoint S01E01", false},
		{"Trap Case 3: Mr Robot", "Mr Robot", "Mr.Robot.S01E01", true},
		{"Partial Word", "Flash", "The Flash", true},
		{"Partial Word Reverse", "The Flash", "Flash", false},
	}

	logger := utils.NewLogger(true, os.Stdout)
	matcher := NewMatcherService(cfg, logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matcher.Matches(tt.query, tt.candidate)
			if got != tt.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tt.query, tt.candidate, got, tt.want)
			}
		})
	}
}
