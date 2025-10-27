package cmd

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	DbURL      string `yaml:"db_url"`
	SSHHost    string `yaml:"ssh_host"`
	SSHUser    string `yaml:"ssh_user"`
	SSHKeyPath string `yaml:"ssh_key_path"`
	SSHTimeout string `yaml:"ssh_timeout"`
}

type Config struct {
	Profiles map[string]Profile `yaml:"profiles"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pgtransfer_config.yaml")
}

func loadConfig() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return &Config{Profiles: make(map[string]Profile)}, nil
	}
	var cfg Config
	yaml.Unmarshal(data, &cfg)
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	data, _ := yaml.Marshal(cfg)
	return os.WriteFile(configPath(), data, 0644)
}
