package io

import (
	"bytes"
	"context"
	"fmt"
	goio "io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/andymarthin/pgtransfer/internal/config"
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

// DumpDatabase writes a plain SQL dump of dbURL to dumpPath.
//
// The CLI and MCP server both reach the database through a profile, so they go
// through DumpDatabaseWithConnection{,AndOptions}; this is the direct form for
// callers that already hold a connection string. It is also the path the
// integration tests exercise, because it needs no profile.
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
	defer bar.Finish()

	if err := cmd.Start(); err != nil {
		outFile.Close()
		return removeOnFailure(dumpPath, fmt.Errorf("failed to start pg_dump: %w", err))
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
				outFile.Close()
				return removeOnFailure(dumpPath, fmt.Errorf("pg_dump failed: %w", err))
			}
			if err := syncFile(outFile); err != nil {
				outFile.Close()
				return removeOnFailure(dumpPath, err)
			}
			if err := outFile.Close(); err != nil {
				return removeOnFailure(dumpPath, fmt.Errorf("failed to close %s: %w", dumpPath, err))
			}
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
		utils.PrintInfo(nil, "Command: pg_dump %s", strings.Join(redactArgs(args), " "))
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

// redactDSNURL removes the password from a connection URL so that it can be
// printed. The URL is the last pg_dump argument, and --verbose prints the whole
// argument list: the database password of the profile would otherwise be
// written to the terminal and to any log the caller redirects into.
//
// Anything that does not parse as a URL is redacted textually, because an
// unparseable string is exactly the case where a password is most likely to
// survive a naive rewrite.
func redactDSNURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		if at := strings.LastIndex(raw, "@"); at > 0 {
			if scheme := strings.Index(raw, "://"); scheme >= 0 && scheme < at {
				return raw[:scheme+3] + "***@" + raw[at+1:]
			}
		}
		return raw
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
		}
	}
	// url.URL.String would re-encode the *** placeholder; the marker is
	// deliberate, not data.
	return strings.Replace(u.String(), "%2A%2A%2A", "***", -1)
}

// redactArgs returns a copy of a command's arguments with any connection URL
// password removed, for printing.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.Contains(a, "://") && strings.Contains(a, "@") {
			out[i] = redactDSNURL(a)
			continue
		}
		out[i] = a
	}
	return out
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
	// Strict makes any restore error fatal. By default restore tolerates
	// non-fatal errors reported by pg_restore (e.g. minor client/server
	// version mismatches such as "unrecognized configuration parameter").
	// Note that pg_restore exits with code 1 when it encounters ignorable
	// errors, so the default (Strict == false) keeps restores resilient to
	// version skew.
	Strict bool
	// Verbose prints the underlying command, with the password redacted.
	Verbose bool
}

