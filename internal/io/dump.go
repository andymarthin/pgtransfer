package io

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	goio "io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/andymarthin/pgtransfer/internal/sshx"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"golang.org/x/crypto/ssh"
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

	// Normalize the output path extension based on the requested format.
	dumpPath = normalizeDumpPath(dumpPath, options)

	// Build pg_dump command arguments.
	args := []string{}

	// Format.
	if options.Format != "" && options.Format != "plain" {
		args = append(args, "--format", options.Format)
	}

	// Compression (not applicable to plain format).
	if options.Compress && options.Format != "" && options.Format != "plain" {
		args = append(args, "--compress", "6")
	}

	// Schema / data only.
	if options.SchemaOnly {
		args = append(args, "--schema-only")
	} else if options.DataOnly {
		args = append(args, "--data-only")
	}

	// Table filters.
	for _, table := range options.Tables {
		args = append(args, "--table", table)
	}
	for _, table := range options.ExcludeTables {
		args = append(args, "--exclude-table", table)
	}

	// Schema filter.
	if options.Schema != "" {
		args = append(args, "--schema", options.Schema)
	}

	// Verbose.
	if options.Verbose {
		args = append(args, "--verbose")
	}

	// Output file. pg_dump supports --file for every format, including plain.
	args = append(args, "--file", dumpPath)

	// Database URL last.
	args = append(args, dbURL)

	// Create output directory.
	if err := os.MkdirAll(filepath.Dir(dumpPath), 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %w", err)
	}

	utils.PrintInfo(nil, "Starting PostgreSQL dump with advanced options...")
	if options.Verbose {
		utils.PrintInfo(nil, "Command: pg_dump %s", strings.Join(args, " "))
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Stdout = os.Stdout
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

	// Handle timeout.
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
		case <-timeoutChan:
			if err := cmd.Process.Kill(); err != nil {
				utils.PrintError(nil, "Failed to kill timed out process: %v", err)
			}
			return fmt.Errorf("dump operation timed out after %d seconds", options.Timeout)
		}
	}
}

// normalizeDumpPath adjusts the dump file extension to match the requested
// pg_dump output format (plain, custom, tar or directory).
func normalizeDumpPath(dumpPath string, options *DumpOptions) string {
	switch options.Format {
	case "custom":
		if !strings.HasSuffix(dumpPath, ".dump") && !strings.HasSuffix(dumpPath, ".backup") {
			dumpPath = strings.TrimSuffix(dumpPath, ".sql") + ".dump"
		}
	case "tar":
		if !strings.HasSuffix(dumpPath, ".tar") {
			dumpPath = strings.TrimSuffix(dumpPath, ".sql") + ".tar"
		}
	case "directory":
		// For directory format the path is a directory, not a file.
		dumpPath = strings.TrimSuffix(dumpPath, ".sql")
	default: // plain or unspecified
		if !strings.HasSuffix(dumpPath, ".sql") {
			dumpPath = strings.TrimSuffix(dumpPath, ".dump") + ".sql"
		}
	}
	return dumpPath
}

// RestoreOptions controls restore behaviour for pg_restore/psql.
type RestoreOptions struct {
	// Clean drops database objects before recreating them (pg_restore only).
	Clean bool
	// Timeout, when > 0, aborts the restore after the given number of seconds.
	Timeout int
	// IgnoreErrors tolerates non-fatal restore errors (e.g. minor version
	// mismatches such as "unrecognized configuration parameter") reported by
	// pg_restore. When true, a non-zero exit is treated as a warning rather
	// than a failure, provided the process did not time out.
	IgnoreErrors bool
	// Strict forces any restore error to be reported as a failure, overriding
	// IgnoreErrors. Note that pg_restore exits with code 1 when it encounters
	// ignorable errors, so Strict defaults to false at the CLI level to remain
	// resilient to client/server version skew.
	Strict bool
}

// buildPgRestoreArgs assembles the pg_restore argument list.
func buildPgRestoreArgs(dbURL, dumpPath string, clean bool) []string {
	args := []string{"--no-owner", "--no-privileges"}
	if clean {
		// --clean requires --if-exists to avoid hard failures when
		// objects do not exist yet.
		args = append(args, "--clean", "--if-exists")
	}
	args = append(args, "--dbname", dbURL, dumpPath)
	return args
}

