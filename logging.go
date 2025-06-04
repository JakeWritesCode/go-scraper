package main

import (
	"fmt"
	"time"
)

type LogLevel string

const (
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
	LevelDebug LogLevel = "DEBUG"
)

type Logger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// StdoutLogger is a simple logger that writes logs to standard output.
type StdoutLogger struct{}

// log is a helper function that formats the log message with a timestamp and log level.
func (l *StdoutLogger) log(level LogLevel, msg string, args ...interface{}) {
	timestamp := time.Now().Format(time.RFC3339)
	fmt.Printf("%s [%s] %s \n", timestamp, level, fmt.Sprintf(msg, args...))
}

// Info, Warn, Error, and Debug methods implement the Logger interface for StdoutLogger.
func (l *StdoutLogger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

func (l *StdoutLogger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

func (l *StdoutLogger) Error(msg string, args ...interface{}) {
	l.log(LevelError, msg, args...)
}

func (l *StdoutLogger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}
