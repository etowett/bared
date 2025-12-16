package util

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
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
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	var buf bytes.Buffer
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
	logger := &Logger{
		slog:  slog.New(handler),
		level: levelVar,
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
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	var buf bytes.Buffer
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
	logger := &Logger{
		slog:  slog.New(handler),
		level: levelVar,
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
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	handler := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: levelVar})
	logger := &Logger{
		slog:  slog.New(handler),
		level: levelVar,
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

	// Manually create logger with custom output for testing
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelDebug)
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
	globalLogger = &Logger{
		slog:  slog.New(handler),
		level: levelVar,
	}

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

func TestStructuredLogging(t *testing.T) {
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	var buf bytes.Buffer
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelDebug)

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
	logger := &Logger{
		slog:  slog.New(handler),
		level: levelVar,
	}

	// Test structured logging with key-value pairs
	logger.InfoS("user action", "user", "john", "action", "login", "ip", "192.168.1.1")

	output := buf.String()
	// Verify key-value pairs are present
	if !strings.Contains(output, "user") || !strings.Contains(output, "john") {
		t.Error("Structured logging missing user field")
	}
	if !strings.Contains(output, "action") || !strings.Contains(output, "login") {
		t.Error("Structured logging missing action field")
	}
	if !strings.Contains(output, "ip") || !strings.Contains(output, "192.168.1.1") {
		t.Error("Structured logging missing ip field")
	}
}

func TestStructuredLoggingWithContext(t *testing.T) {
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	var buf bytes.Buffer
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
	logger := &Logger{
		slog:  slog.New(handler),
		level: levelVar,
	}

	ctx := context.Background()
	logger.InfoCtx(ctx, "operation completed", "duration", 123, "status", "success")

	output := buf.String()
	if !strings.Contains(output, "operation completed") {
		t.Error("Context logging missing message")
	}
	if !strings.Contains(output, "duration") || !strings.Contains(output, "123") {
		t.Error("Context logging missing duration field")
	}
}

func TestLogHooksWithSlog(t *testing.T) {
	// Reset global state
	globalLogger = nil
	globalHook = nil
	once = sync.Once{}

	var capturedLogs []string
	SetLogHook(func(level LogLevel, message string) {
		capturedLogs = append(capturedLogs, message)
	})

	var buf bytes.Buffer
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
	hooked := &hookHandler{
		handler: baseHandler,
		hookMu:  &hookMu,
		getHook: func() LogHook {
			hookMu.RLock()
			defer hookMu.RUnlock()
			return globalHook
		},
	}

	globalLogger = &Logger{
		slog:  slog.New(hooked),
		level: levelVar,
	}

	// Test printf-style logging with hook
	Info("test message %d", 42)

	if len(capturedLogs) != 1 {
		t.Errorf("Expected 1 captured log, got %d", len(capturedLogs))
	}
	if !strings.Contains(capturedLogs[0], "test message") {
		t.Errorf("Captured log doesn't contain message: %s", capturedLogs[0])
	}

	// Test structured logging with hook
	capturedLogs = nil
	GetLogger().InfoS("structured test", "key", "value")

	if len(capturedLogs) != 1 {
		t.Errorf("Expected 1 captured log for structured logging, got %d", len(capturedLogs))
	}
	if !strings.Contains(capturedLogs[0], "structured test") {
		t.Errorf("Captured structured log doesn't contain message: %s", capturedLogs[0])
	}
	if !strings.Contains(capturedLogs[0], "key=value") {
		t.Errorf("Captured structured log doesn't contain attributes: %s", capturedLogs[0])
	}

	// Clean up
	SetLogHook(nil)
}

func TestJSONvsTextOutput(t *testing.T) {
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	// Test JSON output
	t.Run("JSON output", func(t *testing.T) {
		var buf bytes.Buffer
		levelVar := new(slog.LevelVar)
		levelVar.Set(slog.LevelInfo)

		handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: levelVar})
		logger := &Logger{
			slog:  slog.New(handler),
			level: levelVar,
		}

		logger.InfoS("test message", "key", "value", "number", 42)

		output := buf.String()
		// Verify it's valid JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Errorf("JSON output is not valid JSON: %v", err)
		}

		// Verify fields
		if parsed["msg"] != "test message" {
			t.Errorf("JSON message = %v, want 'test message'", parsed["msg"])
		}
		if parsed["key"] != "value" {
			t.Errorf("JSON key = %v, want 'value'", parsed["key"])
		}
		if parsed["number"] != float64(42) {
			t.Errorf("JSON number = %v, want 42", parsed["number"])
		}
	})

	// Test text output
	t.Run("Text output", func(t *testing.T) {
		var buf bytes.Buffer
		levelVar := new(slog.LevelVar)
		levelVar.Set(slog.LevelInfo)

		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
		logger := &Logger{
			slog:  slog.New(handler),
			level: levelVar,
		}

		logger.InfoS("test message", "key", "value")

		output := buf.String()
		// Verify it's human-readable text
		if !strings.Contains(output, "test message") {
			t.Error("Text output doesn't contain message")
		}
		if !strings.Contains(output, "key") {
			t.Error("Text output doesn't contain key")
		}
	})
}

