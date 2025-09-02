package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SourceConfig defines the structure for an indexer source
type SourceConfig struct {
	Type       string `yaml:"type"`
	URL        string `yaml:"url"`
	APIKey     string `yaml:"api_key"`
	SearchMode string `yaml:"search_mode,omitempty"`
}

type Config struct {
	App struct {
		Port                   int    `yaml:"port"`
		DataPath               string `yaml:"data_path"`
		UIEnabled              bool   `yaml:"ui_enabled"`
		UIPassword             string `yaml:"ui_password"`
		Debug                  bool   `yaml:"debug"`
		JWTSecret              string `yaml:"jwt_secret"`
		FilterLogLevel         string `yaml:"filter_log_level"` // "none" or "detail"
		MagnetToTorrentEnabled bool   `yaml:"magnet_to_torrent_enabled"`
		MagnetToTorrentTimeout int    `yaml:"magnet_to_torrent_timeout"`
	} `yaml:"app"`

	TorrentClient struct {
		Type         string `yaml:"type"`
		Host         string `yaml:"host"`
		Username     string `yaml:"username"`
		Password     string `yaml:"password"`
		DownloadPath string `yaml:"download_path"`
	} `yaml:"torrent_client"`

	Metadata struct {
		Language string `yaml:"language"`
		TMDB     struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"tmdb"`
		IMDB struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"imdb"`
		TVmaze struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"tvmaze"`
		AniList struct {
			// AniList doesn't require an API key for public queries
		} `yaml:"anilist"`
		Trakt struct { // Add this section
			ClientID string `yaml:"client_id"`
		} `yaml:"trakt"`
	} `yaml:"metadata"`

	Movies struct {
		Providers         []string       `yaml:"providers"`
		Sources           []SourceConfig `yaml:"sources"`
		DownloadFolder    string         `yaml:"download_folder"`
		DestinationFolder string         `yaml:"destination_folder"`
		MoveMethod        string         `yaml:"move_method"`
	} `yaml:"movies"`

	TVShows struct {
		Providers         []string       `yaml:"providers"`
		Sources           []SourceConfig `yaml:"sources"`
		DownloadFolder    string         `yaml:"download_folder"`
		DestinationFolder string         `yaml:"destination_folder"`
		MoveMethod        string         `yaml:"move_method"`
	} `yaml:"tv-shows"`

	Anime struct {
		Providers         []string       `yaml:"providers"`
		Sources           []SourceConfig `yaml:"sources"`
		DownloadFolder    string         `yaml:"download_folder"`
		DestinationFolder string         `yaml:"destination_folder"`
		MoveMethod        string         `yaml:"move_method"`
	} `yaml:"anime"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Notifications struct {
		Pushbullet struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"pushbullet"`
	} `yaml:"notifications"`

	Automation struct {
		SearchInterval            string   `yaml:"search_interval"`
		MaxConcurrentDownloads    int      `yaml:"max_concurrent_downloads"`
		QualityPreferences        []string `yaml:"quality_preferences"`
		MinSeeders                int      `yaml:"min_seeders"`
		KeepTorrentsForDays       int      `yaml:"keep_torrents_for_days"`
		KeepTorrentsSeedRatio     float64  `yaml:"keep_torrents_seed_ratio"`
		EpisodeDownloadDelayHours int      `yaml:"episode_download_delay_hours"`
		RejectCommon              []string `yaml:"reject-common"`
		Notifications             []string `yaml:"notifications"`
	} `yaml:"automation"`

	RejectCommon      []string `yaml:"reject-common"`
	ExtraTrackersList []string `yaml:"extra_trackers_list"`
}

func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found at '%s'", path)
	}

	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	loadFromEnv(cfg)
	return cfg, nil
}

func loadFromEnv(cfg *Config) {
	// Add environment variable overrides here if needed
}
