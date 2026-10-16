package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newTestSession wires a client to a fresh in-memory server session.
func newTestSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	server := New()
	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return cs
}

// writeConfig points HOME at a temp dir holding the given profiles YAML.
func writeConfig(t *testing.T, yaml string) {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".pgtransfer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".pgtransfer", "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
}

const twoProfiles = `
active_profile: alpha
profiles:
  alpha:
    name: alpha
    host: host-a
    port: 5432
    user: alice
    password: secret
    database: db_a
  beta:
    name: beta
    host: host-b
    port: 5433
    user: bob
    password: hunter2
    database: db_b
    ssh:
      enabled: true
      user: tunnel
      host: jump
      port: 22
`

// textOf returns the concatenated text content of a result.
func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// decodeData unmarshals the JSON text content of a successful result.
func decodeData(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(textOf(t, res)), &out); err != nil {
		t.Fatalf("result content is not valid JSON: %v\n%s", err, textOf(t, res))
	}
	return out
}

// decodeError extracts the structured error from a failed result.
func decodeError(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	data := decodeData(t, res)
	errObj, ok := data["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected an error object, got %v", data)
	}
	return errObj
}

func TestListTools(t *testing.T) {
	cs := newTestSession(t)

	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{
		"list_profiles":    false,
		"test_connection":  false,
		"export_csv":       false,
		"import_csv":       false,
		"export_dump":      false,
		"import_dump":      false,
		"migrate_database": false,
	}
	for _, tool := range res.Tools {
		if _, ok := want[tool.Name]; !ok {
			t.Errorf("unexpected tool %q", tool.Name)
			continue
		}
		want[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %q has no description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q has no input schema", tool.Name)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %q is not registered", name)
		}
	}
	if len(res.Tools) != len(want) {
		t.Errorf("expected %d tools, got %d", len(want), len(res.Tools))
	}
}

func TestListProfiles_NoPasswords(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_profiles",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, res))
	}

	raw := textOf(t, res)
	for _, secret := range []string{"secret", "hunter2"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("password %q leaked into tool output:\n%s", secret, raw)
		}
	}

	data := decodeData(t, res)
	if data["active_profile"] != "alpha" {
		t.Errorf("expected active profile alpha, got %v", data["active_profile"])
	}
	if data["count"].(float64) != 2 {
		t.Errorf("expected 2 profiles, got %v", data["count"])
	}

	profiles := data["profiles"].([]any)
	var alpha map[string]any
	for _, p := range profiles {
		m := p.(map[string]any)
		if m["name"] == "alpha" {
			alpha = m
		}
	}
	if alpha == nil {
		t.Fatal("alpha profile missing from output")
	}
	if alpha["has_password"] != true {
		t.Errorf("expected has_password=true for alpha, got %v", alpha["has_password"])
	}
	if alpha["active"] != true {
		t.Errorf("expected alpha active=true, got %v", alpha["active"])
	}
}

func TestListProfiles_EmptyConfig(t *testing.T) {
	writeConfig(t, "profiles: {}\n")
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_profiles",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", textOf(t, res))
	}
	data := decodeData(t, res)
	if data["count"].(float64) != 0 {
		t.Errorf("expected 0 profiles, got %v", data["count"])
	}
}

func TestUnknownProfile_IsToolError(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "test_connection",
		Arguments: map[string]any{"profile": "ghost"},
	})
	if err != nil {
		// A lookup failure must be a tool-level error the model can read, not
		// a protocol-level error.
		t.Fatalf("expected a tool result, got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true, got false: %s", textOf(t, res))
	}
	if code := decodeError(t, res)["code"]; code != "profile_not_found" {
		t.Errorf("expected code profile_not_found, got %v", code)
	}
}

func TestNoActiveProfile_IsToolError(t *testing.T) {
	writeConfig(t, "profiles: {}\n")
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "test_connection",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("expected a tool result, got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true, got false: %s", textOf(t, res))
	}
	if code := decodeError(t, res)["code"]; code != "no_active_profile" {
		t.Errorf("expected code no_active_profile, got %v", code)
	}
}

func TestExportCSV_RequiresOutputFile(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "export_csv",
		Arguments: map[string]any{"profile": "alpha"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true, got false")
	}
	if code := decodeError(t, res)["code"]; code != "invalid_input" {
		t.Errorf("expected invalid_input, got %v", code)
	}
}

func TestExportCSV_RequiresTableOrQuery(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "export_csv",
		Arguments: map[string]any{"profile": "alpha", "output_file": "/tmp/out.csv"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for missing table/query")
	}
	if msg, _ := decodeError(t, res)["message"].(string); !strings.Contains(msg, "table or query") {
		t.Errorf("unexpected message: %v", msg)
	}
}