// buildPgRestoreArgs assembles the pg_restore argument list.
func buildPgRestoreArgs(dbURL, dumpPath string, clean, strict bool) []string {
	args := []string{"--no-owner", "--no-privileges"}
	if clean {
		// --clean requires --if-exists to avoid hard failures when
		// objects do not exist yet.
		args = append(args, "--clean", "--if-exists")
	}
	if strict {
		// Without --exit-on-error pg_restore reports a failed object on stderr
		// and keeps going, then exits 1 — indistinguishable from the benign
		// version-skew case. With it, the first failure aborts the restore.
		args = append(args, "--exit-on-error")
	}
	args = append(args, "--dbname", dbURL, dumpPath)
	return args
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
	return runRestoreCommand("pg_restore", buildPgRestoreArgs(dbURL, dumpPath, clean, restoreIsStrict(options)), dumpPath, options, start)
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

	// The arguments name the database, so one of them carries the password.
	// Print them only when asked, and redacted.
	if options != nil && options.Verbose {
		utils.PrintInfo(nil, "Command: %s %s", tool, strings.Join(redactArgs(args), " "))
	}

	// Restore errors are classified by what the tool wrote to stderr, not by
	// its exit status. pg_restore exits 1 both for "completed with ignorable
	// errors" and for "this table failed to load", so the exit code alone
	// cannot distinguish a version-skew warning from a restore that silently
	// dropped data. Capture stderr, classify it, and pass everything that is
	// not a known-benign message through to the user.
	collector := &stderrCollector{w: os.Stderr}
	cmd.Stderr = collector

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
				collector.flush()
				problems := collector.erroneousLines()
				if !restoreIsStrict(options) && len(problems) == 0 {
					// Nothing in the output looks like a real failure, so the
					// non-zero status came from warnings alone: version skew,
					// objects already present, or privileges we were told not to
					// restore. That is the case the tolerant default exists for.
					bar.Finish()
					duration := time.Since(start)
					utils.PrintWarning(nil, "⚠️  Database restored with warnings from %s", dumpPath)
					if n := collector.warningCount(); n > 0 {
						utils.PrintInfo(nil, "ℹ️  %d message(s) were ignored (client/server version differences, or objects that already existed)", n)
					}
					utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
					return nil
				}
				if len(problems) > 0 {
					return fmt.Errorf("%s reported %d error(s) restoring %s (first: %s): %w",
						tool, len(problems), dumpPath, problems[0], err)
				}
				return fmt.Errorf("%s failed: %w", tool, err)
			}
			collector.flush()
			if problems := collector.erroneousLines(); len(problems) > 0 {
				// Exit status 0 but real errors on stderr: psql with
				// ON_ERROR_STOP=off does exactly this.
				return fmt.Errorf("%s completed but reported %d error(s) restoring %s (first: %s)",
					tool, len(problems), dumpPath, problems[0])
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

// stderrCollector tees a restore tool's stderr to the user while remembering
// enough of it to decide whether the run actually succeeded.
//
// The exit status alone is not a reliable signal: pg_restore returns 1 both
// when it ignored benign version-skew messages and when a table failed to
// load. Classifying the messages is the only way to tell those apart.
type stderrCollector struct {
	w goio.Writer

	// kept holds lines that could not be classified immediately. It is bounded
	// so a restore that emits megabytes of errors cannot exhaust memory; once
	// full, further lines are counted but not retained.
	kept         []string
	dropped      int
	warnings     int
	erroneous    int
	unknownLines int
	pending      []byte
}

const stderrCollectorLimit = 200

func (c *stderrCollector) Write(p []byte) (int, error) {
	// Always show the output; classification is a side effect.
	n, err := c.w.Write(p)
	if n > 0 {
		c.scan(p[:n])
	}
	return n, err
}

// scan splits the chunk into lines and classifies the complete ones, keeping
// any trailing partial line for the next call.
func (c *stderrCollector) scan(p []byte) {
	c.pending = append(c.pending, p...)
	for {
		i := bytes.IndexByte(c.pending, '\n')
		if i < 0 {
			break
		}
		c.classify(string(c.pending[:i]))
		c.pending = c.pending[i+1:]
	}
	// Guard against a tool that writes an unterminated megabyte.
	if len(c.pending) > 64*1024 {
		c.classify(string(c.pending))
		c.pending = c.pending[:0]
	}
}

// flush classifies a trailing partial line left at end of output.
func (c *stderrCollector) flush() {
	if len(c.pending) > 0 {
		c.classify(string(c.pending))
		c.pending = c.pending[:0]
	}
}

// classify records one stderr line and reports whether it looks like a real
// restore failure.
func (c *stderrCollector) classify(line string) bool {
	switch classifyRestoreLine(line) {
	case restoreLineBenign:
		c.warnings++
		return false
	case restoreLineError:
		c.erroneous++
		if len(c.kept) < stderrCollectorLimit {
			c.kept = append(c.kept, strings.TrimSpace(line))
		} else {
			c.dropped++
		}
		return true
	default:
		c.unknownLines++
		return false
	}
}

// erroneousLines returns up to a handful of lines that look like real errors.
func (c *stderrCollector) erroneousLines() []string { return c.kept }

func (c *stderrCollector) warningCount() int { return c.warnings }

// restoreLineKind classifies a single line of restore output.
type restoreLineKind int

const (
	restoreLineUnknown restoreLineKind = iota
	restoreLineBenign
	restoreLineError
)

// benignRestoreSubstrings are message fragments that indicate a difference
// between client and server versions, or a pre-existing object, rather than a
// failed restore. Everything else that mentions an error is treated as fatal,
// so a new failure mode breaks the build rather than being silently tolerated.
var benignRestoreSubstrings = []string{
	"unrecognized configuration parameter",
	"already exists",
	"does not exist, skipping",
	"no privileges could be revoked",
	"no ownership could be changed", // --no-owner on objects owned by another role
	"WARNING:",
	"NOTICE:",
}

// classifyRestoreLine decides whether a line of psql/pg_restore output
// represents a benign warning, a real error, or neither.
func classifyRestoreLine(line string) restoreLineKind {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return restoreLineUnknown
	}

	isError := strings.Contains(trimmed, "ERROR:") || strings.Contains(trimmed, "FATAL:") ||
		strings.Contains(trimmed, "error:") || strings.Contains(trimmed, "pg_restore: error")

	if !isError {
		for _, b := range benignRestoreSubstrings {
			if strings.Contains(trimmed, b) {
				return restoreLineBenign
			}
		}
		return restoreLineUnknown
	}

	// An error caused by version skew or a pre-existing object is the case the
	// tolerant default exists for.
	for _, b := range benignRestoreSubstrings {
		if strings.Contains(trimmed, b) {
			return restoreLineBenign
		}
	}
	return restoreLineError
}

