package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMigrateDatabase_ExposesStrict pins that the MCP tool offers `strict` and
// that it is documented in the input schema. The CLI's --strict reaches
// pg_restore; a tool that omitted it silently gave agents a weaker migration
// than the command it mirrors.
func TestMigrateDatabase_ExposesStrict(t *testing.T) {
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "migrate_database", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected an error for missing required arguments, got success")
	}

	// The tool list is where the schema is visible; read it from there.
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	var found bool
	for _, tool := range tools.Tools {
		if tool.Name != "migrate_database" {
			continue
		}
		found = true
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Fatalf("migrate_database input schema is %T, want an object", tool.InputSchema)
		}
		props, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("migrate_database schema has no properties")
		}
		if _, ok := props["strict"]; !ok {
			t.Errorf("migrate_database schema has no strict property: %v", props)
		}
	}
	if !found {
		t.Fatalf("migrate_database tool is not registered")
	}
}

// TestMigrateDatabase_AcceptsStrict pins that passing strict is not rejected as
// an unknown argument before the tool body runs. Both booleans travel together
// here so a schema/decode mismatch shows up as a single failure.
func TestMigrateDatabase_AcceptsStrict(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "migrate_database",
		Arguments: map[string]any{
			"source_profile": "alpha",
			"target_profile": "beta",
			"overwrite":      true,
			"strict":         true,
		},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}

	// The profiles point at host-a / host-b, which do not resolve, so the call
	// fails at connect time. What matters is that it got that far rather than
	// rejecting `strict` as an unknown argument.
	if !res.IsError {
		t.Fatalf("expected a connection failure, got success")
	}
	data := decodeData(t, res)
	errObj, _ := data["error"].(map[string]any)
	if code, _ := errObj["code"].(string); code != "migration_failed" {
		t.Fatalf("error code = %q, want migration_failed (got %v)", code, data)
	}
}
