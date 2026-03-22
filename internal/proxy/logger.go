package proxy

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// LogLevel represents log severity
type LogLevel int

const (
	LevelTrace LogLevel = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelCritical
)

var (
	currentLevel = LevelInfo
	mu           sync.Mutex
)

// SetLogLevel sets the current log level
func SetLogLevel(level string) {
	switch strings.ToUpper(level) {
	case "TRACE":
		currentLevel = LevelTrace
	case "DEBUG":
		currentLevel = LevelDebug
	case "INFO":
		currentLevel = LevelInfo
	case "WARN", "WARNING":
		currentLevel = LevelWarn
	case "ERROR":
		currentLevel = LevelError
	case "CRITICAL", "FATAL":
		currentLevel = LevelCritical
	default:
		currentLevel = LevelInfo
	}
}

// GetLogLevel returns the current log level
func GetLogLevel() string {
	switch currentLevel {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelCritical:
		return "CRITICAL"
	default:
		return "INFO"
	}
}

// shouldLog checks if the given level should be logged
func shouldLog(level LogLevel) bool {
	return level >= currentLevel
}

// trace prints trace-level messages
func trace(format string, args ...interface{}) {
	if shouldLog(LevelTrace) {
		fmt.Printf("[TRACE] "+format+"\n", args...)
	}
}

// debug prints debug-level messages
func debug(format string, args ...interface{}) {
	if shouldLog(LevelDebug) {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}

// info prints info-level messages
func info(format string, args ...interface{}) {
	if shouldLog(LevelInfo) {
		fmt.Printf("[INFO] "+format+"\n", args...)
	}
}

// warn prints warning-level messages
func warn(format string, args ...interface{}) {
	if shouldLog(LevelWarn) {
		fmt.Fprintf(os.Stderr, "[WARN] "+format+"\n", args...)
	}
}

// error prints error-level messages
func errorf(format string, args ...interface{}) {
	if shouldLog(LevelError) {
		fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
	}
}

// critical prints critical-level messages
func critical(format string, args ...interface{}) {
	if shouldLog(LevelCritical) {
		fmt.Fprintf(os.Stderr, "[CRITICAL] "+format+"\n", args...)
	}
}