func RestoreDatabase(dbURL, dumpPath string) error {
	start := time.Now()

	if _, err := os.Stat(dumpPath); err != nil {
		return fmt.Errorf("dump file not found: %w", err)
	}

	if !commandExists("pg_restore") {
		return fmt.Errorf("pg_restore not found in PATH — please install PostgreSQL client tools")
	}

	utils.PrintInfo(nil, "Starting database restore from %s...", dumpPath)

	return runRestoreCommand("pg_restore", buildPgRestoreArgs(dbURL, dumpPath, false), dumpPath, nil, start)
}

// RestoreDatabaseSmart automatically detects dump format and uses appropriate tool
func RestoreDatabaseSmart(dbURL, dumpPath string) error {
	return RestoreDatabaseSmartWithOptions(dbURL, dumpPath, nil)
}

// RestoreDatabaseSmartWithOptions detects the dump format and restores it,
// honouring the supplied RestoreOptions.
func RestoreDatabaseSmartWithOptions(dbURL, dumpPath string, options *RestoreOptions) error {
	if _, err := os.Stat(dumpPath); err != nil {
		return fmt.Errorf("dump file not found: %w", err)
	}
	return restoreDatabaseSmartWithOptions(dbURL, dumpPath, options, time.Now())
}

// isPlainTextDump reports whether the dump at dumpPath is a plain-text SQL
// dump (restorable with psql) as opposed to a binary pg_dump archive
// (custom/tar, restorable with pg_restore).
func isPlainTextDump(dumpPath string) (bool, error) {
	file, err := os.Open(dumpPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read the first 1024 bytes to inspect the file signature.
	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && err != goio.EOF {
		return false, err
	}
	content := buffer[:n]

	// Binary pg_dump archives (custom format) start with the magic "PGDMP".
	if bytes.HasPrefix(content, []byte("PGDMP")) {
		return false, nil
	}

	// tar archives are detected by the ustar magic at offset 257.
	if len(content) >= 262 && string(content[257:262]) == "ustar" {
		return false, nil
	}

	// Otherwise require a recognisable plain-SQL marker near the top of the
	// file to avoid misclassifying binary content as SQL text.
	text := string(content)
	if !utf8.Valid(content) {
		return false, nil
	}
	return strings.Contains(text, "-- PostgreSQL database dump") ||
		strings.Contains(text, "SET statement_timeout") ||
		strings.Contains(text, "CREATE TABLE") ||
		strings.Contains(text, "COPY ") ||
		strings.Contains(text, "INSERT INTO"), nil
}

// restoreWithPsql restores a plain text SQL dump using psql.
func restoreWithPsql(dbURL, dumpPath string, options *RestoreOptions, start time.Time) error {
	if !commandExists("psql") {
		return fmt.Errorf("psql not found in PATH — please install PostgreSQL client tools")
	}

	utils.PrintInfo(nil, "Starting database restore from %s using psql...", dumpPath)

	// Build the psql argument list. By default we continue past individual
	// statement errors (ON_ERROR_STOP=off) so that benign client/server
	// version-skew artifacts — e.g. "unrecognized configuration parameter"
	// emitted by a newer pg_dump — do not abort the whole restore. Callers can
	// request strict behaviour, in which case the first error stops the run.
	args := []string{dbURL}
	if restoreIsStrict(options) {
		args = append(args, "-v", "ON_ERROR_STOP=1")
	} else {
		args = append(args, "-v", "ON_ERROR_STOP=off")
	}
	args = append(args, "-f", dumpPath)

	return runRestoreCommand("psql", args, dumpPath, options, start)
}

// restoreWithPgRestore restores a custom format dump using pg_restore.
func restoreWithPgRestore(dbURL, dumpPath string, options *RestoreOptions, start time.Time) error {
	if !commandExists("pg_restore") {
		return fmt.Errorf("pg_restore not found in PATH — please install PostgreSQL client tools")
	}

	utils.PrintInfo(nil, "Starting database restore from %s using pg_restore...", dumpPath)

	clean := options != nil && options.Clean
	return runRestoreCommand("pg_restore", buildPgRestoreArgs(dbURL, dumpPath, clean), dumpPath, options, start)
}

// runRestoreCommand runs a restore tool (psql/pg_restore) while driving the
// progress spinner, enforcing an optional timeout and reporting timing.
func runRestoreCommand(tool string, args []string, dumpPath string, options *RestoreOptions, start time.Time) error {
	ctx := context.Background()
	var cancel context.CancelFunc
	if options != nil && options.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(options.Timeout)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	bar := NewProgressBarWithTimer(0, fmt.Sprintf("Restoring from %s", dumpPath))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", tool, err)
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
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("%s timed out after %d seconds", tool, options.Timeout)
			}
			if err != nil {
				// pg_restore exits with status 1 when it encountered errors it
				// was told (or defaulted) to ignore — e.g. benign client/server
				// version skew such as "unrecognized configuration parameter".
				// Treat that as a warning unless the caller asked for strict
				// behaviour.
				if tool == "pg_restore" && !restoreIsStrict(options) && pgRestoreErrorsIgnored(err) {
					bar.Finish()
					duration := time.Since(start)
					utils.PrintWarning(nil, "⚠️  Database restored with warnings from %s", dumpPath)
					utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
					return nil
				}
				return fmt.Errorf("%s failed: %w", tool, err)
			}
			bar.Finish()
			duration := time.Since(start)
			utils.PrintSuccess(nil, "✅ Database restored successfully from %s", dumpPath)
			utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
			return nil
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				_ = cmd.Process.Kill()
				return fmt.Errorf("%s timed out after %d seconds", tool, options.Timeout)
			}
			return ctx.Err()
		case <-ticker.C:
			bar.Add(1)
		}
	}
}

