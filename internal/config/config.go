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

	Indexer struct {
		Type   string `yaml:"type"`
		APIKey string `yaml:"api_key"`
		URL    string `yaml:"url"`
	} `yaml:"indexer"`

	TorrentClient struct {
		Type         string `yaml:"type"`
		Host         string `yaml:"host"`
		Username     string `yaml:"username"`
		Password     string `yaml:"password"`
		DownloadPath string `yaml:"download_path"`
	} `yaml:"torrent_client"`

	Metadata struct {
		Providers []string `yaml:"providers"`
		TMDB      struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"tmdb"`
		IMDB struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"imdb"`
		Language string `yaml:"language"`
	} `yaml:"metadata"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Automation struct {
		SearchInterval         string   `yaml:"search_interval"`
		MaxConcurrentDownloads int      `yaml:"max_concurrent_downloads"`
		QualityPreferences     []string `yaml:"quality_preferences"`
		MinSeeders             int      `yaml:"min_seeders"`
	} `yaml:"automation"`
}

func Load(path string) (*Config, error) {
	// *** Check if the config file exists first ***
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// *** Return a clear error if the file is not found ***
		return nil, fmt.Errorf("config file not found at '%s'", path)
	}

	cfg := &Config{}

	// No need to set defaults here anymore if the file is mandatory

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
