package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Level represents log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	currentLevel = LevelInfo
	mu           sync.RWMutex
)

// SetLevel sets the minimum log level
func SetLevel(level Level) {
	mu.Lock()
	defer mu.Unlock()
	currentLevel = level
}

// SetOutput sets log output writer
func SetOutput(w io.Writer) {
	log.SetOutput(w)
}

// Init initializes logging to file
func Init(logPath string) error {
	// Create log directory
	logDir := filepath.Dir(logPath)
	if logDir != "" && logDir != "." {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Open log file
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Set output to file only (no console)
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return nil
}

// Debug logs debug message
func Debug(format string, v ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if currentLevel <= LevelDebug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs info message
func Info(format string, v ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if currentLevel <= LevelInfo {
		log.Printf("[INFO] "+format, v...)
	}
}

// Warn logs warning message
func Warn(format string, v ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if currentLevel <= LevelWarn {
		log.Printf("[WARN] "+format, v...)
	}
}

// Error logs error message
func Error(format string, v ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if currentLevel <= LevelError {
		log.Printf("[ERROR] "+format, v...)
	}
}

// Fatal logs fatal error and exits
func Fatal(format string, v ...interface{}) {
	log.Printf("[FATAL] "+format, v...)
	os.Exit(1)
}

// ParseLevel parses log level from string
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}