// restoreIsStrict reports whether restore errors should be treated as fatal.
func restoreIsStrict(options *RestoreOptions) bool {
	return options != nil && options.Strict
}

// pgRestoreErrorsIgnored reports whether the pg_restore error corresponds to
// the "errors ignored" condition (exit status 1 with the ignore-errors flag).
func pgRestoreErrorsIgnored(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	// pg_restore uses exit code 1 for "completed with ignorable errors".
	return exitErr.ExitCode() == 1
}

func commandExists(cmd string) bool {
	path, err := exec.LookPath(cmd)
	return err == nil && path != ""
}

// DumpDatabaseWithConnection performs a database dump using a DBConnection (supports SSH tunnels)
func DumpDatabaseWithConnection(profile config.Profile, dumpPath string) error {
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

	// Establish connection (handles both direct and SSH tunnel)
	conn, err := db.Connect(profile)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	// For SSH tunnels, we need to handle the connection differently
	if conn.Mode == "tunnel" {
		return dumpViaTunnel(profile, dumpPath, start)
	}

	// For direct connections, use the standard approach
	dbURL := config.BuildDSN(profile)
	return executePgDump(dbURL, dumpPath, start, nil)
}

// DumpDatabaseWithConnectionAndOptions performs a database dump with advanced options using DBConnection
func DumpDatabaseWithConnectionAndOptions(profile config.Profile, dumpPath string, options *DumpOptions) error {
	start := time.Now()

	if !commandExists("pg_dump") {
		return fmt.Errorf("pg_dump not found in PATH — please install PostgreSQL client tools")
	}

	// Establish connection (handles both direct and SSH tunnel)
	conn, err := db.Connect(profile)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	// For SSH tunnels, we need to handle the connection differently
	if conn.Mode == "tunnel" {
		return dumpViaTunnelWithOptions(profile, dumpPath, options, start)
	}

	// For direct connections, use the standard approach
	dbURL := config.BuildDSN(profile)
	return executePgDumpWithOptions(dbURL, dumpPath, options, start)
}

// dumpViaTunnel handles database dumps through SSH tunnels
func dumpViaTunnel(profile config.Profile, dumpPath string, start time.Time) error {
	utils.PrintInfo(nil, "🔐 Using SSH tunnel for database dump...")

	// Establish SSH tunnel and get local port
	tunnel, localPort, err := establishSSHTunnel(profile)
	if err != nil {
		return fmt.Errorf("failed to establish SSH tunnel: %w", err)
	}
	defer tunnel.Close()

	// Create local profile using the tunnel
	localProfile := profile
	localProfile.Host = "localhost"
	localProfile.Port = localPort
	localProfile.SSH.Enabled = false // Disable SSH for the local connection

	dbURL := config.BuildDSN(localProfile)
	return executePgDump(dbURL, dumpPath, start, nil)
}

