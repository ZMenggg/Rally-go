package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for Rally.
type Config struct {
	Bind    string    `yaml:"bind"    json:"bind"`
	Balance string    `yaml:"balance" json:"balance"`
	Log     LogConfig `yaml:"log"     json:"log"`
	VPS     []VPS     `yaml:"vps"     json:"vps"`
}

type LogConfig struct {
	Level  string `yaml:"level" json:"level"`
	Output string `yaml:"output" json:"output"`
}

// VPS represents a single VPS backend.
type VPS struct {
	Name     string `yaml:"name"     json:"name"`
	Type     string `yaml:"type"     json:"type"`
	Server   string `yaml:"server"   json:"server"`
	Port     int    `yaml:"port"     json:"port"`
	Password string `yaml:"password" json:"password"`
	SNI      string `yaml:"sni,omitempty"     json:"sni,omitempty"`

	// Cipher method for Shadowsocks (e.g. "AEAD_CHACHA20_POLY1305")
	Cipher string `yaml:"cipher,omitempty"   json:"cipher,omitempty"`

	// UUID for VLESS protocol
	UUID string `yaml:"uuid,omitempty" json:"uuid,omitempty"`
	// Flow control for VLESS (e.g. "xtls-rprx-vision")
	Flow string `yaml:"flow,omitempty" json:"flow,omitempty"`

	// Insecure disables TLS certificate verification (for self-signed certs)
	Insecure bool `yaml:"insecure,omitempty" json:"insecure,omitempty"`

	// Bandwidth in Mbps (optional, Hysteria2 specific)
	DownMbps int `yaml:"down_mbps,omitempty" json:"down_mbps,omitempty"`
	UpMbps   int `yaml:"up_mbps,omitempty"   json:"up_mbps,omitempty"`

	// Health check
	HealthTimeout int `yaml:"health_timeout,omitempty" json:"health_timeout,omitempty"`

	// Enabled controls whether this node participates in bandwidth aggregation.
	// Defaults to true if not set.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// Load reads and parses a YAML config file from disk.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return LoadBytes(data)
}

// LoadBytes parses a YAML config from raw bytes and applies defaults.
func LoadBytes(data []byte) (*Config, error) {
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
	return SaveBytes(path, data)
}

// SaveBytes writes raw validated config bytes to disk using the same
// permissions and atomic replacement as Save.
func SaveBytes(path string, data []byte) error {
	return writeFileAtomic(path, data, 0600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".rally-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	cleanup = false
	return os.Chmod(path, perm)
}