func TestEnvironmentDetection(t *testing.T) {
	// Save original env vars
	origEnv := os.Getenv("BARED_ENV")
	origFormat := os.Getenv("BARED_LOG_FORMAT")
	defer func() {
		os.Setenv("BARED_ENV", origEnv)
		os.Setenv("BARED_LOG_FORMAT", origFormat)
	}()

	tests := []struct {
		name     string
		env      string
		format   string
		expected string
	}{
		{"BARED_ENV production", "production", "", "production"},
		{"BARED_ENV development", "development", "", "development"},
		{"BARED_LOG_FORMAT json", "", "json", "production"},
		{"BARED_LOG_FORMAT text", "", "text", "development"},
		{"ENV production", "", "", "production"}, // Will be tested via ENV var
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("BARED_ENV", tt.env)
			os.Setenv("BARED_LOG_FORMAT", tt.format)

			result := detectEnvironment()
			if result != tt.expected {
				t.Errorf("detectEnvironment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSensitiveDataRedaction(t *testing.T) {
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	var buf bytes.Buffer
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	opts := &slog.HandlerOptions{
		Level: levelVar,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			key := strings.ToLower(a.Key)
			if key == "password" || key == "secret" || key == "token" {
				return slog.Attr{Key: a.Key, Value: slog.StringValue("***REDACTED***")}
			}
			return a
		},
	}

	handler := slog.NewTextHandler(&buf, opts)
	logger := &Logger{
		slog:  slog.New(handler),
		level: levelVar,
	}

	// Test that sensitive fields are redacted
	logger.InfoS("database connection", "host", "localhost", "password", "secretpass123")

	output := buf.String()
	if strings.Contains(output, "secretpass123") {
		t.Error("Password was not redacted in logs")
	}
	if !strings.Contains(output, "***REDACTED***") {
		t.Error("Redaction marker not found in logs")
	}
	if !strings.Contains(output, "localhost") {
		t.Error("Non-sensitive data was incorrectly redacted")
	}
}

func TestWithMethods(t *testing.T) {
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	var buf bytes.Buffer
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
	logger := &Logger{
		slog:  slog.New(handler),
		level: levelVar,
	}

	// Test With method
	loggerWithAttrs := logger.With("request_id", "12345", "user", "alice")
	loggerWithAttrs.InfoS("operation completed")

	output := buf.String()
	if !strings.Contains(output, "request_id") || !strings.Contains(output, "12345") {
		t.Error("With() attributes not included in log")
	}
	if !strings.Contains(output, "user") || !strings.Contains(output, "alice") {
		t.Error("With() user attribute not included in log")
	}

	// Test WithGroup method
	buf.Reset()
	loggerWithGroup := logger.WithGroup("http")
	loggerWithGroup.InfoS("request", "method", "GET", "path", "/api/v1")

	output = buf.String()
	// Group should be reflected in the output
	if !strings.Contains(output, "method") || !strings.Contains(output, "GET") {
		t.Error("WithGroup() attributes not included in log")
	}
}

func TestDynamicLevelChange(t *testing.T) {
	// Reset global state
	globalLogger = nil
	once = sync.Once{}

	var buf bytes.Buffer
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.LevelInfo)

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: levelVar})
	logger := &Logger{
		slog:  slog.New(handler),
		level: levelVar,
	}

	// Debug should not appear at INFO level
	logger.DebugS("debug message")
	if strings.Contains(buf.String(), "debug message") {
		t.Error("Debug message appeared at INFO level")
	}

	// Change to DEBUG level dynamically
	buf.Reset()
	logger.SetLevel(DEBUG)
	logger.DebugS("debug message now visible")

	if !strings.Contains(buf.String(), "debug message now visible") {
		t.Error("Debug message did not appear after level change to DEBUG")
	}
}
