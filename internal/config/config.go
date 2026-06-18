package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	sscore "github.com/shadowsocks/go-shadowsocks2/core"
	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for Rally.
type Config struct {
	Bind    string       `yaml:"bind"    json:"bind"`
	Balance string       `yaml:"balance" json:"balance"`
	Log     LogConfig    `yaml:"log"     json:"log"`
	Health  HealthConfig `yaml:"health,omitempty" json:"health,omitempty"`
	VPS     []VPS        `yaml:"vps"     json:"vps"`
}

type LogConfig struct {
	Level  string `yaml:"level" json:"level"`
	Output string `yaml:"output" json:"output"`
}

type HealthConfig struct {
	Target   string `yaml:"target,omitempty" json:"target,omitempty"`
	Interval int    `yaml:"interval,omitempty" json:"interval,omitempty"`
	MaxFails int    `yaml:"max_fails,omitempty" json:"max_fails,omitempty"`
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
	// Xray-backed VLESS transport/security settings.
	Network     string   `yaml:"network,omitempty" json:"network,omitempty"`
	Security    string   `yaml:"security,omitempty" json:"security,omitempty"`
	Fingerprint string   `yaml:"fingerprint,omitempty" json:"fingerprint,omitempty"`
	PublicKey   string   `yaml:"public_key,omitempty" json:"public_key,omitempty"`
	ShortID     string   `yaml:"short_id,omitempty" json:"short_id,omitempty"`
	SpiderX     string   `yaml:"spider_x,omitempty" json:"spider_x,omitempty"`
	ALPN        []string `yaml:"alpn,omitempty" json:"alpn,omitempty"`
	Path        string   `yaml:"path,omitempty" json:"path,omitempty"`
	Host        string   `yaml:"host,omitempty" json:"host,omitempty"`
	ServiceName string   `yaml:"service_name,omitempty" json:"service_name,omitempty"`
	Mode        string   `yaml:"mode,omitempty" json:"mode,omitempty"`
	XrayPath    string   `yaml:"xray_path,omitempty" json:"xray_path,omitempty"`

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

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.Health.Target != "" {
		if _, _, err := netSplitHostPort(cfg.Health.Target); err != nil {
			return fmt.Errorf("health.target: %w", err)
		}
	}
	if cfg.Health.Interval < 0 {
		return fmt.Errorf("health.interval must be non-negative")
	}
	if cfg.Health.MaxFails < 0 {
		return fmt.Errorf("health.max_fails must be non-negative")
	}
	seen := make(map[string]struct{}, len(cfg.VPS))
	for i, vps := range cfg.VPS {
		if vps.Name == "" {
			return fmt.Errorf("vps[%d]: name is required", i)
		}
		if _, ok := seen[vps.Name]; ok {
			return fmt.Errorf("vps[%d]: duplicate node name %q", i, vps.Name)
		}
		seen[vps.Name] = struct{}{}
		if vps.Server == "" {
			return fmt.Errorf("vps[%d]: server is required", i)
		}
		if vps.Port <= 0 || vps.Port > 65535 {
			return fmt.Errorf("vps[%d]: port must be between 1 and 65535", i)
		}
		switch vps.Type {
		case "", "hysteria2":
			if vps.Password == "" {
				return fmt.Errorf("vps[%d]: password is required for hysteria2", i)
			}
		case "socks5":
		case "ss", "shadowsocks":
			if vps.Password == "" {
				return fmt.Errorf("vps[%d]: password is required for shadowsocks", i)
			}
			if _, err := sscore.PickCipher(defaultShadowSocksCipher(vps.Cipher), nil, vps.Password); err != nil {
				return fmt.Errorf("vps[%d]: invalid shadowsocks cipher: %w", i, err)
			}
		case "trojan":
			if vps.Password == "" {
				return fmt.Errorf("vps[%d]: password is required for trojan", i)
			}
		case "vless":
			if vps.UUID == "" {
				return fmt.Errorf("vps[%d]: uuid is required for vless", i)
			}
			if err := validateUUID(vps.UUID); err != nil {
				return fmt.Errorf("vps[%d]: invalid vless uuid: %w", i, err)
			}
			if err := validateVLESSAdvanced(i, vps); err != nil {
				return err
			}
		default:
			return fmt.Errorf("vps[%d]: unsupported protocol %q", i, vps.Type)
		}
	}
	return nil
}

func (vps VPS) UsesXrayVLESS() bool {
	if vps.Type != "vless" {
		return false
	}
	security := strings.ToLower(vps.Security)
	network := strings.ToLower(vps.Network)
	return vps.Flow != "" ||
		(security != "" && security != "none") ||
		(network != "" && network != "tcp" && network != "raw") ||
		vps.Fingerprint != "" ||
		vps.PublicKey != "" ||
		vps.ShortID != "" ||
		vps.SpiderX != "" ||
		len(vps.ALPN) > 0 ||
		vps.Path != "" ||
		vps.Host != "" ||
		vps.ServiceName != "" ||
		vps.Mode != "" ||
		vps.XrayPath != ""
}

func validateVLESSAdvanced(i int, vps VPS) error {
	network := strings.ToLower(vps.Network)
	switch network {
	case "", "tcp", "raw", "ws", "grpc", "xhttp":
	default:
		return fmt.Errorf("vps[%d]: unsupported vless network %q", i, vps.Network)
	}

	security := strings.ToLower(vps.Security)
	switch security {
	case "", "none", "tls", "reality":
	default:
		return fmt.Errorf("vps[%d]: unsupported vless security %q", i, vps.Security)
	}
	if security == "reality" {
		if vps.PublicKey == "" {
			return fmt.Errorf("vps[%d]: public_key is required for vless reality", i)
		}
		if vps.SNI == "" {
			return fmt.Errorf("vps[%d]: sni is required for vless reality", i)
		}
		if network == "ws" {
			return fmt.Errorf("vps[%d]: vless reality is not supported with ws network", i)
		}
	}
	if security == "tls" && vps.Fingerprint != "" && vps.SNI == "" {
		return fmt.Errorf("vps[%d]: sni is required when vless tls fingerprint is set", i)
	}
	if vps.Flow != "" {
		if network != "" && network != "tcp" && network != "raw" {
			return fmt.Errorf("vps[%d]: vless flow is only supported with tcp/raw network", i)
		}
		if security != "tls" && security != "reality" {
			return fmt.Errorf("vps[%d]: vless flow requires tls or reality security", i)
		}
	}
	return nil
}

// Save writes a Config to a YAML file.
func Save(path string, cfg *Config) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return SaveBytes(path, data)
}

func defaultShadowSocksCipher(cipher string) string {
	if cipher == "" {
		return "AEAD_CHACHA20_POLY1305"
	}
	return cipher
}

func validateUUID(s string) error {
	hex := s
	if len(s) == 36 {
		if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
			return fmt.Errorf("invalid UUID format")
		}
		hex = strings.ReplaceAll(s, "-", "")
	}
	if len(hex) != 32 {
		return fmt.Errorf("invalid UUID length: %d", len(s))
	}
	for i := 0; i < 32; i++ {
		if !isHex(hex[i]) {
			return fmt.Errorf("invalid UUID hex at position %d", i)
		}
	}
	return nil
}

func isHex(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func netSplitHostPort(addr string) (string, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", err
	}
	if host == "" || port == "" {
		return "", "", fmt.Errorf("must be host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n <= 0 || n > 65535 {
		return "", "", fmt.Errorf("port must be between 1 and 65535")
	}
	return host, port, nil
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
