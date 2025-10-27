package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/andymarthin/pgtransfer/internal/utils"
	"gopkg.in/yaml.v3"
)

type SSHConfig struct {
	Enabled    bool   `yaml:"enabled"`
	User       string `yaml:"user,omitempty"`
	Host       string `yaml:"host,omitempty"`
	Port       int    `yaml:"port,omitempty"`
	KeyPath    string `yaml:"key_path,omitempty"`
	Passphrase string `yaml:"passphrase,omitempty"`
	Password   string `yaml:"password,omitempty"`
	Timeout    int    `yaml:"timeout,omitempty"`
}

type Profile struct {
	Name     string    `yaml:"name"`
	User     string    `yaml:"user,omitempty"`
	Password string    `yaml:"password,omitempty"`
	Host     string    `yaml:"host,omitempty"`
	Port     int       `yaml:"port,omitempty"`
	Database string    `yaml:"database,omitempty"`
	SSLMode  string    `yaml:"sslmode,omitempty"`
	DBURL    string    `yaml:"dburl,omitempty"`
	SSH      SSHConfig `yaml:"ssh,omitempty"`
}

type ConfigFile struct {
	ActiveProfile string             `yaml:"active_profile,omitempty"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

// LoadConfig reads ~/.pgtransfer/config.yaml or initializes a new one.
func LoadConfig() (*ConfigFile, error) {
	configPath := utils.GetConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &ConfigFile{Profiles: make(map[string]Profile)}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	return &cfg, nil
}

// SaveConfig writes the YAML configuration file to disk.
func SaveConfig(cfg *ConfigFile) error {
	configDir := utils.GetConfigDir()
	configPath := utils.GetConfigPath()

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode YAML: %w", err)
	}

	return os.WriteFile(configPath, data, 0600)
}

func GetActiveProfile() (Profile, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Profile{}, err
	}

	if cfg.ActiveProfile == "" {
		return Profile{}, errors.New("no active profile set; use `pgtransfer profile use <name>`")
	}

	profile, ok := cfg.Profiles[cfg.ActiveProfile]
	if !ok {
		return Profile{}, fmt.Errorf("active profile '%s' not found", cfg.ActiveProfile)
	}
	return profile, nil
}

func SetActiveProfile(name string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	if _, exists := cfg.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' does not exist", name)
	}

	cfg.ActiveProfile = name
	return SaveConfig(cfg)
}

func AddOrUpdateProfile(p Profile, testConnection func(Profile) error) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	if existing, exists := cfg.Profiles[p.Name]; exists {
		fmt.Printf("⚠️  Profile '%s' already exists. Overwriting...\n", existing.Name)
	}

	if testConnection != nil {
		if err := testConnection(p); err != nil {
			return fmt.Errorf("connection validation failed: %v", err)
		}
	}

	cfg.Profiles[p.Name] = p
	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = p.Name
	}

	return SaveConfig(cfg)
}

func DeleteProfile(name string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	if _, exists := cfg.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' does not exist", name)
	}

	delete(cfg.Profiles, name)
	if cfg.ActiveProfile == name {
		cfg.ActiveProfile = ""
	}

	return SaveConfig(cfg)
}

func ListProfiles() (*ConfigFile, error) {
	return LoadConfig()
}

func BuildDSN(p Profile) string {
	if p.DBURL != "" {
		return p.DBURL
	}

	port := p.Port
	if port == 0 {
		port = 5432
	}

	ssl := "disable"
	if p.SSLMode != "" {
		ssl = p.SSLMode
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User,
		p.Password,
		p.Host,
		port,
		p.Database,
		ssl,
	)
}

func ProfileExists(name string) (bool, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return false, err
	}
	_, exists := cfg.Profiles[name]
	return exists, nil
}
