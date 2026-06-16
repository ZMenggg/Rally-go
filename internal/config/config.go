package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for Rally.
type Config struct {
	Bind    string     `yaml:"bind"`
	Balance string     `yaml:"balance"`
	Log     LogConfig  `yaml:"log"`
	VPS     []VPS      `yaml:"vps"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
}

// VPS represents a single VPS backend.
type VPS struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	SNI      string `yaml:"sni,omitempty"`

	// Bandwidth in Mbps (optional, Hysteria2 specific)
	DownMbps int `yaml:"down_mbps,omitempty"`
	UpMbps   int `yaml:"up_mbps,omitempty"`

	// Health check
	HealthTimeout int `yaml:"health_timeout,omitempty"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply defaults
	if cfg.Bind == "" {
		cfg.Bind = ":1080"
	}
	if cfg.Balance == "" {
		cfg.Balance = "roundrobin"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}

	return &cfg, nil
}

// Save writes a Config to a YAML file.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
