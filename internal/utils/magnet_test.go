package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindMagnet2TorrentBinary_NotFound(t *testing.T) {
	// With no binary in PATH or next to executable, should return error
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir()) // empty dir as PATH

	_, err := findMagnet2TorrentBinary()
	if err == nil {
		t.Error("expected error when binary not found")
	}

	os.Setenv("PATH", oldPath)
}

func TestFindMagnet2TorrentBinary_InPath(t *testing.T) {
	dir := t.TempDir()

	// Create a fake binary
	fakeBin := filepath.Join(dir, "magnet2torrent")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir)

	path, err := findMagnet2TorrentBinary()
	if err != nil {
		t.Fatalf("expected to find binary in PATH, got: %v", err)
	}
	if path != fakeBin {
		t.Errorf("expected %q, got %q", fakeBin, path)
	}

	os.Setenv("PATH", oldPath)
}

func TestConvertMagnetToTorrent_BinaryNotFound(t *testing.T) {
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())

	logger := NewLogger(false, os.Stderr)
	_, err := ConvertMagnetToTorrent("magnet:?xt=urn:btih:abc123", 5, t.TempDir(), logger)
	if err == nil {
		t.Error("expected error when binary not found")
	}

	os.Setenv("PATH", oldPath)
}