// dumpViaTunnelWithOptions handles database dumps through SSH tunnels with options
func dumpViaTunnelWithOptions(profile config.Profile, dumpPath string, options *DumpOptions, start time.Time) error {
	utils.PrintInfo(nil, "🔐 Using SSH tunnel for database dump with options...")

	// Establish SSH tunnel and get local port
	tunnel, localPort, err := establishSSHTunnel(profile)
	if err != nil {
		return fmt.Errorf("failed to establish SSH tunnel: %w", err)
	}
	defer tunnel.Close()

	// Create local profile using the tunnel
	localProfile := profile
	localProfile.Host = "localhost"
	localProfile.Port = localPort
	localProfile.SSH.Enabled = false // Disable SSH for the local connection

	dbURL := config.BuildDSN(localProfile)
	return executePgDumpWithOptions(dbURL, dumpPath, options, start)
}

// executePgDump executes the pg_dump command with basic options
func executePgDump(dbURL, dumpPath string, start time.Time, options *DumpOptions) error {
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

// executePgDumpWithOptions executes the pg_dump command with advanced options
func executePgDumpWithOptions(dbURL, dumpPath string, options *DumpOptions, start time.Time) error {
	// Normalize the output path extension based on the requested format.
	dumpPath = normalizeDumpPath(dumpPath, options)

	// Build pg_dump command arguments.
	args := []string{}

	// Format.
	if options.Format != "" && options.Format != "plain" {
		args = append(args, "--format", options.Format)
	}

	// Compression option (not available for plain format).
	if options.Compress && options.Format != "" && options.Format != "plain" {
		args = append(args, "--compress", "6")
	}

	// Schema / data options.
	if options.SchemaOnly {
		args = append(args, "--schema-only")
	} else if options.DataOnly {
		args = append(args, "--data-only")
	}

	// Table options.
	for _, table := range options.Tables {
		args = append(args, "--table", table)
	}
	for _, table := range options.ExcludeTables {
		args = append(args, "--exclude-table", table)
	}

	// Schema option.
	if options.Schema != "" {
		args = append(args, "--schema", options.Schema)
	}

	// Verbose option.
	if options.Verbose {
		args = append(args, "--verbose")
	}

	// File output option.
	args = append(args, "--file", dumpPath)

	// Database URL.
	args = append(args, dbURL)

	utils.PrintInfo(nil, "Starting PostgreSQL dump with advanced options...")
	if options.Verbose {
		utils.PrintInfo(nil, "Command: pg_dump %s", strings.Join(args, " "))
	}

	// Create output directory if needed.
	if err := os.MkdirAll(filepath.Dir(dumpPath), 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %w", err)
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	bar := NewProgressBarWithTimer(0, fmt.Sprintf("Dumping database to %s", dumpPath))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_dump: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Handle timeout if specified.
	var timeoutChan <-chan time.Time
	if options.Timeout > 0 {
		timeoutChan = time.After(time.Duration(options.Timeout) * time.Second)
	}

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
		case <-timeoutChan:
			if err := cmd.Process.Kill(); err != nil {
				utils.PrintError(nil, "Failed to kill timed out process: %v", err)
			}
			return fmt.Errorf("pg_dump timed out after %d seconds", options.Timeout)
		case <-ticker.C:
			bar.Add(1) // Advances the spinner animation
		}
	}
}

// RestoreDatabaseWithConnection performs a database restore using a DBConnection (supports SSH tunnels)
// RestoreDatabaseWithConnection performs a database restore using a DBConnection (supports SSH tunnels)
func RestoreDatabaseWithConnection(profile config.Profile, dumpPath string) error {
	return RestoreDatabaseWithConnectionAndOptions(profile, dumpPath, nil)
}

// RestoreDatabaseWithConnectionAndOptions performs a database restore using a
// DBConnection (supports SSH tunnels), honouring the supplied RestoreOptions.
func RestoreDatabaseWithConnectionAndOptions(profile config.Profile, dumpPath string, options *RestoreOptions) error {
	start := time.Now()

	if _, err := os.Stat(dumpPath); err != nil {
		return fmt.Errorf("dump file not found: %w", err)
	}

	// Establish connection (handles both direct and SSH tunnel)
	conn, err := db.Connect(profile)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	// For SSH tunnels, we need to handle the connection differently
	if conn.Mode == "tunnel" {
		return restoreViaTunnel(profile, dumpPath, options, start)
	}

	// For direct connections, use the standard approach
	dbURL := config.BuildDSN(profile)
	return restoreDatabaseSmartWithOptions(dbURL, dumpPath, options, start)
}

