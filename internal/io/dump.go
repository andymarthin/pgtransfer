package io

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
)

func DumpDatabase(dbURL, dumpPath string) error {
	start := time.Now()

	if !strings.HasSuffix(dumpPath, ".sql") {
		dumpPath += ".sql"
	}

	if err := os.MkdirAll(filepath.Dir(dumpPath), 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %w", err)
	}

	if !commandExists("pg_dump") {
		return fmt.Errorf("pg_dump not found in PATH — please install PostgreSQL client tools")
	}

	utils.PrintInfo(nil, "Starting PostgreSQL dump...")

	cmd := exec.Command("pg_dump", dbURL)
	outFile, err := os.Create(dumpPath)
	if err != nil {
		return fmt.Errorf("failed to create dump file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr

	bar := NewProgressBarWithTimer(0, fmt.Sprintf("Dumping database to %s", dumpPath))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_dump: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("pg_dump failed: %w", err)
			}
			duration := time.Since(start)
			utils.PrintSuccess(nil, "✅ Database dumped successfully to %s", dumpPath)
			utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
			return nil
		case <-ticker.C:
			bar.Add(1)
		}
	}
}

func RestoreDatabase(dbURL, dumpPath string) error {
	start := time.Now()

	if _, err := os.Stat(dumpPath); err != nil {
		return fmt.Errorf("dump file not found: %w", err)
	}

	// ✅ Check pg_restore existence
	if !commandExists("pg_restore") {
		return fmt.Errorf("pg_restore not found in PATH — please install PostgreSQL client tools")
	}

	utils.PrintInfo(nil, "Starting database restore from %s...", dumpPath)

	cmd := exec.Command("pg_restore", "--no-owner", "--no-privileges", "--dbname", dbURL, dumpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	bar := NewProgressBarWithTimer(0, fmt.Sprintf("Restoring from %s", dumpPath))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start restore: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("pg_restore failed: %w", err)
			}
			duration := time.Since(start)
			utils.PrintSuccess(nil, "✅ Database restored successfully from %s", dumpPath)
			utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
			return nil
		case <-ticker.C:
			bar.Add(1)
		}
	}
}

func commandExists(cmd string) bool {
	path, err := exec.LookPath(cmd)
	return err == nil && path != ""
}
