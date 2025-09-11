package utils

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/anacrolix/torrent"
)

// ConvertMagnetToTorrent fetches torrent metadata from a magnet link with a specified timeout.
func ConvertMagnetToTorrent(magnetURI string, timeout time.Duration, dataPath string, logger *Logger) ([]byte, error) {
	cfg := torrent.NewDefaultClientConfig()
	cfg.NoUpload = true // We are only interested in metadata
	cfg.DisablePEX = true
	cfg.DataDir = dataPath

	client, err := torrent.NewClient(cfg)
	if err != nil {
		logger.Error("Error creating torrent client:", err)
		return nil, fmt.Errorf("error creating torrent client: %w", err)
	}
	defer client.Close()

	t, err := client.AddMagnet(magnetURI)
	if err != nil {
		logger.Error("Error adding magnet:", err)
		return nil, fmt.Errorf("error adding magnet: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger.Info("Fetching metadata for magnet link...")

	select {
	case <-t.GotInfo():
		// Metadata was fetched successfully
		logger.Info("Successfully fetched metadata from magnet.")
		mi := t.Metainfo()
		var buf bytes.Buffer
		err := mi.Write(&buf)
		if err != nil {
			logger.Error("Failed to write bencoded metainfo:", err)
			return nil, fmt.Errorf("failed to write bencoded metainfo: %w", err)
		}
		return buf.Bytes(), nil
	case <-ctx.Done():
		// Timeout was reached
		logger.Warn("Timeout reached while fetching metadata for magnet.")
		return nil, fmt.Errorf("timeout reached while fetching metadata for magnet")
	}
}
