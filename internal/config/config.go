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

type FileRenamingConfig struct {
	MovieTemplate  string `yaml:"movie_template"`
	SeriesTemplate string `yaml:"series_template"`
	AnimeTemplate  string `yaml:"anime_template"`
}

type PostProcessingConfig struct {
	Enabled                 bool             `yaml:"enabled"`
	ValidateFiles           bool             `yaml:"validate_files"`
	MinFileSizeMB           int              `yaml:"min_file_size_mb"`
	WaitForFileTimeout      int              `yaml:"wait_for_file_timeout"`       // seconds
	RetryAttempts           int              `yaml:"retry_attempts"`
	RetryDelay              int              `yaml:"retry_delay"`                 // seconds
	CleanupOnFailure        bool             `yaml:"cleanup_on_failure"`
	KeepFailedDownloadsDays int              `yaml:"keep_failed_downloads_days"`
	Pipeline                PipelineConfig   `yaml:"pipeline"`
}

type PipelineConfig struct {
	Enabled  bool          `yaml:"enabled"`  // Use pipeline architecture
	Rollback bool          `yaml:"rollback"` // Rollback on failure
	Stages   []StageConfig `yaml:"stages"`   // Stage configuration (empty = use defaults)
}

type StageConfig struct {
	Name      string            `yaml:"name"`                 // Stage name: validate, create_folders, move_files, rename, subtitles, notify
	Enabled   bool              `yaml:"enabled"`              // Enable/disable this stage
	Condition string            `yaml:"condition,omitempty"`  // Optional condition: e.g., "file_size > 100MB"
	Options   map[string]string `yaml:"options,omitempty"`    // Stage-specific options
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
		SearchTimeout          int    `yaml:"search_timeout"`
	} `yaml:"app"`

	TorrentClient struct {
		Type         string `yaml:"type"`
		Host         string `yaml:"host"`
		Username     string `yaml:"username"`
		Password     string `yaml:"password"`
		Secret       string `yaml:"secret"`
		DownloadPath string `yaml:"download_path"`
	} `yaml:"torrent_client"`

	Metadata struct {
		Language string `yaml:"language"`
		Timeout  int    `yaml:"timeout"`
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
		Trakt struct {
			ClientID string `yaml:"client_id"`
		} `yaml:"trakt"`
	} `yaml:"metadata"`

	Subtitles struct {
		Enabled      bool     `yaml:"enabled"`
		APIKey       string   `yaml:"api_key"`
		Languages    []string `yaml:"languages"`
		DownloadPath string   `yaml:"download_path"`
	} `yaml:"subtitles"`

	Movies struct {
		Providers         []string       `yaml:"providers"`
		Sources           []SourceConfig `yaml:"sources"`
		DownloadFolder    string         `yaml:"download_folder"`
		DestinationFolder string         `yaml:"destination_folder"`
		MoveMethod        []string       `yaml:"move_method"`
	} `yaml:"movies"`

	TVShows struct {
		Providers         []string       `yaml:"providers"`
		Sources           []SourceConfig `yaml:"sources"`
		DownloadFolder    string         `yaml:"download_folder"`
		DestinationFolder string         `yaml:"destination_folder"`
		MoveMethod        []string       `yaml:"move_method"`
	} `yaml:"tv-shows"`

	Anime struct {
		Providers         []string       `yaml:"providers"`
		Sources           []SourceConfig `yaml:"sources"`
		DownloadFolder    string         `yaml:"download_folder"`
		DestinationFolder string         `yaml:"destination_folder"`
		MoveMethod        []string       `yaml:"move_method"`
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
		SearchInterval            string   `yaml:"search_interval"` // Default: @every 30m
		RSSProcessingInterval     string   `yaml:"rss_processing_interval"`
		DownloadStatusInterval    string   `yaml:"download_status_interval"`
		NewEpisodesCheckInterval  string   `yaml:"new_episodes_check_interval"`
		CleanupInterval           string   `yaml:"cleanup_interval"`
		RetryFailedInterval       string   `yaml:"retry_failed_interval"`
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

	FileRenaming   FileRenamingConfig   `yaml:"file_renaming"`
	PostProcessing PostProcessingConfig `yaml:"postprocessing"`
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
	cfg.setDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func loadFromEnv(cfg *Config) {
	// Environment variable overrides will go here if needed
}

// setDefaults sets default values for optional fields
func (c *Config) setDefaults() {
	// App defaults
	if c.App.Port == 0 {
		c.App.Port = 8081
	}
	if c.App.DataPath == "" {
		c.App.DataPath = "./data"
	}
	if c.App.SearchTimeout == 0 {
		c.App.SearchTimeout = 30
	}

	// Metadata defaults
	if c.Metadata.Language == "" {
		c.Metadata.Language = "en"
	}
	if c.Metadata.Timeout == 0 {
		c.Metadata.Timeout = 10
	}

	// PostProcessing defaults
	if c.PostProcessing.Enabled {
		if c.PostProcessing.WaitForFileTimeout == 0 {
			c.PostProcessing.WaitForFileTimeout = 30
		}
		if c.PostProcessing.RetryAttempts == 0 {
			c.PostProcessing.RetryAttempts = 3
		}
		if c.PostProcessing.RetryDelay == 0 {
			c.PostProcessing.RetryDelay = 5
		}
		if c.PostProcessing.MinFileSizeMB == 0 {
			c.PostProcessing.MinFileSizeMB = 1
		}
		// Pipeline defaults
		c.PostProcessing.Pipeline.Rollback = true // Default to rollback on failure

		// Default stage ordering if not specified
		if len(c.PostProcessing.Pipeline.Stages) == 0 {
			c.PostProcessing.Pipeline.Stages = []StageConfig{
				{Name: "validate", Enabled: true},
				{Name: "create_folders", Enabled: true},
				{Name: "move_files", Enabled: true},
				{Name: "rename", Enabled: true},
				{Name: "subtitles", Enabled: true},
				{Name: "notify", Enabled: true},
			}
		}
	}
}

// Validate checks if the configuration is valid
// Uses simple conditionals to minimize memory allocations
func (c *Config) Validate() error {
	// Validate app settings
	if c.App.Port < 1 || c.App.Port > 65535 {
		return fmt.Errorf("app.port must be between 1 and 65535")
	}
	if c.App.DataPath == "" {
		return fmt.Errorf("app.data_path is required")
	}
	if c.App.JWTSecret == "" {
		return fmt.Errorf("app.jwt_secret is required")
	}

	// Validate torrent client
	if c.TorrentClient.Type == "" {
		return fmt.Errorf("torrent_client.type is required")
	}
	validTorrentClients := [6]string{"transmission", "qbittorrent", "aria2", "deluge", "mock", ""}
	valid := false
	for i := 0; i < 5; i++ {
		if c.TorrentClient.Type == validTorrentClients[i] {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("torrent_client.type must be one of: transmission, qbittorrent, aria2, deluge, mock")
	}
	if c.TorrentClient.Host == "" {
		return fmt.Errorf("torrent_client.host is required")
	}

	// Validate database
	if c.Database.Path == "" {
		return fmt.Errorf("database.path is required")
	}

	// Validate media type configs
	if err := c.validateMediaType("movies", c.Movies.DownloadFolder, c.Movies.DestinationFolder, c.Movies.MoveMethod); err != nil {
		return err
	}
	if err := c.validateMediaType("tv-shows", c.TVShows.DownloadFolder, c.TVShows.DestinationFolder, c.TVShows.MoveMethod); err != nil {
		return err
	}
	if err := c.validateMediaType("anime", c.Anime.DownloadFolder, c.Anime.DestinationFolder, c.Anime.MoveMethod); err != nil {
		return err
	}

	// Validate subtitles
	if c.Subtitles.Enabled && c.Subtitles.APIKey == "" {
		return fmt.Errorf("subtitles.api_key is required when subtitles are enabled")
	}

	// Validate postprocessing
	if c.PostProcessing.Enabled {
		if c.PostProcessing.WaitForFileTimeout < 1 {
			return fmt.Errorf("postprocessing.wait_for_file_timeout must be at least 1 second")
		}
		if c.PostProcessing.RetryAttempts < 1 {
			return fmt.Errorf("postprocessing.retry_attempts must be at least 1")
		}
		if c.PostProcessing.MinFileSizeMB < 0 {
			return fmt.Errorf("postprocessing.min_file_size_mb cannot be negative")
		}
	}

	return nil
}

// validateMediaType validates folder paths and move methods for a media type
func (c *Config) validateMediaType(mediaType, downloadFolder, destFolder string, moveMethods []string) error {
	if downloadFolder == "" {
		return fmt.Errorf("%s.download_folder is required", mediaType)
	}
	if destFolder == "" {
		return fmt.Errorf("%s.destination_folder is required", mediaType)
	}
	if len(moveMethods) == 0 {
		return fmt.Errorf("%s.move_method must have at least one method", mediaType)
	}

	// Validate move methods
	validMethods := [4]string{"hardlink", "symlink", "move", "copy"}
	for _, method := range moveMethods {
		valid := false
		for i := 0; i < 4; i++ {
			if method == validMethods[i] {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("%s.move_method contains invalid method '%s' (valid: hardlink, symlink, move, copy)", mediaType, method)
		}
	}

	return nil
}
