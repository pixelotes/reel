package torrent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"reel/internal/utils"
)

// MockClient implements TorrentClient for testing purposes.
type MockClient struct {
	torrents map[string]*TorrentStatus
	logger   *utils.Logger
}

// NewMockClient creates a new instance of MockClient.
func NewMockClient(logger *utils.Logger) *MockClient {
	return &MockClient{
		torrents: make(map[string]*TorrentStatus),
		logger:   logger,
	}
}

// AddTorrent simulates adding a magnet link.
func (c *MockClient) AddTorrent(magnetLink string, downloadPath string) (string, error) {
	c.logger.Info("MockClient: Adding torrent from magnet:", magnetLink)

	// Generate a mock hash
	hash, err := generateRandomHash()
	if err != nil {
		return "", err
	}

	// Extract name from magnet or use default
	name := "Mock_Video_S01E01.mkv" // Default
	if strings.Contains(magnetLink, "dn=") {
		parts := strings.Split(magnetLink, "dn=")
		if len(parts) > 1 {
			nameParam := strings.Split(parts[1], "&")[0]
			// rudimentary decoding for the mock
			name = strings.ReplaceAll(nameParam, "+", " ")
			// Ensure it has an extension for post-processor
			if !strings.HasSuffix(name, ".mkv") && !strings.HasSuffix(name, ".mp4") {
				name += ".mkv"
			}
		}
	}

	return c.createMockDownload(hash, name, downloadPath)
}

// AddTorrentFile simulates adding a torrent file.
func (c *MockClient) AddTorrentFile(fileContent []byte, downloadPath string) (string, error) {
	c.logger.Info("MockClient: Adding torrent from file")

	hash, err := generateRandomHash()
	if err != nil {
		return "", err
	}

	name := fmt.Sprintf("Mock_Torrent_File_%s.mkv", hash[:8])
	return c.createMockDownload(hash, name, downloadPath)
}

func (c *MockClient) createMockDownload(hash, name, downloadPath string) (string, error) {
	// Create the download directory if it doesn't exist
	if err := os.MkdirAll(downloadPath, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create download dir: %w", err)
	}

	filePath := filepath.Join(downloadPath, name)

	// Create dummy file > 128KB (e.g. 200KB)
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create mock file: %w", err)
	}
	defer f.Close()

	// Write 200KB of zeros (or random data)
	dummyData := make([]byte, 200*1024)
	if _, err := rand.Read(dummyData); err != nil {
		return "", err
	}
	if _, err := f.Write(dummyData); err != nil {
		return "", err
	}

	// Update internal state
	status := &TorrentStatus{
		Hash:         hash,
		Name:         name,
		Progress:     1.0,            // 100%
		Files:        []string{name}, // Relative path usually, or just filename
		DownloadDir:  downloadPath,
		IsCompleted:  true,
		DownloadRate: 1024 * 1024 * 10, // 10 MB/s
		UploadRate:   0,
		ETA:          0,
		UploadRatio:  2.0,
	}
	c.torrents[hash] = status

	c.logger.Info("MockClient: Download simulated:", filePath)
	return hash, nil
}

// GetTorrentStatus returns the status of a torrent.
func (c *MockClient) GetTorrentStatus(hash string) (TorrentStatus, error) {
	status, ok := c.torrents[hash]
	if !ok {
		return TorrentStatus{}, fmt.Errorf("torrent not found")
	}
	return *status, nil
}

// RemoveTorrent removes a torrent.
func (c *MockClient) RemoveTorrent(hash string) error {
	c.logger.Info("MockClient: Removing torrent:", hash)
	delete(c.torrents, hash)
	return nil
}

// AddTrackers is a no-op.
func (c *MockClient) AddTrackers(hash string, trackers []string) error {
	return nil
}

// HealthCheck returns true.
func (c *MockClient) HealthCheck() (bool, error) {
	return true, nil
}

func generateRandomHash() (string, error) {
	bytes := make([]byte, 20) // SHA-1 is 20 bytes
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
