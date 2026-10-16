package mcpserver

import (
	"context"
	"fmt"

	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerProfileTools adds the read-only profile/connection tools.
func registerProfileTools(srv *mcp.Server) {
	// list_profiles ---------------------------------------------------------
	srv.AddTool(&mcp.Tool{
		Name:        "list_profiles",
		Description: "List all saved pgtransfer connection profiles, including which one is active. Passwords are never included.",
		InputSchema: objectSchema(map[string]any{}),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cfg, err := loadConfig()
		if err != nil {
			return failResult("config_load_failed", err.Error())
		}
		profiles := make([]map[string]any, 0, len(cfg.Profiles))
		for name, p := range cfg.Profiles {
			profiles = append(profiles, map[string]any{
				"name":         name,
				"user":         p.User,
				"host":         p.Host,
				"port":         p.Port,
				"database":     p.Database,
				"has_password": p.Password != "",
				"ssh_enabled":  p.SSH.Enabled,
				"active":       cfg.ActiveProfile == name,
			})
		}
		return okResult(map[string]any{
			"ok":             true,
			"active_profile": cfg.ActiveProfile,
			"count":          len(profiles),
			"profiles":       profiles,
		})
	})

	// test_connection -------------------------------------------------------
	srv.AddTool(&mcp.Tool{
		Name: "test_connection",
		Description: "Test connectivity to a database profile, optionally through an SSH tunnel. " +
			"Returns whether the connection succeeded.",
		InputSchema: objectSchema(map[string]any{
			"profile": str("Name of the saved profile to test. Omit to use the active profile."),
		}),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in struct {
			Profile string `json:"profile"`
		}
		if err := decodeInput(req.Params.Arguments, &in); err != nil {
			return failResult("invalid_input", err.Error())
		}

		profile, errRes, err := resolveProfile(in.Profile)
		if err != nil {
			return errRes, nil
		}

		if err := db.TestConnection(profile); err != nil {
			return failResult("connection_failed",
				fmt.Sprintf("connection test failed for profile %q: %v", profile.Name, err))
		}
		return okResult(map[string]any{
			"ok":       true,
			"profile":  profile.Name,
			"host":     profile.Host,
			"port":     profile.Port,
			"database": profile.Database,
			"ssh":      profile.SSH.Enabled,
		})
	})
}
