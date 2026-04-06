package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(true, &buf)

	logger.Info("hello", "world")

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output, got empty")
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &entry); err != nil {
		t.Fatalf("expected valid JSON log, got: %s", output)
	}

	if entry.Level != "info" {
		t.Errorf("expected level 'info', got %q", entry.Level)
	}
	if entry.Message != "hello world" {
		t.Errorf("expected message 'hello world', got %q", entry.Message)
	}
	if entry.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestLogger_DebugDisabled(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(false, &buf)

	logger.Debug("should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output when debug disabled, got: %s", buf.String())
	}
}

func TestLogger_DebugEnabled(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(true, &buf)

	logger.Debug("debug message")

	if buf.Len() == 0 {
		t.Error("expected debug output when enabled")
	}
}

func TestLogger_AllLevels(t *testing.T) {
	levels := []struct {
		name string
		fn   func(*Logger, ...interface{})
	}{
		{"info", func(l *Logger, v ...interface{}) { l.Info(v...) }},
		{"warn", func(l *Logger, v ...interface{}) { l.Warn(v...) }},
	}

	for _, tt := range levels {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(true, &buf)
			tt.fn(logger, "test message")

			var entry LogEntry
			if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &entry); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if entry.Level != tt.name {
				t.Errorf("expected level %q, got %q", tt.name, entry.Level)
			}
		})
	}
}

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name string
		args []interface{}
		want string
	}{
		{"empty", nil, ""},
		{"single string", []interface{}{"hello"}, "hello"},
		{"multiple strings", []interface{}{"hello", "world"}, "hello world"},
		{"mixed types", []interface{}{"count:", 42}, "count: 42"},
		{"with error", []interface{}{"err:", fmt.Errorf("boom")}, "err: boom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMessage(tt.args...)
			if got != tt.want {
				t.Errorf("formatMessage(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestNewFilterLogger(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewFilterLogger(dir)
	if err != nil {
		t.Fatalf("NewFilterLogger failed: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	logger.Println("test entry")
}
