package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_ValidationFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	// Valid YAML but missing required fields
	os.WriteFile(path, []byte("app:\n  port: 0\n"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.setDefaults()

	if cfg.App.Port != 8081 {
		t.Errorf("expected default port 8081, got %d", cfg.App.Port)
	}
	if cfg.App.DataPath != "./data" {
		t.Errorf("expected default data_path './data', got %q", cfg.App.DataPath)
	}
	if cfg.App.SearchTimeout != 30 {
		t.Errorf("expected default search_timeout 30, got %d", cfg.App.SearchTimeout)
	}
	if cfg.Metadata.Language != "en" {
		t.Errorf("expected default language 'en', got %q", cfg.Metadata.Language)
	}
	if cfg.Metadata.Timeout != 10 {
		t.Errorf("expected default metadata timeout 10, got %d", cfg.Metadata.Timeout)
	}
}

func TestSetDefaults_PostProcessing(t *testing.T) {
	cfg := &Config{}
	cfg.PostProcessing.Enabled = true
	cfg.setDefaults()

	if cfg.PostProcessing.WaitForFileTimeout != 30 {
		t.Errorf("expected default wait_for_file_timeout 30, got %d", cfg.PostProcessing.WaitForFileTimeout)
	}
	if cfg.PostProcessing.RetryAttempts != 3 {
		t.Errorf("expected default retry_attempts 3, got %d", cfg.PostProcessing.RetryAttempts)
	}
	if cfg.PostProcessing.RetryDelay != 5 {
		t.Errorf("expected default retry_delay 5, got %d", cfg.PostProcessing.RetryDelay)
	}
	if cfg.PostProcessing.MinFileSizeMB != 1 {
		t.Errorf("expected default min_file_size_mb 1, got %d", cfg.PostProcessing.MinFileSizeMB)
	}
	if !cfg.PostProcessing.Pipeline.Rollback {
		t.Error("expected pipeline rollback default to true")
	}
	if len(cfg.PostProcessing.Pipeline.Stages) == 0 {
		t.Error("expected default pipeline stages to be populated")
	}
}

func validConfig() *Config {
	cfg := &Config{}
	cfg.App.Port = 8081
	cfg.App.DataPath = "./data"
	cfg.App.JWTSecret = "test-secret"
	cfg.TorrentClient.Type = "transmission"
	cfg.TorrentClient.Host = "http://localhost:9091"
	cfg.Database.Path = "./data/reel.db"
	cfg.Movies.DownloadFolder = "/downloads/movies"
	cfg.Movies.DestinationFolder = "/media/movies"
	cfg.Movies.MoveMethod = []string{"hardlink"}
	cfg.TVShows.DownloadFolder = "/downloads/tv"
	cfg.TVShows.DestinationFolder = "/media/tv"
	cfg.TVShows.MoveMethod = []string{"hardlink"}
	cfg.Anime.DownloadFolder = "/downloads/anime"
	cfg.Anime.DestinationFolder = "/media/anime"
	cfg.Anime.MoveMethod = []string{"hardlink"}
	return cfg
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := validConfig()
	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no validation error, got: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{}
	cfg.App.Port = 99999
	cfg.App.DataPath = "./data"
	cfg.App.JWTSecret = "secret"

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestValidate_MissingJWTSecret(t *testing.T) {
	cfg := &Config{}
	cfg.App.Port = 8081
	cfg.App.DataPath = "./data"

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing JWT secret")
	}
}

func TestValidate_MissingTorrentClient(t *testing.T) {
	cfg := &Config{}
	cfg.App.Port = 8081
	cfg.App.DataPath = "./data"
	cfg.App.JWTSecret = "secret"

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing torrent client type")
	}
}

func TestValidate_InvalidTorrentClient(t *testing.T) {
	cfg := &Config{}
	cfg.App.Port = 8081
	cfg.App.DataPath = "./data"
	cfg.App.JWTSecret = "secret"
	cfg.TorrentClient.Type = "invalid_client"

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid torrent client type")
	}
}

func TestSave_And_Reload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	cfg := &Config{}
	cfg.App.Port = 9090
	cfg.App.DataPath = "/tmp/data"
	cfg.App.JWTSecret = "test"

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read saved config: %v", err)
	}
	if len(data) == 0 {
		t.Error("saved config file is empty")
	}
}