// syncFile flushes a written file to stable storage.
//
// A dump is read back by pg_restore or shipped to another machine, and a
// reported success that is still sitting in the page cache can be lost by a
// crash. Sync reports that failure instead of leaving the caller to discover a
// truncated file later.
func syncFile(f *os.File) error {
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to flush %s: %w", f.Name(), err)
	}
	return nil
}

// removeOnFailure deletes a partially written dump file.
//
// pg_dump writes to a path the caller then restores or ships. If it is killed
// halfway the file exists and looks like a backup, so a later restore reads a
// truncated archive and reports whatever error that produces — usually one that
// blames the restore rather than the dump that never finished.
// syncPath flushes a file that another process wrote, by opening it and
// syncing the handle.
func syncPath(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open %s for flushing: %w", path, err)
	}
	defer f.Close()
	return syncFile(f)
}

func removeOnFailure(path string, err error) error {
	if err == nil {
		return nil
	}
	if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("%w (also failed to remove partial dump %s: %v)", err, path, removeErr)
	}
	return fmt.Errorf("%w (partial dump %s removed)", err, path)
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

	tunnel, err := tunnelForCommand(profile)
	if err != nil {
		return fmt.Errorf("failed to establish SSH tunnel: %w", err)
	}
	if tunnel != nil {
		defer tunnel.Close()
	}

	return executePgDump(dbURLFor(profile, tunnel), dumpPath, start, nil)
}

