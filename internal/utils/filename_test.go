package utils

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"Clean name", "Movie Title", "Movie Title"},
		{"Colons", "Movie: The Sequel", "Movie The Sequel"},
		{"Angle brackets", "Movie <2024>", "Movie 2024"},
		{"Quotes", `Movie "Title"`, "Movie Title"},
		{"Pipe", "Movie | Title", "Movie  Title"},
		{"Question mark", "Who?", "Who"},
		{"Asterisk", "Star*Wars", "StarWars"},
		{"Backslash", `Path\Name`, "PathName"},
		{"Forward slash", "Path/Name", "PathName"},
		{"Trailing dots", "Movie...", "Movie"},
		{"Trailing spaces", "Movie   ", "Movie"},
		{"Mixed special chars", `<Movie>: "The *Best?"`, "Movie The Best"},
		{"Empty after sanitize dots", "...", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
