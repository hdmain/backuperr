package client

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is loaded from YAML for the backup client.
type Config struct {
	URL        string   `yaml:"url"`
	APIKey     string   `yaml:"api_key"`
	BackupRoot string   `yaml:"backup_root"`
	RestoreTo  string   `yaml:"restore_to"`
	StatePath  string   `yaml:"state_path"`
	TempDir    string   `yaml:"temp_dir"`
	Exclude    []string `yaml:"exclude"`
	WebhookURL string   `yaml:"webhook_url"`
}

// BaseURL returns the origin for API requests (e.g. http://127.0.0.1:8443).
func (c *Config) BaseURL() (string, error) {
	if c.URL == "" {
		return "", fmt.Errorf("config: url is required")
	}
	u, err := url.Parse(strings.TrimSpace(c.URL))
	if err != nil {
		return "", fmt.Errorf("config: invalid url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("config: url must include scheme and host, e.g. http://127.0.0.1:8443")
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

// LoadConfig reads and parses a YAML client configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.BackupRoot = strings.TrimSpace(cfg.BackupRoot)
	cfg.RestoreTo = strings.TrimSpace(cfg.RestoreTo)
	cfg.StatePath = strings.TrimSpace(cfg.StatePath)
	cfg.TempDir = strings.TrimSpace(cfg.TempDir)
	cfg.WebhookURL = strings.TrimSpace(cfg.WebhookURL)
	if cfg.StatePath == "" && cfg.BackupRoot != "" {
		cfg.StatePath = filepath.Join(cfg.BackupRoot, ".backuperr-state.json")
	}
	if cfg.RestoreTo == "" && cfg.BackupRoot != "" {
		cfg.RestoreTo = cfg.BackupRoot
	}
	return &cfg, nil
}
