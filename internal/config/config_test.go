package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempHome points HOME at a fresh temp dir so config files are isolated.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestLoadConfig_MissingReturnsEmpty(t *testing.T) {
	withTempHome(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Profiles == nil {
		t.Fatalf("expected initialized Profiles map")
	}
	if len(cfg.Profiles) != 0 {
		t.Fatalf("expected no profiles, got %d", len(cfg.Profiles))
	}
}

func TestSaveAndLoadConfig_RoundTrip(t *testing.T) {
	home := withTempHome(t)

	cfg := &ConfigFile{
		ActiveProfile: "local",
		Profiles: map[string]Profile{
			"local": {
				Name:     "local",
				User:     "postgres",
				Password: "secret",
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				SSLMode:  "disable",
			},
		},
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// The file must live under $HOME/.pgtransfer/config.yaml.
	path := filepath.Join(home, ".pgtransfer", "config.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config at %s: %v", path, err)
	}

	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if got.ActiveProfile != "local" {
		t.Fatalf("expected active profile 'local', got %q", got.ActiveProfile)
	}
	p, ok := got.Profiles["local"]
	if !ok {
		t.Fatalf("expected profile 'local' to be loaded")
	}
	if p.Database != "testdb" || p.Port != 5432 {
		t.Fatalf("profile not round-tripped correctly: %+v", p)
	}
}

func TestGetActiveProfile_NoneSet(t *testing.T) {
	withTempHome(t)

	if _, err := GetActiveProfile(); err == nil {
		t.Fatalf("expected error when no active profile is set")
	}
}

func TestSetActiveProfile_Unknown(t *testing.T) {
	withTempHome(t)

	if err := SetActiveProfile("nope"); err == nil {
		t.Fatalf("expected error for unknown profile")
	}
}

func TestAddListDeleteProfile(t *testing.T) {
	withTempHome(t)

	p := Profile{Name: "p1", Host: "h", Port: 5432, Database: "d"}

	// testConnection == nil skips validation.
	if err := AddOrUpdateProfile(p, nil); err != nil {
		t.Fatalf("AddOrUpdateProfile failed: %v", err)
	}

	// First profile becomes active automatically.
	active, err := GetActiveProfile()
	if err != nil {
		t.Fatalf("GetActiveProfile failed: %v", err)
	}
	if active.Name != "p1" {
		t.Fatalf("expected active profile p1, got %q", active.Name)
	}

	cfg, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(cfg.Profiles))
	}

	if err := DeleteProfile("p1"); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}
	cfg, _ = ListProfiles()
	if len(cfg.Profiles) != 0 {
		t.Fatalf("expected 0 profiles after delete, got %d", len(cfg.Profiles))
	}
}

func TestProfileExists(t *testing.T) {
	withTempHome(t)

	if exists, _ := ProfileExists("ghost"); exists {
		t.Fatalf("expected profile to not exist")
	}
	if err := AddOrUpdateProfile(Profile{Name: "real"}, nil); err != nil {
		t.Fatalf("AddOrUpdateProfile failed: %v", err)
	}
	if exists, _ := ProfileExists("real"); !exists {
		t.Fatalf("expected profile to exist")
	}
}

func TestBuildDSN_Defaults(t *testing.T) {
	p := Profile{User: "postgres", Password: "secret", Host: "localhost", Database: "testdb"}
	got := BuildDSN(p)
	if !strings.HasPrefix(got, "postgres://") {
		t.Fatalf("expected postgres:// prefix, got %q", got)
	}
	if !strings.Contains(got, "@localhost:5432/") {
		t.Fatalf("expected default port 5432, got %q", got)
	}
	if !strings.HasSuffix(got, "sslmode=disable") {
		t.Fatalf("expected default sslmode=disable, got %q", got)
	}
}

func TestBuildDSN_DBURLPassthrough(t *testing.T) {
	p := Profile{DBURL: "postgres://u:p@host:6000/db?sslmode=require"}
	if got := BuildDSN(p); got != p.DBURL {
		t.Fatalf("expected DBURL passthrough %q, got %q", p.DBURL, got)
	}
}

func TestBuildDSN_SpecialCharacterEncoding(t *testing.T) {
	p := Profile{
		User:     "user@domain",
		Password: "p@ss:w/rd?x",
		Host:     "db.example.com",
		Port:     5432,
		Database: "my db",
		SSLMode:  "require",
	}
	got := BuildDSN(p)
	if strings.Contains(got, "p@ss:w/rd?x") {
		t.Fatalf("password not encoded, got %q", got)
	}
	if !strings.Contains(got, "user%40domain") {
		t.Fatalf("expected user percent-encoded, got %q", got)
	}
	if !strings.Contains(got, "p%40ss%3Aw%2Frd%3Fx") {
		t.Fatalf("expected password percent-encoded, got %q", got)
	}
	if !strings.Contains(got, "/my%20db") {
		t.Fatalf("expected database path-escaped, got %q", got)
	}
}

func TestBuildDSN_ExplicitPort(t *testing.T) {
	p := Profile{User: "u", Password: "p", Host: "h", Port: 6543, Database: "d"}
	if got := BuildDSN(p); !strings.Contains(got, "@h:6543/d") {
		t.Fatalf("expected explicit port 6543, got %q", got)
	}
}

func TestAddOrUpdateProfile_TestConnectionFailure(t *testing.T) {
	withTempHome(t)

	fail := func(Profile) error { return os.ErrInvalid }
	err := AddOrUpdateProfile(Profile{Name: "bad"}, fail)
	if err == nil {
		t.Fatalf("expected error propagated from testConnection")
	}
	if exists, _ := ProfileExists("bad"); exists {
		t.Fatalf("profile should not be saved when validation fails")
	}
}
