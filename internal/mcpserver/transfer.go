package mcpserver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/andymarthin/pgtransfer/internal/io"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTransferTools adds the tools that move data.
func registerTransferTools(srv *mcp.Server) {
	registerExportCSV(srv)
	registerImportCSV(srv)
	registerExportDump(srv)
	registerImportDump(srv)
	registerMigrate(srv)
}

func registerExportCSV(srv *mcp.Server) {
	srv.AddTool(&mcp.Tool{
		Name: "export_csv",
		Description: "Export a PostgreSQL table (or the result of a custom query) to a CSV file. " +
			"Returns the output path and the number of rows written.",
		InputSchema: objectSchema(map[string]any{
			"profile":     str("Saved profile name. Omit to use the active profile."),
			"table":       str("Table to export, e.g. \"users\" or \"myschema.users\". Required unless query is set."),
			"query":       str("Custom SELECT query to export instead of a whole table."),
			"output_file": str("Destination CSV path. Required."),
			"headers":     boolean("Write a header row. Defaults to false."),
			"batch_size":  integer("Rows per batch. Defaults to 500."),
			"overwrite":   boolean("Overwrite the output file if it exists. Defaults to false."),
		}, "output_file"),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in struct {
			Profile    string `json:"profile"`
			Table      string `json:"table"`
			Query      string `json:"query"`
			OutputFile string `json:"output_file"`
			Headers    bool   `json:"headers"`
			BatchSize  int    `json:"batch_size"`
			Overwrite  bool   `json:"overwrite"`
		}
		if err := decodeInput(req.Params.Arguments, &in); err != nil {
			return failResult("invalid_input", err.Error())
		}
		if in.OutputFile == "" {
			return failResult("invalid_input", "output_file is required")
		}
		if in.Table == "" && in.Query == "" {
			return failResult("invalid_input", "either table or query must be provided")
		}
		if in.BatchSize <= 0 {
			in.BatchSize = 500
		}
		if _, err := os.Stat(in.OutputFile); err == nil && !in.Overwrite {
			return failResult("file_exists",
				fmt.Sprintf("output file %q already exists; pass overwrite=true to replace it", in.OutputFile))
		}

		profile, errRes, err := resolveProfile(in.Profile)
		if err != nil {
			return errRes, nil
		}

		conn, err := db.Connect(profile)
		if err != nil {
			return failResult("connection_failed", err.Error())
		}
		defer conn.Close()

		opts := &io.CSVOptions{BatchSize: in.BatchSize, HasHeader: in.Headers}

		result := map[string]any{
			"ok":          true,
			"profile":     profile.Name,
			"output_file": in.OutputFile,
		}

		if in.Query != "" {
			n, err := io.ExportCSVWithQuery(conn.DB, in.Query, in.OutputFile, in.Headers)
			if err != nil {
				return failResult("export_failed", err.Error())
			}
			result["rows"] = n
			result["mode"] = "query"
			return okResult(result)
		}

		table := qualifyTable(in.Table)
		if err := io.ExportCSVWithOptions(conn.DB, table, in.OutputFile, opts); err != nil {
			return failResult("export_failed", err.Error())
		}
		result["table"] = table
		result["mode"] = "table"
		result["rows"] = io.CountCSVDataRows(in.OutputFile, in.Headers)
		return okResult(result)
	})
}

func registerImportCSV(srv *mcp.Server) {
	srv.AddTool(&mcp.Tool{
		Name:        "import_csv",
		Description: "Import a CSV file into a PostgreSQL table. Optionally truncate the table first.",
		InputSchema: objectSchema(map[string]any{
			"profile":    str("Saved profile name. Omit to use the active profile."),
			"table":      str("Destination table, e.g. \"users\" or \"myschema.users\". Required."),
			"input_file": str("Source CSV path. Required."),
			"headers":    boolean("Treat the first row as a header. Defaults to false."),
			"truncate":   boolean("Truncate the table before importing. Defaults to false."),
			"batch_size": integer("Rows per batch. Defaults to 500."),
		}, "table", "input_file"),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in struct {
			Profile   string `json:"profile"`
			Table     string `json:"table"`
			InputFile string `json:"input_file"`
			Headers   bool   `json:"headers"`
			Truncate  bool   `json:"truncate"`
			BatchSize int    `json:"batch_size"`
		}
		if err := decodeInput(req.Params.Arguments, &in); err != nil {
			return failResult("invalid_input", err.Error())
		}
		if in.Table == "" || in.InputFile == "" {
			return failResult("invalid_input", "table and input_file are required")
		}
		if in.BatchSize <= 0 {
			in.BatchSize = 500
		}
		if _, err := os.Stat(in.InputFile); err != nil {
			return failResult("file_not_found", fmt.Sprintf("input file %q not found", in.InputFile))
		}

		profile, errRes, err := resolveProfile(in.Profile)
		if err != nil {
			return errRes, nil
		}

		conn, err := db.Connect(profile)
		if err != nil {
			return failResult("connection_failed", err.Error())
		}
		defer conn.Close()

		table := qualifyTable(in.Table)
		if in.Truncate {
			if _, err := conn.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s", table)); err != nil {
				return failResult("truncate_failed", err.Error())
			}
		}

		opts := &io.CSVOptions{BatchSize: in.BatchSize, HasHeader: in.Headers}
		if err := io.ImportCSVWithOptions(conn.DB, table, in.InputFile, opts); err != nil {
			return failResult("import_failed", err.Error())
		}

		return okResult(map[string]any{
			"ok":         true,
			"profile":    profile.Name,
			"table":      table,
			"input_file": in.InputFile,
			"truncated":  in.Truncate,
		})
	})
}

