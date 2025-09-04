package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// ConvertSRTToVTT converts SRT subtitle content to WebVTT format
func ConvertSRTToVTT(filePath string) (io.ReadSeeker, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open subtitle file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string

	// Read all lines
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read subtitle file: %w", err)
	}

	// Convert SRT to VTT
	vttContent := convertSRTLinesToVTT(lines)

	// Return as ReadSeeker
	reader := strings.NewReader(vttContent)
	return reader, nil
}

func convertSRTLinesToVTT(lines []string) string {
	var result strings.Builder
	result.WriteString("WEBVTT\n\n")

	// Regex to match SRT timestamp format: 00:00:00,000 --> 00:00:00,000
	timestampRegex := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3}) --> (\d{2}:\d{2}:\d{2}),(\d{3})`)

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines
		if line == "" {
			i++
			continue
		}

		// Skip sequence numbers (lines that are just numbers)
		if isSequenceNumber(line) {
			i++
			continue
		}

		// Check if this is a timestamp line
		if timestampRegex.MatchString(line) {
			// Convert SRT timestamp to VTT format (replace comma with dot)
			vttTimestamp := timestampRegex.ReplaceAllString(line, "$1.$2 --> $3.$4")
			result.WriteString(vttTimestamp + "\n")
			i++

			// Add subtitle text lines until we hit an empty line or end
			for i < len(lines) {
				textLine := strings.TrimSpace(lines[i])
				if textLine == "" {
					break
				}
				// Skip if it's the next sequence number
				if isSequenceNumber(textLine) {
					break
				}
				result.WriteString(textLine + "\n")
				i++
			}
			result.WriteString("\n") // Add blank line between cues
		} else {
			i++
		}
	}

	return result.String()
}

func isSequenceNumber(line string) bool {
	// Check if the line is just a number (sequence number in SRT)
	matched, _ := regexp.MatchString(`^\d+$`, strings.TrimSpace(line))
	return matched
}
