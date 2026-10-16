// Package mcpserver exposes pgtransfer functionality as Model Context Protocol
// (MCP) tools, so an AI host (Claude Desktop, Cursor, an agent runtime, …) can
// drive database transfers as native typed tools.
//
// The server speaks MCP over stdio and is started by hosts as a subprocess:
//
//	pgtransfer mcp
//
// Each tool returns a structured JSON payload (also rendered as text content so
// human-readable clients remain useful).
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is the server implementation version reported during initialisation.
// It is set from the CLI version at startup.
var Version = "dev"

// New builds an MCP server with every pgtransfer tool registered.
func New() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:        "pgtransfer",
		Title:       "PGTransfer",
		Description: "Transfer PostgreSQL data between databases or CSV files.",
		Version:     Version,
	}, nil)

	registerProfileTools(srv)
	registerTransferTools(srv)
	return srv
}

// Run serves MCP over stdio until the context is cancelled or stdin closes.
//
// The real stdout must be passed in because the process-level os.Stdout is
// redirected to stderr for the duration of the server (see
// output.EnableMCPMode) to keep stray library output off the protocol stream.
func Run(ctx context.Context, stdout io.Writer) error {
	return New().Run(ctx, &mcp.IOTransport{
		Reader: os.Stdin,
		Writer: nopWriteCloser{stdout},
	})
}

// nopWriteCloser adapts an io.Writer to the io.WriteCloser the transport wants.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// --- helpers ---------------------------------------------------------------

// objectSchema is a small helper for building JSON Schema objects.
func objectSchema(properties map[string]any, required ...string) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// str is a JSON Schema string property.
func str(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

// integer is a JSON Schema integer property.
func integer(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

// boolean is a JSON Schema boolean property.
func boolean(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

// decodeInput unmarshals raw MCP arguments into v.
func decodeInput(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}

// okResult renders a successful tool call.
func okResult(payload any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return failResult("marshal_failed", err.Error())
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(data)}},
		StructuredContent: payload,
	}, nil
}

// failResult renders a tool-level error. Per MCP guidance, tool errors are
// reported in the content with IsError set so the model can see and react to
// them, rather than as a protocol-level error.
func failResult(code, message string) (*mcp.CallToolResult, error) {
	payload := map[string]any{
		"ok":    false,
		"error": map[string]string{"code": code, "message": message},
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return &mcp.CallToolResult{
		IsError:           true,
		Content:           []mcp.Content{&mcp.TextContent{Text: string(data)}},
		StructuredContent: payload,
	}, nil
}

// loadConfig is a thin indirection over config.LoadConfig so the MCP layer has
// a single, mockable entry point.
func loadConfig() (*config.ConfigFile, error) {
	return config.LoadConfig()
}

// resolveProfile returns the named profile, or the active profile when name is
// empty. On failure it returns a populated *mcp.CallToolResult (a tool-level
// error) together with a non-nil sentinel error; callers must stop and return
// that result whenever the sentinel is returned.
func resolveProfile(name string) (config.Profile, *mcp.CallToolResult, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		res, _ := failResult("config_load_failed", fmt.Sprintf("failed to load config: %v", err))
		return config.Profile{}, res, errProfileResolution
	}

	if name == "" {
		p, err := config.GetActiveProfile()
		if err != nil {
			res, _ := failResult("no_active_profile", err.Error())
			return config.Profile{}, res, errProfileResolution
		}
		return p, nil, nil
	}

	p, ok := cfg.Profiles[name]
	if !ok {
		res, _ := failResult("profile_not_found", fmt.Sprintf("profile %q not found", name))
		return config.Profile{}, res, errProfileResolution
	}
	return p, nil, nil
}

// errProfileResolution signals that resolveProfile produced a tool-level error
// result the caller must return. It is deliberately non-nil so callers can
// never mistake a failed lookup for a successful one.
var errProfileResolution = errors.New("profile resolution failed")
