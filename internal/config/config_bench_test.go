package config

import (
	"os"
	"path/filepath"

	"github.com/andymarthin/pgtransfer/internal/utils"
	"testing"
)

// Profiles are read on every command, so DSN construction and config loading
// are on the startup path of every invocation. These benchmarks guard that
// path against regressions.

func benchProfile() Profile {
	return Profile{
		Name:     "production",
		Host:     "db.example.com",
		Port:     5432,
		User:     "app_user",
		Password: "s3cr3t-pa55word",
		Database: "appdb",
		SSLMode:  "require",
	}
}

func BenchmarkBuildDSN(b *testing.B) {
	p := benchProfile()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = BuildDSN(p)
	}
}

// BenchmarkBuildDSNWithSpecialCharacters covers the escaping path, which is
// slower than the plain case because of url.QueryEscape.
func BenchmarkBuildDSNWithSpecialCharacters(b *testing.B) {
	p := benchProfile()
	p.User = "user@corp"
	p.Password = "p@ss:w/rd?x=1&y=2"
	p.Database = "app db/name"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = BuildDSN(p)
	}
}

// BenchmarkBuildDSNURL covers the short-circuit when a full URL is configured.
func BenchmarkBuildDSNURL(b *testing.B) {
	p := benchProfile()
	p.DBURL = "postgres://app_user:pw@db.example.com:5432/appdb?sslmode=require"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = BuildDSN(p)
	}
}

// BenchmarkLoadConfig measures reading and parsing a config file with a
// representative number of profiles.
func BenchmarkLoadConfig(b *testing.B) {
	for _, profiles := range []int{1, 10, 50} {
		b.Run(profileCount(profiles), func(b *testing.B) {
			dir := b.TempDir()
			// GetConfigPath resolves under $HOME, so point HOME at the temp dir
			// for the duration of the benchmark.
			b.Setenv("HOME", dir)

			cfg := &ConfigFile{Profiles: map[string]Profile{}}
			for i := 0; i < profiles; i++ {
				p := benchProfile()
				name := "profile-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
				p.Name = name
				cfg.Profiles[name] = p
			}
			path := utils.GetConfigPath()
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				b.Fatal(err)
			}
			if err := SaveConfig(cfg); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got, err := LoadConfig()
				if err != nil {
					b.Fatal(err)
				}
				if len(got.Profiles) != profiles {
					b.Fatalf("got %d profiles, want %d", len(got.Profiles), profiles)
				}
			}
		})
	}
}

// BenchmarkProfileExists measures the lookup performed before writing, which
// re-reads the config file each time.
func BenchmarkProfileExists(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("HOME", dir)

	cfg := &ConfigFile{Profiles: map[string]Profile{"production": benchProfile()}}
	path := utils.GetConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		b.Fatal(err)
	}
	if err := SaveConfig(cfg); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ProfileExists("production"); err != nil {
			b.Fatal(err)
		}
	}
}

func profileCount(n int) string {
	switch n {
	case 1:
		return "1_profile"
	case 10:
		return "10_profiles"
	case 50:
		return "50_profiles"
	}
	return "profiles"
}
