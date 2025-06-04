package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureOutput(f func()) string {
	// Backup the original stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run function that logs
	f()

	// Close writer, restore stdout, read buffer
	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestStdoutLogger_Info(t *testing.T) {
	logger := &StdoutLogger{}
	output := captureOutput(func() {
		logger.Info("hello %s", "world")
	})

	if !strings.Contains(output, "[INFO] hello world") {
		t.Errorf("expected INFO log, got: %s", output)
	}
}

func TestStdoutLogger_Warn(t *testing.T) {
	logger := &StdoutLogger{}
	output := captureOutput(func() {
		logger.Warn("watch out")
	})

	if !strings.Contains(output, "[WARN] watch out") {
		t.Errorf("expected WARN log, got: %s", output)
	}
}

func TestStdoutLogger_Error(t *testing.T) {
	logger := &StdoutLogger{}
	output := captureOutput(func() {
		logger.Error("something went %s", "wrong")
	})

	if !strings.Contains(output, "[ERROR] something went wrong") {
		t.Errorf("expected ERROR log, got: %s", output)
	}
}

func TestStdoutLogger_Debug(t *testing.T) {
	logger := &StdoutLogger{}
	output := captureOutput(func() {
		logger.Debug("debug message %d", 42)
	})

	if !strings.Contains(output, "[DEBUG] debug message 42") {
		t.Errorf("expected DEBUG log, got: %s", output)
	}
}
