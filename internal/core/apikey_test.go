package core

import "testing"

func TestAPIKeyMissing(t *testing.T) {
	for key, want := range map[string]bool{
		"":                          true,
		"   ":                       true,
		"REPLACE_WITH_TMDB_API_KEY": true,
		"0123456789abcdef":          false,
	} {
		if got := apiKeyMissing(key); got != want {
			t.Errorf("apiKeyMissing(%q) = %v, want %v", key, got, want)
		}
	}
}
