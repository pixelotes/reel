package config

import (
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
		Type   string `yaml:"type"` // New: 'scarf' is the only option for now
		APIKey string `yaml:"api_key"`
		URL    string `yaml:"url"`
	} `yaml:"indexer"`

	TorrentClient struct {
		Type         string `yaml:"type"` // 'transmission' or 'qbittorrent'
		Host         string `yaml:"host"`
		Username     string `yaml:"username"`
		Password     string `yaml:"password"`
		DownloadPath string `yaml:"download_path"`
	} `yaml:"torrent_client"`

	Metadata struct {
		// New: A list of providers to try in order
		Providers []string `yaml:"providers"`
		TMDB      struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"tmdb"`
		IMDB struct {
			// IMDb doesn't have a public API, this is a placeholder
			// for a potential future implementation (e.g., web scraping)
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
	cfg := &Config{}
	setDefaults(cfg)

	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	loadFromEnv(cfg)
	return cfg, nil
}

func setDefaults(cfg *Config) {
	cfg.App.Port = 8081
	cfg.App.DataPath = "./data"
	cfg.App.UIEnabled = true
	cfg.App.UIPassword = "password"
	cfg.App.Debug = false

	cfg.Indexer.Type = "scarf"
	cfg.Indexer.URL = "http://localhost:8080"

	cfg.TorrentClient.Type = "transmission"
	cfg.TorrentClient.Host = "localhost:9091"
	cfg.TorrentClient.DownloadPath = "/downloads/media"

	cfg.Metadata.Providers = []string{"tmdb"} // Default to TMDB
	cfg.Metadata.Language = "en"

	cfg.Database.Path = "./data/reel.db"

	cfg.Automation.SearchInterval = "1h"
	cfg.Automation.MaxConcurrentDownloads = 3
	cfg.Automation.QualityPreferences = []string{"1080p", "720p"}
	cfg.Automation.MinSeeders = 5
}

func loadFromEnv(cfg *Config) {
	// Add environment variable overrides here if needed
}
