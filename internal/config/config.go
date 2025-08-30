package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		Port       int    `yaml:"port"`
		DataPath   string `yaml:"data_path"`
		UIEnabled  bool   `yaml:"ui_enabled"`
		UIPassword string `yaml:"ui_password"`
		Debug      bool   `yaml:"debug"`
		JWTSecret  string `yaml:"jwt_secret"`
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
		AniDB struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"anidb"`
	} `yaml:"metadata"`

	Movies struct {
		Providers []string `yaml:"providers"`
		Sources   []struct {
			Type   string `yaml:"type"`
			URL    string `yaml:"url"`
			APIKey string `yaml:"api_key"`
		} `yaml:"sources"`
	} `yaml:"movies"`

	TVShows struct {
		Providers []string `yaml:"providers"`
		Sources   []struct {
			Type   string `yaml:"type"`
			URL    string `yaml:"url"`
			APIKey string `yaml:"api_key"`
		} `yaml:"sources"`
	} `yaml:"tv-shows"`

	Anime struct {
		Providers []string `yaml:"providers"`
		Sources   []struct {
			Type   string `yaml:"type"`
			URL    string `yaml:"url"`
			APIKey string `yaml:"api_key"`
		} `yaml:"sources"`
	} `yaml:"anime"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Automation struct {
		SearchInterval         string   `yaml:"search_interval"`
		MaxConcurrentDownloads int      `yaml:"max_concurrent_downloads"`
		QualityPreferences     []string `yaml:"quality_preferences"`
		MinSeeders             int      `yaml:"min_seeders"`
		RejectCommon           []string `yaml:"reject-common"`
	} `yaml:"automation"`
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