func registerExportDump(srv *mcp.Server) {
	srv.AddTool(&mcp.Tool{
		Name: "export_dump",
		Description: "Create a pg_dump backup of a database (plain SQL, custom, tar or directory format). " +
			"Supports SSH tunnels via the profile.",
		InputSchema: objectSchema(map[string]any{
			"profile":     str("Saved profile name. Omit to use the active profile."),
			"output_file": str("Destination dump path. Required."),
			"format":      str("Dump format: plain (default), custom, tar or directory."),
			"compress":    boolean("Compress the dump (custom/tar formats)."),
			"schema_only": boolean("Dump the schema without data."),
			"data_only":   boolean("Dump data without the schema."),
			"tables":      str("Comma-separated list of tables to include."),
			"overwrite":   boolean("Overwrite the output file if it exists. Defaults to false."),
		}, "output_file"),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in struct {
			Profile    string `json:"profile"`
			OutputFile string `json:"output_file"`
			Format     string `json:"format"`
			Compress   bool   `json:"compress"`
			SchemaOnly bool   `json:"schema_only"`
			DataOnly   bool   `json:"data_only"`
			Tables     string `json:"tables"`
			Overwrite  bool   `json:"overwrite"`
		}
		if err := decodeInput(req.Params.Arguments, &in); err != nil {
			return failResult("invalid_input", err.Error())
		}
		if in.OutputFile == "" {
			return failResult("invalid_input", "output_file is required")
		}
		if in.SchemaOnly && in.DataOnly {
			return failResult("invalid_input", "schema_only and data_only are mutually exclusive")
		}
		if _, err := os.Stat(in.OutputFile); err == nil && !in.Overwrite {
			return failResult("file_exists",
				fmt.Sprintf("output file %q already exists; pass overwrite=true to replace it", in.OutputFile))
		}

		profile, errRes, err := resolveProfile(in.Profile)
		if err != nil {
			return errRes, nil
		}

		opts := &io.DumpOptions{
			Format:     in.Format,
			Compress:   in.Compress,
			SchemaOnly: in.SchemaOnly,
			DataOnly:   in.DataOnly,
			Tables:     splitCSV(in.Tables),
		}
		if err := io.DumpDatabaseWithConnectionAndOptions(profile, in.OutputFile, opts); err != nil {
			return failResult("dump_failed", err.Error())
		}

		result := map[string]any{
			"ok":          true,
			"profile":     profile.Name,
			"output_file": in.OutputFile,
		}
		if fi, err := os.Stat(in.OutputFile); err == nil {
			result["bytes"] = fi.Size()
		}
		return okResult(result)
	})
}