func TestExportCSV_RefusesToOverwrite(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	dir := t.TempDir()
	out := filepath.Join(dir, "exists.csv")
	if err := os.WriteFile(out, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "export_csv",
		Arguments: map[string]any{
			"profile":     "alpha",
			"table":       "users",
			"output_file": out,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true when the output file exists")
	}
	if code := decodeError(t, res)["code"]; code != "file_exists" {
		t.Errorf("expected file_exists, got %v", code)
	}

	// The existing file must be untouched.
	got, _ := os.ReadFile(out)
	if string(got) != "existing" {
		t.Errorf("existing file was modified: %q", got)
	}
}

func TestImportCSV_MissingFile(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "import_csv",
		Arguments: map[string]any{
			"profile":    "alpha",
			"table":      "users",
			"input_file": filepath.Join(t.TempDir(), "missing.csv"),
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for a missing input file")
	}
	if code := decodeError(t, res)["code"]; code != "file_not_found" {
		t.Errorf("expected file_not_found, got %v", code)
	}
}

func TestExportDump_MissingOutputFile(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "export_dump",
		Arguments: map[string]any{"profile": "alpha"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for a missing output_file")
	}
	if code := decodeError(t, res)["code"]; code != "invalid_input" {
		t.Errorf("expected invalid_input, got %v", code)
	}
}

func TestExportDump_RejectsSchemaAndDataOnly(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "export_dump",
		Arguments: map[string]any{
			"profile":     "alpha",
			"output_file": filepath.Join(t.TempDir(), "d.sql"),
			"schema_only": true,
			"data_only":   true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for schema_only+data_only")
	}
}

func TestImportDump_MissingFile(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "import_dump",
		Arguments: map[string]any{"profile": "alpha", "input_file": "/nonexistent/x.sql"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for a missing input file")
	}
	if code := decodeError(t, res)["code"]; code != "file_not_found" {
		t.Errorf("expected file_not_found, got %v", code)
	}
}

func TestMigrate_RequiresTarget(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "migrate_database",
		Arguments: map[string]any{"source_profile": "alpha"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true without a target")
	}
	if code := decodeError(t, res)["code"]; code != "invalid_input" {
		t.Errorf("expected invalid_input, got %v", code)
	}
}

func TestMigrate_RejectsSameDatabase(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "migrate_database",
		Arguments: map[string]any{
			"source_profile":  "alpha",
			"source_database": "same",
			"target_database": "same",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for identical databases")
	}
}

func TestMigrate_RejectsSchemaAndDataOnly(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "migrate_database",
		Arguments: map[string]any{
			"source_profile": "alpha",
			"target_profile": "beta",
			"schema_only":    true,
			"data_only":      true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for schema_only+data_only")
	}
}

func TestMigrate_UnknownSourceProfile(t *testing.T) {
	writeConfig(t, twoProfiles)
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "migrate_database",
		Arguments: map[string]any{
			"source_profile": "nope",
			"target_profile": "beta",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for an unknown source profile")
	}
	if code := decodeError(t, res)["code"]; code != "profile_not_found" {
		t.Errorf("expected profile_not_found, got %v", code)
	}
}

func TestSchemaHelpers(t *testing.T) {
	s := objectSchema(map[string]any{"a": str("desc")}, "a")
	if s["type"] != "object" {
		t.Errorf("expected type object, got %v", s["type"])
	}
	required, ok := s["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "a" {
		t.Errorf("unexpected required: %v", s["required"])
	}
	if objectSchema(map[string]any{})["required"] != nil {
		t.Errorf("expected no required key when none supplied")
	}
}

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{" a , b ,,c ", []string{"a", "b", "c"}},
	}
	for _, c := range cases {
		got := splitCSV(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitCSV(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitCSV(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestQualifyTable(t *testing.T) {
	if got := qualifyTable("users"); got != "public.users" {
		t.Errorf("qualifyTable(users) = %q, want public.users", got)
	}
	if got := qualifyTable("app.users"); got != "app.users" {
		t.Errorf("qualifyTable(app.users) = %q, want app.users", got)
	}
}

func TestDecodeInput_Empty(t *testing.T) {
	var v struct {
		A string `json:"a"`
	}
	if err := decodeInput(nil, &v); err != nil {
		t.Fatalf("decodeInput(nil) returned error: %v", err)
	}
	if err := decodeInput(json.RawMessage(`{"a":"x"}`), &v); err != nil {
		t.Fatalf("decodeInput returned error: %v", err)
	}
	if v.A != "x" {
		t.Errorf("expected a=x, got %q", v.A)
	}
}

func TestFailResult_Shape(t *testing.T) {
	res, err := failResult("some_code", "some message")
	if err != nil {
		t.Fatalf("failResult returned an error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true")
	}
	payload, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured content, got %T", res.StructuredContent)
	}
	if payload["ok"] != false {
		t.Errorf("expected ok=false, got %v", payload["ok"])
	}
	if code := decodeError(t, res)["code"]; code != "some_code" {
		t.Errorf("expected some_code, got %v", code)
	}
}

func TestOkResult_Shape(t *testing.T) {
	res, err := okResult(map[string]any{"value": 7})
	if err != nil {
		t.Fatalf("okResult returned an error: %v", err)
	}
	if res.IsError {
		t.Fatal("expected IsError=false")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected one content item, got %d", len(res.Content))
	}
}
