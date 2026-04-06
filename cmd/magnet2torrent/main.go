// magnet2torrent is a helper binary that converts magnet URIs to .torrent files.
// It is invoked by the main reel binary when magnet-to-torrent conversion is enabled.
// Keeping this as a separate binary avoids pulling ~60 heavy dependencies (anacrolix/torrent,
// pion/webrtc, etc.) into the main binary.
//
// Usage: magnet2torrent <magnet-uri> [data-path]
// Output: writes raw .torrent bytes to stdout
// Exit codes: 0 = success, 1 = error (message on stderr)
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/anacrolix/torrent"
)

const defaultTimeout = 60 * time.Second

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: magnet2torrent <magnet-uri> [data-path]")
		os.Exit(1)
	}

	magnetURI := os.Args[1]
	dataPath := os.TempDir()
	if len(os.Args) >= 3 {
		dataPath = os.Args[2]
	}

	torrentBytes, err := convert(magnetURI, dataPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Stdout.Write(torrentBytes)
}

func convert(magnetURI, dataPath string) ([]byte, error) {
	cfg := torrent.NewDefaultClientConfig()
	cfg.NoUpload = true
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

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	select {
	case <-t.GotInfo():
		mi := t.Metainfo()
		var buf bytes.Buffer
		if err := mi.Write(&buf); err != nil {
			return nil, fmt.Errorf("failed to write metainfo: %w", err)
		}
		return buf.Bytes(), nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout fetching metadata")
	}
}
