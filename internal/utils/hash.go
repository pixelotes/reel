package utils

import (
	"encoding/binary"
	"fmt"
	"os"
)

// ComputeMovieHash computes the hash of a media file for OpenSubtitles (OSDb protocol).
// It reads the first 64KB and the last 64KB of the file, adding the file size.
// This allows identifying files without reading the entire content.
func ComputeMovieHash(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}
	fileSize := stat.Size()

	// 64 KB chunk size
	const chunkSize = 65536

	if fileSize < chunkSize {
		return "", fmt.Errorf("file is too small to hash")
	}

	// Buffer for reading 64-bit integers (8 bytes)
	buf := make([]byte, 8)
	var hash uint64 = uint64(fileSize)

	// Read head (first 64KB)
	for i := 0; i < chunkSize/8; i++ {
		if _, err := file.Read(buf); err != nil {
			return "", err
		}
		hash += binary.LittleEndian.Uint64(buf)
	}

	// Read tail (last 64KB)
	// Seek to file end minus 64KB
	if _, err := file.Seek(fileSize-chunkSize, 0); err != nil {
		return "", err
	}

	for i := 0; i < chunkSize/8; i++ {
		if _, err := file.Read(buf); err != nil {
			return "", err
		}
		hash += binary.LittleEndian.Uint64(buf)
	}

	return fmt.Sprintf("%x", hash), nil
}
