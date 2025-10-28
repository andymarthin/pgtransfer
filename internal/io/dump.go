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

// DumpOptions contains advanced options for pg_dump
type DumpOptions struct {
	Format        string   // plain, custom, directory, tar
	Compress      bool     // Enable compression
	SchemaOnly    bool     // Export schema only
	DataOnly      bool     // Export data only
	Tables        []string // Specific tables to include
	ExcludeTables []string // Tables to exclude
	Schema        string   // Specific schema
	Verbose       bool     // Verbose output
	Timeout       int      // Command timeout in seconds
}

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
			bar.Finish()
			duration := time.Since(start)
			utils.PrintSuccess(nil, "✅ Database dumped successfully to %s", dumpPath)
			utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
			return nil
		case <-ticker.C:
			bar.Add(1) // Advances the spinner animation
		}
	}
}

// DumpDatabaseWithOptions performs a database dump with advanced options
func DumpDatabaseWithOptions(dbURL, dumpPath string, options *DumpOptions) error {
	start := time.Now()

	if !commandExists("pg_dump") {
		return fmt.Errorf("pg_dump not found in PATH — please install PostgreSQL client tools")
	}

	// Build pg_dump command arguments
	args := []string{}

	// Add format option
	if options.Format != "" && options.Format != "plain" {
		args = append(args, "--format", options.Format)
		// Adjust file extension based on format
		switch options.Format {
		case "custom":
			if !strings.HasSuffix(dumpPath, ".dump") && !strings.HasSuffix(dumpPath, ".backup") {
				dumpPath += ".dump"
			}
		case "tar":
			if !strings.HasSuffix(dumpPath, ".tar") {
				dumpPath += ".tar"
			}
		case "directory":
			// For directory format, ensure it's a directory path
			if strings.HasSuffix(dumpPath, ".sql") {
				dumpPath = strings.TrimSuffix(dumpPath, ".sql")
			}
		default: // plain
			if !strings.HasSuffix(dumpPath, ".sql") {
				dumpPath += ".sql"
			}
		}
	} else {
		// Default to .sql for plain format
		if !strings.HasSuffix(dumpPath, ".sql") {
			dumpPath += ".sql"
		}
	}

	// Add compression
	if options.Compress && options.Format != "plain" {
		args = append(args, "--compress", "6")
	}

	// Add schema/data only options
	if options.SchemaOnly {
		args = append(args, "--schema-only")
	} else if options.DataOnly {
		args = append(args, "--data-only")
	}

	// Add table filters
	for _, table := range options.Tables {
		args = append(args, "--table", table)
	}
	for _, table := range options.ExcludeTables {
		args = append(args, "--exclude-table", table)
	}

	// Add schema filter
	if options.Schema != "" {
		args = append(args, "--schema", options.Schema)
	}

	// Add verbose option
	if options.Verbose {
		args = append(args, "--verbose")
	}

	// Add output file (except for directory format)
	if options.Format != "directory" {
		args = append(args, "--file", dumpPath)
	} else {
		args = append(args, "--file", dumpPath)
	}

	// Add database URL
	args = append(args, dbURL)

	// Create output directory
	if err := os.MkdirAll(filepath.Dir(dumpPath), 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %w", err)
	}

	utils.PrintInfo(nil, "Starting PostgreSQL dump with advanced options...")
	if options.Verbose {
		utils.PrintInfo(nil, "Command: pg_dump %s", strings.Join(args, " "))
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Stderr = os.Stderr

	// For plain format without file output, redirect to file
	if options.Format == "" || options.Format == "plain" {
		outFile, err := os.Create(dumpPath)
		if err != nil {
			return fmt.Errorf("failed to create dump file: %w", err)
		}
		defer outFile.Close()
		cmd.Stdout = outFile
		// Remove --file argument for plain format
		for i, arg := range args {
			if arg == "--file" && i+1 < len(args) {
				args = append(args[:i], args[i+2:]...)
				break
			}
		}
		cmd.Args = append([]string{"pg_dump"}, args...)
	}

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

	// Handle timeout
	var timeoutChan <-chan time.Time
	if options.Timeout > 0 {
		timeoutChan = time.After(time.Duration(options.Timeout) * time.Second)
	}

	for {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("pg_dump failed: %w", err)
			}
			bar.Finish()
			duration := time.Since(start)
			utils.PrintSuccess(nil, "✅ Database dumped successfully to %s", dumpPath)
			utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
			return nil
		case <-ticker.C:
			bar.Add(1) // Advances the spinner animation
		case <-timeoutChan:
			if err := cmd.Process.Kill(); err != nil {
				utils.PrintError(nil, "Failed to kill timed out process: %v", err)
			}
			return fmt.Errorf("dump operation timed out after %d seconds", options.Timeout)
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

// RestoreDatabaseSmart automatically detects dump format and uses appropriate tool
func RestoreDatabaseSmart(dbURL, dumpPath string) error {
	start := time.Now()

	if _, err := os.Stat(dumpPath); err != nil {
		return fmt.Errorf("dump file not found: %w", err)
	}

	// Detect dump format by checking file content
	isPlainText, err := isPlainTextDump(dumpPath)
	if err != nil {
		return fmt.Errorf("failed to detect dump format: %w", err)
	}

	if isPlainText {
		return restoreWithPsql(dbURL, dumpPath, start)
	} else {
		return restoreWithPgRestore(dbURL, dumpPath, start)
	}
}

// isPlainTextDump checks if the dump file is a plain text SQL dump
func isPlainTextDump(dumpPath string) (bool, error) {
	file, err := os.Open(dumpPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read first 1024 bytes to check format
	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return false, err
	}

	content := string(buffer[:n])
	
	// Plain text dumps typically start with SQL comments or SET commands
	return strings.Contains(content, "-- PostgreSQL database dump") ||
		   strings.Contains(content, "SET ") ||
		   strings.HasPrefix(strings.TrimSpace(content), "--"), nil
}

// restoreWithPsql restores a plain text SQL dump using psql
func restoreWithPsql(dbURL, dumpPath string, start time.Time) error {
	if !commandExists("psql") {
		return fmt.Errorf("psql not found in PATH — please install PostgreSQL client tools")
	}

	utils.PrintInfo(nil, "Starting database restore from %s using psql...", dumpPath)

	cmd := exec.Command("psql", dbURL, "-f", dumpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	bar := NewProgressBarWithTimer(0, fmt.Sprintf("Restoring from %s", dumpPath))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start psql: %w", err)
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
				return fmt.Errorf("psql failed: %w", err)
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

// restoreWithPgRestore restores a custom format dump using pg_restore
func restoreWithPgRestore(dbURL, dumpPath string, start time.Time) error {
	if !commandExists("pg_restore") {
		return fmt.Errorf("pg_restore not found in PATH — please install PostgreSQL client tools")
	}

	utils.PrintInfo(nil, "Starting database restore from %s using pg_restore...", dumpPath)

	cmd := exec.Command("pg_restore", "--no-owner", "--no-privileges", "--dbname", dbURL, dumpPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	bar := NewProgressBarWithTimer(0, fmt.Sprintf("Restoring from %s", dumpPath))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_restore: %w", err)
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
