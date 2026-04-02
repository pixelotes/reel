package utils

import "testing"

func TestParseQuality(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"1080p", "Show.S01E01.1080p.WEB-DL.x264", "1080p"},
		{"720p", "Movie.720p.BluRay", "720p"},
		{"2160p", "Movie.2160p.UHD.BluRay", "2160p"},
		{"4K alias", "Movie.4K.HDR", "2160p"},
		{"480p", "Show.480p.HDTV", "480p"},
		{"360p", "Show.360p", "360p"},
		{"WEB-DL no resolution", "Show.S01E01.WEB-DL", "WEB-DL"},
		{"BluRay no resolution", "Movie.BluRay.x264", "BluRay"},
		{"HDTV no resolution", "Show.HDTV.x264", "HDTV"},
		{"DVDRip", "Movie.DVDRip", "DVDRip"},
		{"WEBRip", "Show.WEBRip.x265", "WEBRip"},
		{"Empty string", "", "Unknown"},
		{"No quality info", "random-file-name", "Unknown"},
		{"Case insensitive", "show.1080P.web-dl", "1080p"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseQuality(tt.input)
			if got != tt.want {
				t.Errorf("ParseQuality(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseQualityDetailed(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantRes    string
		wantSource string
	}{
		{"Full info", "Show.1080p.WEB-DL.x264", "1080p", "WEB-DL"},
		{"BluRay 720p", "Movie.720p.BluRay", "720p", "BluRay"},
		{"No resolution", "Show.WEBRip", "Unknown", "WEBRip"},
		{"No source", "Show.1080p.x264", "1080p", "Unknown"},
		{"Empty", "", "Unknown", "Unknown"},
		{"HDTV", "Show.480p.HDTV", "480p", "HDTV"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, src := ParseQualityDetailed(tt.input)
			if res != tt.wantRes {
				t.Errorf("resolution = %q, want %q", res, tt.wantRes)
			}
			if src != tt.wantSource {
				t.Errorf("source = %q, want %q", src, tt.wantSource)
			}
		})
	}
}

func TestCompareQuality(t *testing.T) {
	tests := []struct {
		name string
		q1   string
		q2   string
		want int
	}{
		{"1080p > 720p", "1080p", "720p", 1},
		{"720p < 1080p", "720p", "1080p", -1},
		{"Same resolution", "1080p", "1080p", 0},
		{"2160p > 1080p", "2160p", "1080p", 1},
		{"BluRay > HDTV", "BluRay", "HDTV", 1},
		{"WEB-DL > WEBRip", "WEB-DL", "WEBRip", 1},
		{"Unknown == Unknown", "Unknown", "Unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareQuality(tt.q1, tt.q2)
			if got != tt.want {
				t.Errorf("CompareQuality(%q, %q) = %d, want %d", tt.q1, tt.q2, got, tt.want)
			}
		})
	}
}
