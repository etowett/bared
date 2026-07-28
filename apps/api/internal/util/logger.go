// Package util provides utility functions including logging and command execution.
package util

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// LogLevel represents logging levels
type LogLevel int

// Log levels for controlling logging verbosity.
const (
	// DEBUG enables detailed debug logging.
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger provides leveled logging backed by slog
type Logger struct {
	slog  *slog.Logger
	level *slog.LevelVar
	mu    sync.RWMutex
}

var (
	// globalLogger is atomic because GetLogger is called from every goroutine in
	// the daemon — job workers, the scheduler, the API server — and it lazily
	// initialises the logger on first use. A plain pointer made that first-use
	// check a data race: one goroutine reading the nil guard while another wrote
	// the pointer inside once.Do. sync.Once orders the callers of Do, not a read
	// that happens before Do is ever reached. See #117.
	globalLogger atomic.Pointer[Logger]
	once         sync.Once
	globalHook   LogHook
	hookMu       sync.RWMutex
)

// LogHook is a function that intercepts log messages
type LogHook func(level LogLevel, message string)

// hookHandler wraps slog.Handler to intercept log records for hooks
type hookHandler struct {
	handler slog.Handler
	hookMu  *sync.RWMutex
	getHook func() LogHook
}

// Enabled implements slog.Handler
func (h *hookHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler
func (h *hookHandler) Handle(ctx context.Context, record slog.Record) error {
	// Call the hook before passing to underlying handler
	h.hookMu.RLock()
	hook := h.getHook()
	h.hookMu.RUnlock()

	if hook != nil {
		// Convert slog.Level to LogLevel
		var logLevel LogLevel
		switch {
		case record.Level < slog.LevelInfo:
			logLevel = DEBUG
		case record.Level < slog.LevelWarn:
			logLevel = INFO
		case record.Level < slog.LevelError:
			logLevel = WARN
		default:
			logLevel = ERROR
		}

		// Build message with attributes for the hook
		message := buildMessageWithAttrs(record)
		hook(logLevel, message)
	}

	return h.handler.Handle(ctx, record)
}

// WithAttrs implements slog.Handler
func (h *hookHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &hookHandler{
		handler: h.handler.WithAttrs(attrs),
		hookMu:  h.hookMu,
		getHook: h.getHook,
	}
}

// WithGroup implements slog.Handler
func (h *hookHandler) WithGroup(name string) slog.Handler {
	return &hookHandler{
		handler: h.handler.WithGroup(name),
		hookMu:  h.hookMu,
		getHook: h.getHook,
	}
}

// buildMessageWithAttrs reconstructs a message with attributes
func buildMessageWithAttrs(record slog.Record) string {
	var buf strings.Builder

	// Add level prefix for compatibility with old format
	switch {
	case record.Level < slog.LevelInfo:
		buf.WriteString("[DEBUG] ")
	case record.Level < slog.LevelWarn:
		// INFO logs don't have prefix in old format
	case record.Level < slog.LevelError:
		buf.WriteString("[WARN] ")
	default:
		buf.WriteString("[ERROR] ")
	}

	buf.WriteString(record.Message)

	// Append attributes in a readable format
	if record.NumAttrs() > 0 {
		buf.WriteString(" {")
		first := true
		record.Attrs(func(attr slog.Attr) bool {
			if !first {
				buf.WriteString(", ")
			}
			first = false
			fmt.Fprintf(&buf, "%s=%v", attr.Key, attr.Value)
			return true
		})
		buf.WriteString("}")
	}

	return buf.String()
}

// detectEnvironment determines if we're running in production
func detectEnvironment() string {
	// Check environment variables
	if env := os.Getenv("BARED_ENV"); env != "" {
		return env
	}
	if env := os.Getenv("ENV"); env != "" {
		return env
	}

	// Check for explicit format override
	if format := os.Getenv("BARED_LOG_FORMAT"); format != "" {
		if format == "json" {
			return "production"
		}
		if format == "text" {
			return "development"
		}
	}

	// Default to development for interactive terminals
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		// Cannot determine if stdout is a terminal, default to production
		return "production"
	}

	if (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		return "development"
	}

	return "production"
}

// LoggerOptions contains optional configuration for logger initialization
type LoggerOptions struct {
	AddSource  bool
	TimeFormat string
}

// createHandlerWithOptions creates a handler with custom options
func createHandlerWithOptions(w io.Writer, level *slog.LevelVar, env string, logOpts *LoggerOptions) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
		// ReplaceAttr for sensitive data redaction
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Redact sensitive keys
			key := strings.ToLower(a.Key)
			if key == "password" || key == "secret" || key == "token" ||
				key == "secret_access_key" || key == "access_key" {
				return slog.Attr{Key: a.Key, Value: slog.StringValue("***REDACTED***")}
			}
			return a
		},
	}

	// Apply custom options if provided
	if logOpts != nil {
		opts.AddSource = logOpts.AddSource
	}

	if env == "production" {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// determineEnvironment determines environment from format string or auto-detection
func determineEnvironment(logFormat string) string {
	// Check explicit format first
	if logFormat != "" && logFormat != "auto" {
		if logFormat == "json" {
			return "production"
		}
		if logFormat == "text" {
			return "development"
		}
	}

	// Fall back to auto-detection
	return detectEnvironment()
}

// convertLogLevel converts our LogLevel to slog.Level
func convertLogLevel(level LogLevel) slog.Level {
	switch level {
	case DEBUG:
		return slog.LevelDebug
	case INFO:
		return slog.LevelInfo
	case WARN:
		return slog.LevelWarn
	case ERROR:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// InitLogger initializes the global logger with the specified level
func InitLogger(level LogLevel) {
	InitLoggerWithOptions(level, "", nil)
}

// InitLoggerWithOptions initializes the global logger with level, format, and options
func InitLoggerWithOptions(level LogLevel, logFormat string, logOpts *LoggerOptions) {
	once.Do(func() {
		env := determineEnvironment(logFormat)
		levelVar := new(slog.LevelVar)
		levelVar.Set(convertLogLevel(level))

		baseHandler := createHandlerWithOptions(os.Stdout, levelVar, env, logOpts)
		hooked := &hookHandler{
			handler: baseHandler,
			hookMu:  &hookMu,
			getHook: func() LogHook {
				hookMu.RLock()
				defer hookMu.RUnlock()
				return globalHook
			},
		}

		logger := &Logger{
			slog:  slog.New(hooked),
			level: levelVar,
		}
		globalLogger.Store(logger)

		// Redirect standard library log to slog to prevent duplicate output
		slog.SetDefault(logger.slog)
		log.SetOutput(io.Discard)
		log.SetFlags(0)
	})

	// Allow re-initialization of level
	if logger := globalLogger.Load(); logger != nil {
		logger.SetLevel(level)
	}
}

// GetLogger returns the global logger instance, initializing it at INFO if
// nothing has initialized it yet.
func GetLogger() *Logger {
	if logger := globalLogger.Load(); logger != nil {
		return logger
	}
	InitLogger(INFO) // Default to INFO level
	return globalLogger.Load()
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level.Set(convertLogLevel(level))
}

// GetLevel returns the current logging level
func (l *Logger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()

	switch l.level.Level() {
	case slog.LevelDebug:
		return DEBUG
	case slog.LevelWarn:
		return WARN
	case slog.LevelError:
		return ERROR
	default:
		return INFO
	}
}

// Backward-compatible printf-style methods
// These maintain compatibility with existing code

// Debug logs a debug message using printf-style formatting
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.slog.Enabled(context.Background(), slog.LevelDebug) {
		msg := fmt.Sprintf(format, v...)
		l.slog.Debug(msg)
	}
}

// Info logs an info message using printf-style formatting
func (l *Logger) Info(format string, v ...interface{}) {
	if l.slog.Enabled(context.Background(), slog.LevelInfo) {
		msg := fmt.Sprintf(format, v...)
		l.slog.Info(msg)
	}
}

// Warn logs a warning message using printf-style formatting
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.slog.Enabled(context.Background(), slog.LevelWarn) {
		msg := fmt.Sprintf(format, v...)
		l.slog.Warn(msg)
	}
}

// Error logs an error message using printf-style formatting
func (l *Logger) Error(format string, v ...interface{}) {
	if l.slog.Enabled(context.Background(), slog.LevelError) {
		msg := fmt.Sprintf(format, v...)
		l.slog.Error(msg)
	}
}

// New structured logging methods

// DebugS logs a structured debug message with key-value pairs
func (l *Logger) DebugS(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}

// InfoS logs a structured info message with key-value pairs
func (l *Logger) InfoS(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// WarnS logs a structured warning message with key-value pairs
func (l *Logger) WarnS(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

// ErrorS logs a structured error message with key-value pairs
func (l *Logger) ErrorS(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

// Context-aware structured logging methods

// DebugCtx logs a structured debug message with context
func (l *Logger) DebugCtx(ctx context.Context, msg string, args ...any) {
	l.slog.DebugContext(ctx, msg, args...)
}

// InfoCtx logs a structured info message with context
func (l *Logger) InfoCtx(ctx context.Context, msg string, args ...any) {
	l.slog.InfoContext(ctx, msg, args...)
}

// WarnCtx logs a structured warning message with context
func (l *Logger) WarnCtx(ctx context.Context, msg string, args ...any) {
	l.slog.WarnContext(ctx, msg, args...)
}

// ErrorCtx logs a structured error message with context
func (l *Logger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	l.slog.ErrorContext(ctx, msg, args...)
}

// With returns a new logger with the given attributes added to all logs
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		slog:  l.slog.With(args...),
		level: l.level,
	}
}

// WithGroup returns a new logger with the given group name
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		slog:  l.slog.WithGroup(name),
		level: l.level,
	}
}

// ParseLogLevel parses a string into a LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return DEBUG
	case "info", "":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO // Default to INFO for unknown levels
	}
}

// Global convenience functions for backward compatibility

// Debug logs a debug message using the global logger
func Debug(format string, v ...interface{}) {
	GetLogger().Debug(format, v...)
}

// Info logs an info message using the global logger
func Info(format string, v ...interface{}) {
	GetLogger().Info(format, v...)
}

// Warn logs a warning message using the global logger
func Warn(format string, v ...interface{}) {
	GetLogger().Warn(format, v...)
}

// Error logs an error message using the global logger
func Error(format string, v ...interface{}) {
	GetLogger().Error(format, v...)
}

// SetLogHook sets a hook function to intercept log messages
func SetLogHook(hook LogHook) {
	hookMu.Lock()
	defer hookMu.Unlock()
	globalHook = hook
}

// GetLogHook returns the current log hook
func GetLogHook() LogHook {
	hookMu.RLock()
	defer hookMu.RUnlock()
	return globalHook
}
