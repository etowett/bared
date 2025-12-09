// Package util provides utility functions including logging and command execution.
package util

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
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

// Logger provides leveled logging
type Logger struct {
	level  LogLevel
	logger *log.Logger
	mu     sync.RWMutex
}

var (
	globalLogger *Logger
	once         sync.Once
	globalHook   LogHook
	hookMu       sync.RWMutex
)

// LogHook is a function that intercepts log messages
type LogHook func(level LogLevel, message string)

// InitLogger initializes the global logger with the specified level
func InitLogger(level LogLevel) {
	once.Do(func() {
		globalLogger = &Logger{
			level:  level,
			logger: log.New(os.Stdout, "", log.LstdFlags),
		}
	})
	// Allow re-initialization of level
	if globalLogger != nil {
		globalLogger.SetLevel(level)
	}
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		InitLogger(INFO) // Default to INFO level
	}
	return globalLogger
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current logging level
func (l *Logger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// log is the internal logging method that calls the hook
func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	l.logger.Printf(format, v...)

	// Call hook if set
	hookMu.RLock()
	hook := globalHook
	hookMu.RUnlock()

	if hook != nil {
		msg := fmt.Sprintf(format, v...)
		hook(level, msg)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.GetLevel() <= DEBUG {
		l.log(DEBUG, "[DEBUG] "+format, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	if l.GetLevel() <= INFO {
		l.log(INFO, format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.GetLevel() <= WARN {
		l.log(WARN, "[WARN] "+format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	if l.GetLevel() <= ERROR {
		l.log(ERROR, "[ERROR] "+format, v...)
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

// Global convenience functions

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
