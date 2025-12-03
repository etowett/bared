package util

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", DEBUG},
		{"DEBUG", DEBUG},
		{"Debug", DEBUG},
		{" debug ", DEBUG},
		{"info", INFO},
		{"INFO", INFO},
		{"", INFO}, // Empty defaults to INFO
		{"warn", WARN},
		{"warning", WARN},
		{"WARN", WARN},
		{"error", ERROR},
		{"ERROR", ERROR},
		{"invalid", INFO}, // Unknown defaults to INFO
		{"unknown", INFO},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		logger: log.New(&buf, "", 0),
	}

	// DEBUG should not appear when level is INFO
	logger.Debug("debug message")
	if strings.Contains(buf.String(), "debug message") {
		t.Error("DEBUG message appeared when level was INFO")
	}

	// INFO should appear
	buf.Reset()
	logger.Info("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Error("INFO message did not appear when level was INFO")
	}

	// WARN should appear
	buf.Reset()
	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("WARN message did not appear when level was INFO")
	}

	// ERROR should appear
	buf.Reset()
	logger.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Error("ERROR message did not appear when level was INFO")
	}

	// Set to DEBUG and verify DEBUG messages appear
	logger.SetLevel(DEBUG)
	buf.Reset()
	logger.Debug("debug message now visible")
	if !strings.Contains(buf.String(), "debug message now visible") {
		t.Error("DEBUG message did not appear when level was DEBUG")
	}
}

func TestConcurrentLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		logger: log.New(&buf, "", 0),
	}

	var wg sync.WaitGroup
	iterations := 100

	// Test concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			logger.Info("message %d", n)
		}(i)
	}

	wg.Wait()

	// Just verify no race condition occurred (test should pass with -race flag)
	// Count that we got messages (exact count may vary due to buffering)
	messageCount := strings.Count(buf.String(), "message")
	if messageCount == 0 {
		t.Error("No messages logged during concurrent test")
	}
}

func TestSetGetLevel(t *testing.T) {
	logger := &Logger{
		level:  INFO,
		logger: log.New(&bytes.Buffer{}, "", 0),
	}

	// Test initial level
	if logger.GetLevel() != INFO {
		t.Errorf("Initial level = %v, want INFO", logger.GetLevel())
	}

	// Test setting to DEBUG
	logger.SetLevel(DEBUG)
	if logger.GetLevel() != DEBUG {
		t.Errorf("After SetLevel(DEBUG), GetLevel() = %v, want DEBUG", logger.GetLevel())
	}

	// Test setting to ERROR
	logger.SetLevel(ERROR)
	if logger.GetLevel() != ERROR {
		t.Errorf("After SetLevel(ERROR), GetLevel() = %v, want ERROR", logger.GetLevel())
	}
}

func TestGlobalLoggerInit(t *testing.T) {
	// Reset global logger for test isolation
	globalLogger = nil
	once = sync.Once{}

	// First call initializes
	InitLogger(DEBUG)
	if GetLogger().GetLevel() != DEBUG {
		t.Errorf("Global logger level = %v, want DEBUG", GetLogger().GetLevel())
	}

	// Second call should update level
	InitLogger(ERROR)
	if GetLogger().GetLevel() != ERROR {
		t.Errorf("After re-init, global logger level = %v, want ERROR", GetLogger().GetLevel())
	}

	// GetLogger should return same instance
	logger1 := GetLogger()
	logger2 := GetLogger()
	if logger1 != logger2 {
		t.Error("GetLogger() returned different instances")
	}
}

func TestGlobalConvenienceFunctions(t *testing.T) {
	// Reset global logger
	globalLogger = nil
	once = sync.Once{}

	var buf bytes.Buffer
	InitLogger(DEBUG)
	GetLogger().logger = log.New(&buf, "", 0)

	// Test global Debug
	Debug("global debug")
	if !strings.Contains(buf.String(), "global debug") {
		t.Error("Global Debug() did not log message")
	}

	// Test global Info
	buf.Reset()
	Info("global info")
	if !strings.Contains(buf.String(), "global info") {
		t.Error("Global Info() did not log message")
	}

	// Test global Warn
	buf.Reset()
	Warn("global warn")
	if !strings.Contains(buf.String(), "global warn") {
		t.Error("Global Warn() did not log message")
	}

	// Test global Error
	buf.Reset()
	Error("global error")
	if !strings.Contains(buf.String(), "global error") {
		t.Error("Global Error() did not log message")
	}
}
