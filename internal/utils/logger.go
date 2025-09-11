// internal/utils/logger.go
package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Logger provides a structured logger.
type Logger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	err   *log.Logger
	fatal *log.Logger
	out   io.Writer
}

// LogEntry defines the structure for a JSON log entry.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// NewLogger creates a new logger instance for structured JSON logging.
func NewLogger(debug bool, out io.Writer) *Logger {
	var debugHandle io.Writer = io.Discard
	if debug {
		debugHandle = out
	}
	return &Logger{
		debug: log.New(debugHandle, "", 0),
		info:  log.New(out, "", 0),
		warn:  log.New(out, "", 0),
		err:   log.New(os.Stderr, "", 0), // Keep errors in stderr for visibility
		fatal: log.New(os.Stderr, "", 0),
		out:   out,
	}
}

// writeJSONLog creates and writes a JSON log entry.
func (l *Logger) writeJSONLog(logger *log.Logger, level string, v ...interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   formatMessage(v...),
	}
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to plain text if JSON marshaling fails
		logger.Printf("Error marshaling log entry: %v. Original message: %v", err, v)
		return
	}
	logger.Println(string(jsonData))
}

// formatMessage converts a slice of interface{} to a single string.
func formatMessage(v ...interface{}) string {
	if len(v) == 0 {
		return ""
	}
	s := make([]string, len(v))
	for i, val := range v {
		s[i] = formatInterface(val)
	}
	return strings.Join(s, " ")
}

// formatInterface handles different types for logging.
func formatInterface(val interface{}) string {
	switch t := val.(type) {
	case string:
		return t
	case error:
		return t.Error()
	default:
		return fmt.Sprintf("%v", t)
	}
}

// NewFilterLogger creates a dedicated logger that writes to filter.log with a size limit.
func NewFilterLogger(dataPath string) (*log.Logger, error) {
	const maxLogSize = 5 * 1024 * 1024 // 5 MB
	logFilePath := filepath.Join(dataPath, "filter.log")

	fileInfo, err := os.Stat(logFilePath)
	openFlags := os.O_APPEND | os.O_CREATE | os.O_WRONLY

	// If the file exists and is larger than the max size, truncate it.
	if err == nil && fileInfo.Size() > maxLogSize {
		openFlags = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
		log.Printf("INFO: Truncating large filter.log file at %s", logFilePath)
	}

	file, err := os.OpenFile(logFilePath, openFlags, 0666)
	if err != nil {
		return nil, err
	}

	return log.New(file, "", log.LstdFlags), nil
}

// Debug logs a debug message.
func (l *Logger) Debug(v ...interface{}) {
	l.writeJSONLog(l.debug, "debug", v...)
}

// Info logs an info message.
func (l *Logger) Info(v ...interface{}) {
	l.writeJSONLog(l.info, "info", v...)
}

// Warn logs a warning message.
func (l *Logger) Warn(v ...interface{}) {
	l.writeJSONLog(l.warn, "warn", v...)
}

// Error logs an error message.
func (l *Logger) Error(v ...interface{}) {
	l.writeJSONLog(l.err, "error", v...)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(v ...interface{}) {
	l.writeJSONLog(l.fatal, "fatal", v...)
	os.Exit(1)
}
