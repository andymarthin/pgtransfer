package log

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
)

type Entry struct {
	Timestamp string `json:"timestamp"`
	Command   string `json:"command"`
	Profile   string `json:"profile"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Duration  string `json:"duration"`
}

// write appends a log entry to ~/.pgtransfer/logs/YYYY-MM-DD.log
func write(entry Entry) error {
	logDir := utils.GetLogDir()

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	filePath := filepath.Join(logDir, fmt.Sprintf("%s.log", time.Now().Format("2006-01-02")))
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to encode log entry: %w", err)
	}

	_, err = file.Write(append(data, '\n'))
	return err
}

// Success records a success entry
func Success(command, profile, message string, start time.Time) {
	entry := Entry{
		Timestamp: time.Now().Format(time.RFC3339),
		Command:   command,
		Profile:   profile,
		Status:    "SUCCESS",
		Message:   message,
		Duration:  utils.FormatDuration(time.Since(start)),
	}
	_ = write(entry)
}

// Failure records a failed operation
func Failure(command, profile, message string, start time.Time) {
	entry := Entry{
		Timestamp: time.Now().Format(time.RFC3339),
		Command:   command,
		Profile:   profile,
		Status:    "FAILURE",
		Message:   message,
		Duration:  utils.FormatDuration(time.Since(start)),
	}
	_ = write(entry)
}

// Info logs an informational entry
func Info(command, profile, message string) {
	entry := Entry{
		Timestamp: time.Now().Format(time.RFC3339),
		Command:   command,
		Profile:   profile,
		Status:    "INFO",
		Message:   message,
	}
	_ = write(entry)
}
