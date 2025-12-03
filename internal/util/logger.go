package util

import (
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel represents logging levels
type LogLevel int

const (
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
)

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

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.GetLevel() <= DEBUG {
		l.logger.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	if l.GetLevel() <= INFO {
		l.logger.Printf(format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.GetLevel() <= WARN {
		l.logger.Printf("[WARN] "+format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	if l.GetLevel() <= ERROR {
		l.logger.Printf("[ERROR] "+format, v...)
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
