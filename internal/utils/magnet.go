package utils

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/anacrolix/torrent"
)

// ConvertMagnetToTorrent fetches torrent metadata from a magnet link with a specified timeout.
func ConvertMagnetToTorrent(magnetURI string, timeout time.Duration, dataPath string) ([]byte, error) {
	cfg := torrent.NewDefaultClientConfig()
	cfg.NoUpload = true // We are only interested in metadata
	cfg.DisablePEX = true
	cfg.DataDir = dataPath

	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating torrent client: %w", err)
	}
	defer client.Close()

	t, err := client.AddMagnet(magnetURI)
	if err != nil {
		return nil, fmt.Errorf("error adding magnet: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-t.GotInfo():
		// Metadata was fetched successfully
		mi := t.Metainfo()
		var buf bytes.Buffer
		err := mi.Write(&buf)
		if err != nil {
			return nil, fmt.Errorf("failed to write bencoded metainfo: %w", err)
		}
		return buf.Bytes(), nil
	case <-ctx.Done():
		// Timeout was reached
		return nil, fmt.Errorf("timeout reached while fetching metadata for magnet")
	}
}
