package utils

import (
	"io"
	"log"
	"os"
)

// Logger provides a structured logger.
type Logger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	err   *log.Logger
	fatal *log.Logger
}

// NewLogger creates a new logger instance.
func NewLogger(debug bool) *Logger {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var debugHandle io.Writer = io.Discard
	if debug {
		debugHandle = os.Stdout
	}
	return &Logger{
		debug: log.New(debugHandle, "[DEBUG] ", log.LstdFlags|log.Lshortfile),
		info:  log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lshortfile),
		warn:  log.New(os.Stdout, "[WARN] ", log.LstdFlags|log.Lshortfile),
		err:   log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lshortfile),
		fatal: log.New(os.Stderr, "[FATAL] ", log.LstdFlags|log.Lshortfile),
	}
}

// NewFilterLogger creates a dedicated logger that writes to filter.log
func NewFilterLogger(dataPath string) (*log.Logger, error) {
	logFilePath := dataPath + "/filter.log"
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}

	return log.New(file, "", log.LstdFlags), nil
}

// Debug logs a debug message.
func (l *Logger) Debug(v ...interface{}) {
	l.debug.Println(v...)
}

// Info logs an info message.
func (l *Logger) Info(v ...interface{}) {
	l.info.Println(v...)
}

// Warn logs a warning message.
func (l *Logger) Warn(v ...interface{}) {
	l.warn.Println(v...)
}

// Error logs an error message.
func (l *Logger) Error(v ...interface{}) {
	l.err.Println(v...)
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(v ...interface{}) {
	l.fatal.Fatalln(v...)
}