// DumpDatabaseWithConnectionAndOptions performs a database dump with advanced options using DBConnection
func DumpDatabaseWithConnectionAndOptions(profile config.Profile, dumpPath string, options *DumpOptions) error {
	start := time.Now()

	if !commandExists("pg_dump") {
		return fmt.Errorf("pg_dump not found in PATH — please install PostgreSQL client tools")
	}

	tunnel, err := tunnelForCommand(profile)
	if err != nil {
		return fmt.Errorf("failed to establish SSH tunnel: %w", err)
	}
	if tunnel != nil {
		defer tunnel.Close()
	}

	return executePgDumpWithOptions(dbURLFor(profile, tunnel), dumpPath, options, start)
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
	defer bar.Finish()

	if err := cmd.Start(); err != nil {
		outFile.Close()
		return removeOnFailure(dumpPath, fmt.Errorf("failed to start pg_dump: %w", err))
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
				outFile.Close()
				return removeOnFailure(dumpPath, fmt.Errorf("pg_dump failed: %w", err))
			}
			if err := syncFile(outFile); err != nil {
				outFile.Close()
				return removeOnFailure(dumpPath, err)
			}
			if err := outFile.Close(); err != nil {
				return removeOnFailure(dumpPath, fmt.Errorf("failed to close %s: %w", dumpPath, err))
			}
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
		utils.PrintInfo(nil, "Command: pg_dump %s", strings.Join(redactArgs(args), " "))
	}

	// Create output directory if needed.
	if err := os.MkdirAll(filepath.Dir(dumpPath), 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %w", err)
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	bar := NewProgressBarWithTimer(0, fmt.Sprintf("Dumping database to %s", dumpPath))
	defer bar.Finish()

	if err := cmd.Start(); err != nil {
		return removeOnFailure(dumpPath, fmt.Errorf("failed to start pg_dump: %w", err))
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
				return removeOnFailure(dumpPath, fmt.Errorf("pg_dump failed: %w", err))
			}
			// pg_dump wrote the file itself (--file), so flush it through a
			// handle of our own before reporting success.
			if err := syncPath(dumpPath); err != nil {
				return removeOnFailure(dumpPath, err)
			}
			duration := time.Since(start)
			utils.PrintSuccess(nil, "✅ Database dumped successfully to %s", dumpPath)
			utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
			return nil
		case <-timeoutChan:
			if err := cmd.Process.Kill(); err != nil {
				utils.PrintError(nil, "Failed to kill timed out process: %v", err)
			}
			<-done
			return removeOnFailure(dumpPath, fmt.Errorf("pg_dump timed out after %d seconds", options.Timeout))
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

	tunnel, err := tunnelForCommand(profile)
	if err != nil {
		return fmt.Errorf("failed to establish SSH tunnel: %w", err)
	}
	if tunnel != nil {
		defer tunnel.Close()
	}

	return restoreDatabaseSmartWithOptions(dbURLFor(profile, tunnel), dumpPath, options, start)
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

// sshTunnel is a local listener that forwards every accepted connection to the
// database host through an SSH client. It is created once per command and
// closed when the command finishes.
type sshTunnel struct {
	client   *ssh.Client
	listener net.Listener
	port     int
	done     chan struct{}
}

// localPort returns the port the tunnel listens on.
func (t *sshTunnel) localPort() int { return t.port }

// Close stops accepting connections, closes the listener and the SSH client,
// and waits for the forwarding goroutine to exit.
func (t *sshTunnel) Close() error {
	close(t.done)
	err := t.listener.Close()
	if t.client != nil {
		_ = t.client.Close()
	}
	return err
}

// openTunnel dials SSH and starts forwarding. The listener is bound before it
// is returned, so a tunnel handed back to the caller is already accepting
// connections — there is no window in which the port is advertised but dead.
func openTunnel(profile config.Profile) (*sshTunnel, error) {
	if !profile.SSH.Enabled {
		return nil, fmt.Errorf("SSH is not enabled for this profile")
	}

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
		return nil, fmt.Errorf("SSH auth setup failed: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User:            profile.SSH.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(profile.SSH.Timeout) * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", profile.SSH.Host, profile.SSH.Port)
	sshClient, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed SSH connection: %w", err)
	}

	// Bind on the loopback interface only: the forwarded database port must
	// not be reachable from other hosts.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("failed to find available local port: %w", err)
	}

	tunnel := &sshTunnel{
		client:   sshClient,
		listener: listener,
		port:     listener.Addr().(*net.TCPAddr).Port,
		done:     make(chan struct{}),
	}
	go tunnel.forward(profile)
	return tunnel, nil
}

// forward accepts local connections until the tunnel is closed and pipes each
// one to the database host over SSH.
func (t *sshTunnel) forward(profile config.Profile) {
	remoteAddr := fmt.Sprintf("%s:%d", profile.Host, profile.Port)
	for {
		localConn, err := t.listener.Accept()
		if err != nil {
			return
		}
		remoteConn, err := t.client.Dial("tcp", remoteAddr)
		if err != nil {
			_ = localConn.Close()
			continue
		}
		go pipeConnections(localConn, remoteConn)
	}
}

// pipeConnections copies data in both directions and closes both connections
// when either side finishes, so a half-open socket cannot be left behind.
func pipeConnections(local, remote net.Conn) {
	var once sync.Once
	closeBoth := func() {
		_ = local.Close()
		_ = remote.Close()
	}

	done := make(chan struct{}, 2)
	go func() { copyData(remote, local); done <- struct{}{} }()
	go func() { copyData(local, remote); done <- struct{}{} }()

	<-done
	once.Do(closeBoth)
	// Unblock the other direction, which may be parked in a read.
	<-done
}

// tunnelForCommand returns a tunnel only when the profile needs one. It is
// created once per dump or restore instead of relying on a pooled connection
// that is then discarded.
func tunnelForCommand(profile config.Profile) (*sshTunnel, error) {
	if !profile.SSH.Enabled {
		return nil, nil
	}
	utils.PrintInfo(nil, "🔐 Using SSH tunnel to %s:%d...", profile.SSH.Host, profile.SSH.Port)
	return openTunnel(profile)
}

// dbURLFor builds the connection string a dump or restore should use. With
// a tunnel the connection points at the tunnel's local port.
func dbURLFor(profile config.Profile, tunnel *sshTunnel) string {
	if tunnel == nil {
		return config.BuildDSN(profile)
	}
	localProfile := profile
	localProfile.Host = "127.0.0.1"
	localProfile.Port = tunnel.localPort()
	localProfile.SSH.Enabled = false
	return config.BuildDSN(localProfile)
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
