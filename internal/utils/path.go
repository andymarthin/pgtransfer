package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GetDefaultExportPath builds a timestamped export CSV path under ~/.pgtransfer/exports/
func GetDefaultExportPath(table string) (string, error) {
	base := filepath.Join(userHomeDir(), ".pgtransfer", "exports")
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", err
	}
	timestamp := time.Now().Format("20060102_150405")
	return filepath.Join(base, fmt.Sprintf("%s_%s.csv", table, timestamp)), nil
}

// GetDefaultDumpPath builds a timestamped SQL dump path under ~/.pgtransfer/dumps/
func GetDefaultDumpPath(dbName string) (string, error) {
	base := filepath.Join(userHomeDir(), ".pgtransfer", "dumps")
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", err
	}
	timestamp := time.Now().Format("20060102_150405")
	return filepath.Join(base, fmt.Sprintf("%s_%s.sql", dbName, timestamp)), nil
}

// userHomeDir safely returns the user’s home directory.
func userHomeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}

// GetConfigDir returns the ~/.pgtransfer directory path
func GetConfigDir() string {
	return filepath.Join(userHomeDir(), ".pgtransfer")
}

// GetConfigPath returns the full path to ~/.pgtransfer/config.yaml
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.yaml")
}

func GetLogDir() string {
	return filepath.Join(userHomeDir(), ".pgtransfer", "logs")
}
