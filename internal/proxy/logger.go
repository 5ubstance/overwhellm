package proxy

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
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
	logFile      io.Writer = os.Stdout
)

// SetLogFile sets the log file output
func SetLogFile(path string) {
	if path == "" {
		logFile = os.Stdout
		return
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to open log file %s: %v\n", path, err)
		logFile = os.Stdout
		return
	}
	logFile = file
}

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
		timestamp := time.Now().Format(time.RFC3339)
		msg := fmt.Sprintf("[%s] [TRACE] "+format+"\n", append([]interface{}{timestamp}, args...)...)
		mu.Lock()
		fmt.Print(msg)
		logFile.Write([]byte(msg))
		mu.Unlock()
	}
}

// debug prints debug-level messages
func debug(format string, args ...interface{}) {
	if shouldLog(LevelDebug) {
		timestamp := time.Now().Format(time.RFC3339)
		msg := fmt.Sprintf("[%s] [DEBUG] "+format+"\n", append([]interface{}{timestamp}, args...)...)
		mu.Lock()
		fmt.Print(msg)
		logFile.Write([]byte(msg))
		mu.Unlock()
	}
}

// info prints info-level messages
func info(format string, args ...interface{}) {
	if shouldLog(LevelInfo) {
		timestamp := time.Now().Format(time.RFC3339)
		msg := fmt.Sprintf("[%s] [INFO] "+format+"\n", append([]interface{}{timestamp}, args...)...)
		mu.Lock()
		fmt.Print(msg)
		logFile.Write([]byte(msg))
		mu.Unlock()
	}
}

// warn prints warning-level messages
func warn(format string, args ...interface{}) {
	if shouldLog(LevelWarn) {
		timestamp := time.Now().Format(time.RFC3339)
		msg := fmt.Sprintf("[%s] [WARN] "+format+"\n", append([]interface{}{timestamp}, args...)...)
		mu.Lock()
		fmt.Fprint(os.Stderr, msg)
		logFile.Write([]byte(msg))
		mu.Unlock()
	}
}

// error prints error-level messages
func errorf(format string, args ...interface{}) {
	if shouldLog(LevelError) {
		timestamp := time.Now().Format(time.RFC3339)
		msg := fmt.Sprintf("[%s] [ERROR] "+format+"\n", append([]interface{}{timestamp}, args...)...)
		mu.Lock()
		fmt.Fprint(os.Stderr, msg)
		logFile.Write([]byte(msg))
		mu.Unlock()
	}
}

// critical prints critical-level messages
func critical(format string, args ...interface{}) {
	if shouldLog(LevelCritical) {
		timestamp := time.Now().Format(time.RFC3339)
		msg := fmt.Sprintf("[%s] [CRITICAL] "+format+"\n", append([]interface{}{timestamp}, args...)...)
		mu.Lock()
		fmt.Fprint(os.Stderr, msg)
		logFile.Write([]byte(msg))
		mu.Unlock()
	}
}