func registerImportDump(srv *mcp.Server) {
	srv.AddTool(&mcp.Tool{
		Name: "import_dump",
		Description: "Restore a pg_dump backup into a database. Tolerates benign client/server " +
			"version-skew errors by default; set strict=true to fail on any error.",
		InputSchema: objectSchema(map[string]any{
			"profile":    str("Saved profile name. Omit to use the active profile."),
			"input_file": str("Source dump path. Required."),
			"clean":      boolean("Drop existing objects before restoring (pg_restore only)."),
			"strict":     boolean("Fail on the first restore error instead of tolerating version skew."),
			"timeout":    integer("Restore timeout in seconds (0 = none)."),
		}, "input_file"),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in struct {
			Profile   string `json:"profile"`
			InputFile string `json:"input_file"`
			Clean     bool   `json:"clean"`
			Strict    bool   `json:"strict"`
			Timeout   int    `json:"timeout"`
		}
		if err := decodeInput(req.Params.Arguments, &in); err != nil {
			return failResult("invalid_input", err.Error())
		}
		if in.InputFile == "" {
			return failResult("invalid_input", "input_file is required")
		}
		if _, err := os.Stat(in.InputFile); err != nil {
			return failResult("file_not_found", fmt.Sprintf("input file %q not found", in.InputFile))
		}

		profile, errRes, err := resolveProfile(in.Profile)
		if err != nil {
			return errRes, nil
		}

		opts := &io.RestoreOptions{Clean: in.Clean, Strict: in.Strict, Timeout: in.Timeout}
		if err := io.RestoreDatabaseWithConnectionAndOptions(profile, in.InputFile, opts); err != nil {
			return failResult("restore_failed", err.Error())
		}

		return okResult(map[string]any{
			"ok":         true,
			"profile":    profile.Name,
			"input_file": in.InputFile,
			"clean":      in.Clean,
			"strict":     in.Strict,
		})
	})
}

func registerMigrate(srv *mcp.Server) {
	srv.AddTool(&mcp.Tool{
		Name: "migrate_database",
		Description: "Migrate schema and/or data from one database to another. Source and target are " +
			"either two profiles, or one profile with source_database and target_database overrides.",
		InputSchema: objectSchema(map[string]any{
			"source_profile":  str("Source profile name. Required."),
			"target_profile":  str("Target profile name. If omitted, single-profile mode is used with source_database/target_database."),
			"source_database": str("Source database override (single-profile mode)."),
			"target_database": str("Target database override (single-profile mode)."),
			"schema_only":     boolean("Migrate schema without data."),
			"data_only":       boolean("Migrate data without the schema."),
			"tables":          str("Comma-separated list of tables to migrate."),
			"overwrite":       boolean("Overwrite existing objects in the target."),
			"enable_rollback": boolean("Capture a rollback backup before migrating."),
		}, "source_profile"),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in struct {
			SourceProfile  string `json:"source_profile"`
			TargetProfile  string `json:"target_profile"`
			SourceDatabase string `json:"source_database"`
			TargetDatabase string `json:"target_database"`
			SchemaOnly     bool   `json:"schema_only"`
			DataOnly       bool   `json:"data_only"`
			Tables         string `json:"tables"`
			Overwrite      bool   `json:"overwrite"`
			EnableRollback bool   `json:"enable_rollback"`
		}
		if err := decodeInput(req.Params.Arguments, &in); err != nil {
			return failResult("invalid_input", err.Error())
		}
		if in.SchemaOnly && in.DataOnly {
			return failResult("invalid_input", "schema_only and data_only are mutually exclusive")
		}

		source, errRes, err := resolveProfile(in.SourceProfile)
		if err != nil {
			return errRes, nil
		}

		target := source
		if in.TargetProfile != "" && in.TargetProfile != in.SourceProfile {
			target, errRes, err = resolveProfile(in.TargetProfile)
			if err != nil {
				return errRes, nil
			}
		} else {
			// Single-profile mode requires database overrides.
			if in.SourceDatabase == "" || in.TargetDatabase == "" {
				return failResult("invalid_input",
					"provide target_profile, or both source_database and target_database")
			}
			if in.SourceDatabase == in.TargetDatabase {
				return failResult("invalid_input", "source_database and target_database must differ")
			}
			source.Database = in.SourceDatabase
			target.Database = in.TargetDatabase
		}

		opts := &io.MigrationOptions{
			SourceProfile:  source,
			TargetProfile:  target,
			SchemaOnly:     in.SchemaOnly,
			DataOnly:       in.DataOnly,
			Tables:         splitCSV(in.Tables),
			Overwrite:      in.Overwrite,
			EnableRollback: in.EnableRollback,
		}
		if err := io.MigrateDatabaseWithConnection(opts); err != nil {
			return failResult("migration_failed", err.Error())
		}

		return okResult(map[string]any{
			"ok":              true,
			"source":          source.Name,
			"target":          target.Name,
			"source_database": source.Database,
			"target_database": target.Database,
		})
	})
}

// --- small shared helpers --------------------------------------------------

// qualifyTable adds the public schema when no schema is given.
func qualifyTable(table string) string {
	if strings.Contains(table, ".") {
		return table
	}
	return "public." + table
}

// splitCSV splits a comma-separated list, trimming whitespace and dropping
// empty entries.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
