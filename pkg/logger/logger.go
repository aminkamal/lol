package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	logFile     *os.File
	multiWriter io.Writer
)

// Init initializes the global logger
func Init(logFilePath string) error {
	var err error

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer for stdout and file
	multiWriter = io.MultiWriter(os.Stdout, logFile)

	return nil
}

// Helper function that formats and writes log messages with caller info
func logOutput(prefix string, calldepth int, format string, v ...interface{}) {
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	_, file, line, ok := runtime.Caller(calldepth)
	if ok {
		file = filepath.Base(file)
		msg := fmt.Sprintf(format, v...)
		finalMsg := fmt.Sprintf("%s: %s %s:%d: %s\n", prefix, timestamp, file, line, msg)
		multiWriter.Write([]byte(finalMsg))
	}
}

func Info(format string, v ...interface{}) {
	logOutput("INFO", 2, format, v...)
}

func Error(format string, v ...interface{}) {
	logOutput("ERROR", 2, format, v...)
}

func Warn(format string, v ...interface{}) {
	logOutput("WARN", 2, format, v...)
}

func Debug(format string, v ...interface{}) {
	logOutput("DEBUG", 2, format, v...)
}

// Close closes the log file
func Close() error {
	if logFile != nil {
		return logFile.Close()
	}
	return nil
}
