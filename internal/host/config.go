package host

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is loaded from host config YAML.
type Config struct {
	Listen      string `yaml:"listen"`
	DataDir     string `yaml:"data_dir"`
	MainKey     string `yaml:"main_key"`
	WebhookURL  string `yaml:"webhook_url"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Listen = strings.TrimSpace(cfg.Listen)
	cfg.DataDir = strings.TrimSpace(cfg.DataDir)
	cfg.MainKey = strings.TrimSpace(cfg.MainKey)
	cfg.WebhookURL = strings.TrimSpace(cfg.WebhookURL)
	return &cfg, nil
}
