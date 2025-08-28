package utils

import (
	"log"
	"os"
)

type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	fatalLogger *log.Logger
}

func NewLogger(debug bool) *Logger {
	infoLogger := log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger := log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	fatalLogger := log.New(os.Stderr, "FATAL: ", log.Ldate|log.Ltime|log.Lshortfile)

	return &Logger{
		infoLogger:  infoLogger,
		errorLogger: errorLogger,
		fatalLogger: fatalLogger,
	}
}

func (l *Logger) Info(v ...interface{}) {
	l.infoLogger.Println(v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.errorLogger.Println(v...)
}

func (l *Logger) Fatal(v ...interface{}) {
	l.fatalLogger.Fatalln(v...)
}