// restoreViaTunnel handles database restores through SSH tunnels
func restoreViaTunnel(profile config.Profile, dumpPath string, options *RestoreOptions, start time.Time) error {
	utils.PrintInfo(nil, "🔐 Using SSH tunnel for database restore...")

	// Establish SSH tunnel and get local port
	tunnel, localPort, err := establishSSHTunnel(profile)
	if err != nil {
		return fmt.Errorf("failed to establish SSH tunnel: %w", err)
	}
	defer tunnel.Close()

	// Create local profile using the tunnel
	localProfile := profile
	localProfile.Host = "localhost"
	localProfile.Port = localPort
	localProfile.SSH.Enabled = false // Disable SSH for the local connection

	dbURL := config.BuildDSN(localProfile)
	return restoreDatabaseSmartWithOptions(dbURL, dumpPath, options, start)
}

// restoreDatabaseSmartWithOptions detects the dump format and restores it,
// honouring the supplied RestoreOptions, and reports timing.
func restoreDatabaseSmartWithOptions(dbURL, dumpPath string, options *RestoreOptions, start time.Time) error {
	// Detect dump format by checking file content
	isPlainText, err := isPlainTextDump(dumpPath)
	if err != nil {
		return fmt.Errorf("failed to detect dump format: %w", err)
	}

	if isPlainText {
		// pg_restore --clean does not apply to plain SQL dumps; the dump
		// itself is expected to contain the necessary DROP statements.
		return restoreWithPsql(dbURL, dumpPath, options, start)
	}
	return restoreWithPgRestore(dbURL, dumpPath, options, start)
}

// establishSSHTunnel creates a local port forward for external commands like pg_dump
func establishSSHTunnel(profile config.Profile) (*ssh.Client, int, error) {
	if !profile.SSH.Enabled {
		return nil, 0, fmt.Errorf("SSH is not enabled for this profile")
	}

	// Establish SSH tunnel and get local port
	authMethods, err := sshx.BuildAuthMethods(sshx.Config{
		User:       profile.SSH.User,
		Host:       profile.SSH.Host,
		Port:       profile.SSH.Port,
		KeyPath:    profile.SSH.KeyPath,
		Passphrase: profile.SSH.Passphrase,
		Password:   profile.SSH.Password,
		Timeout:    profile.SSH.Timeout,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("SSH auth setup failed: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User:            profile.SSH.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(profile.SSH.Timeout) * time.Second,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", profile.SSH.Host, profile.SSH.Port)
	sshClient, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, 0, fmt.Errorf("failed SSH connection: %w", err)
	}

	// Find an available local port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		sshClient.Close()
		return nil, 0, fmt.Errorf("failed to find available local port: %w", err)
	}
	localPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Start port forwarding in a goroutine
	go func() {
		for {
			// Listen on local port
			localListener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", localPort))
			if err != nil {
				return
			}
			defer localListener.Close()

			for {
				// Accept local connection
				localConn, err := localListener.Accept()
				if err != nil {
					return
				}

				// Connect to remote database through SSH tunnel
				remoteAddr := fmt.Sprintf("%s:%d", profile.Host, profile.Port)
				remoteConn, err := sshClient.Dial("tcp", remoteAddr)
				if err != nil {
					localConn.Close()
					continue
				}

				// Start forwarding data between connections
				go func() {
					defer localConn.Close()
					defer remoteConn.Close()

					// Forward data in both directions
					go func() {
						defer localConn.Close()
						defer remoteConn.Close()
						copyData(localConn, remoteConn)
					}()
					copyData(remoteConn, localConn)
				}()
			}
		}
	}()

	// Give the tunnel a moment to start
	time.Sleep(100 * time.Millisecond)

	return sshClient, localPort, nil
}

// copyData copies data between two connections
func copyData(dst, src net.Conn) {
	buffer := make([]byte, 32*1024)
	for {
		n, err := src.Read(buffer)
		if err != nil {
			return
		}
		_, err = dst.Write(buffer[:n])
		if err != nil {
			return
		}
	}
}
