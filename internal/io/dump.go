package io

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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
			dumpPath = strings.TrimSuffix(dumpPath, ".sql")
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
	// Build pg_dump command arguments
	args := []string{}

	// Add format option
	if options.Format != "" && options.Format != "plain" {
		args = append(args, "--format", options.Format)
		// Adjust file extension based on format
		switch options.Format {
		case "custom":
			if !strings.HasSuffix(dumpPath, ".dump") {
				dumpPath = strings.TrimSuffix(dumpPath, ".sql") + ".dump"
			}
		case "tar":
			if !strings.HasSuffix(dumpPath, ".tar") {
				dumpPath = strings.TrimSuffix(dumpPath, ".sql") + ".tar"
			}
		case "directory":
			// For directory format, ensure the path is a directory
			dumpPath = strings.TrimSuffix(dumpPath, ".sql")
		}
	} else {
		// Default to plain format with .sql extension
		if !strings.HasSuffix(dumpPath, ".sql") {
			dumpPath += ".sql"
		}
	}

	// Add compression option (not available for plain format)
	if options.Compress && options.Format != "plain" && options.Format != "" {
		args = append(args, "--compress")
	}

	// Add schema/data options
	if options.SchemaOnly {
		args = append(args, "--schema-only")
	}
	if options.DataOnly {
		args = append(args, "--data-only")
	}

	// Add table options
	for _, table := range options.Tables {
		args = append(args, "--table", table)
	}
	for _, table := range options.ExcludeTables {
		args = append(args, "--exclude-table", table)
	}

	// Add schema option
	if options.Schema != "" {
		args = append(args, "--schema", options.Schema)
	}

	// Add verbose option
	if options.Verbose {
		args = append(args, "--verbose")
	}

	// Add file output option
	args = append(args, "--file", dumpPath)

	// Add database URL
	args = append(args, dbURL)

	utils.PrintInfo(nil, "Starting PostgreSQL dump with advanced options...")
	if options.Verbose {
		utils.PrintInfo(nil, "Command: pg_dump %s", strings.Join(args, " "))
	}

	// Create output directory if needed
	if err := os.MkdirAll(filepath.Dir(dumpPath), 0755); err != nil {
		return fmt.Errorf("failed to create dump directory: %w", err)
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Stderr = os.Stderr

	bar := NewProgressBarWithTimer(0, fmt.Sprintf("Dumping database to %s", dumpPath))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_dump: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Handle timeout if specified
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
			cmd.Process.Kill()
			return fmt.Errorf("pg_dump timed out after %d seconds", options.Timeout)
		case <-ticker.C:
			bar.Add(1) // Advances the spinner animation
		}
	}
}

// RestoreDatabaseWithConnection performs a database restore using a DBConnection (supports SSH tunnels)
func RestoreDatabaseWithConnection(profile config.Profile, dumpPath string) error {
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
		return restoreViaTunnel(profile, dumpPath, start)
	}

	// For direct connections, use the standard approach
	dbURL := config.BuildDSN(profile)
	return restoreDatabaseSmart(dbURL, dumpPath, start)
}

// restoreViaTunnel handles database restores through SSH tunnels
func restoreViaTunnel(profile config.Profile, dumpPath string, start time.Time) error {
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
	return restoreDatabaseSmart(dbURL, dumpPath, start)
}

// restoreDatabaseSmart automatically detects dump format and uses appropriate tool with timing
func restoreDatabaseSmart(dbURL, dumpPath string, start time.Time) error {
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

// establishSSHTunnel creates a local port forward for external commands like pg_dump
func establishSSHTunnel(profile config.Profile) (*ssh.Client, int, error) {
	if !profile.SSH.Enabled {
		return nil, 0, fmt.Errorf("SSH is not enabled for this profile")
	}

	// Create SSH client configuration
	authMethods, err := sshAuth(profile.SSH)
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

// sshAuth creates SSH authentication methods (copied from db package)
func sshAuth(sshCfg config.SSHConfig) ([]ssh.AuthMethod, error) {
	var authMethods []ssh.AuthMethod

	// Password authentication
	if sshCfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(sshCfg.Password))
	}

	// Key-based authentication
	if sshCfg.KeyPath != "" {
		key, err := os.ReadFile(sshCfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key: %w", err)
		}

		var signer ssh.Signer
		if sshCfg.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(sshCfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// SSH agent authentication
	if agentConn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods available")
	}

	return authMethods, nil
}
