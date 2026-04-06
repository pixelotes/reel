package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ConvertMagnetToTorrent invokes the external magnet2torrent helper binary
// to fetch torrent metadata from a magnet link.
func ConvertMagnetToTorrent(magnetURI string, timeout time.Duration, dataPath string, logger *Logger) ([]byte, error) {
	binPath, err := findMagnet2TorrentBinary()
	if err != nil {
		return nil, fmt.Errorf("magnet2torrent binary not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logger.Info("Invoking magnet2torrent helper:", binPath)

	cmd := exec.CommandContext(ctx, binPath, magnetURI, dataPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout reached while fetching metadata for magnet")
		}
		return nil, fmt.Errorf("magnet2torrent failed: %s (%w)", errMsg, err)
	}

	logger.Info("Successfully fetched metadata from magnet.")
	return stdout.Bytes(), nil
}

// findMagnet2TorrentBinary looks for the magnet2torrent binary next to the
// main executable, then falls back to $PATH.
func findMagnet2TorrentBinary() (string, error) {
	// 1. Check next to the current executable
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "magnet2torrent")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 2. Check $PATH
	path, err := exec.LookPath("magnet2torrent")
	if err == nil {
		return path, nil
	}

	return "", fmt.Errorf("magnet2torrent not found next to executable or in $PATH")
}
