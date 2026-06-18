package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rally.yaml")
	cfg := &Config{
		Bind:    ":1080",
		Balance: "roundrobin",
		VPS: []VPS{{
			Name:     "node-1",
			Server:   "example.com",
			Port:     443,
			Password: "secret",
		}},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("config permissions = %v, want 0600", got)
	}
}

func TestLoadBytesRejectsInvalidVPS(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "duplicate names",
			yaml: `
vps:
  - name: node
    server: example.com
    port: 443
  - name: node
    server: example.org
    port: 443
`,
		},
		{
			name: "invalid port",
			yaml: `
vps:
  - name: node
    server: example.com
    port: 70000
`,
		},
		{
			name: "unsupported protocol",
			yaml: `
vps:
  - name: node
    type: unknown
    server: example.com
    port: 443
`,
		},
		{
			name: "missing hysteria password",
			yaml: `
vps:
  - name: node
    type: hysteria2
    server: example.com
    port: 443
`,
		},
		{
			name: "missing shadowsocks password",
			yaml: `
vps:
  - name: node
    type: ss
    server: example.com
    port: 443
`,
		},
		{
			name: "invalid shadowsocks cipher",
			yaml: `
vps:
  - name: node
    type: ss
    server: example.com
    port: 443
    password: secret
    cipher: invalid-cipher
`,
		},
		{
			name: "missing trojan password",
			yaml: `
vps:
  - name: node
    type: trojan
    server: example.com
    port: 443
`,
		},
		{
			name: "missing vless uuid",
			yaml: `
vps:
  - name: node
    type: vless
    server: example.com
    port: 443
`,
		},
		{
			name: "vless reality missing public key",
			yaml: `
vps:
  - name: node
    type: vless
    server: example.com
    port: 443
    uuid: 550e8400-e29b-41d4-a716-446655440000
    security: reality
    sni: www.example.com
`,
		},
		{
			name: "vless reality with websocket",
			yaml: `
vps:
  - name: node
    type: vless
    server: example.com
    port: 443
    uuid: 550e8400-e29b-41d4-a716-446655440000
    security: reality
    network: ws
    sni: www.example.com
    public_key: abc123
`,
		},
		{
			name: "vless flow without tls",
			yaml: `
vps:
  - name: node
    type: vless
    server: example.com
    port: 443
    uuid: 550e8400-e29b-41d4-a716-446655440000
    flow: xtls-rprx-vision
`,
		},
		{
			name: "invalid health target",
			yaml: `
health:
  target: example.com
vps:
  - name: node
    server: example.com
    port: 443
    password: secret
`,
		},
		{
			name: "invalid health target port",
			yaml: `
health:
  target: example.com:70000
vps:
  - name: node
    server: example.com
    port: 443
    password: secret
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := LoadBytes([]byte(tt.yaml)); err == nil {
				t.Fatal("LoadBytes() succeeded, want error")
			}
		})
	}
}

func TestSaveRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rally.yaml")
	cfg := &Config{VPS: []VPS{{
		Name:   "node",
		Server: "example.com",
		Port:   70000,
	}}}

	if err := Save(path, cfg); err == nil {
		t.Fatal("Save() succeeded, want validation error")
	}
}

func TestLoadBytesAcceptsAdvancedVLESS(t *testing.T) {
	data := []byte(`
vps:
  - name: node
    type: vless
    server: example.com
    port: 443
    uuid: 550e8400-e29b-41d4-a716-446655440000
    flow: xtls-rprx-vision
    security: reality
    sni: www.example.com
    public_key: abc123
    short_id: deadbeef
    fingerprint: chrome
`)

	cfg, err := LoadBytes(data)
	if err != nil {
		t.Fatalf("LoadBytes() error = %v", err)
	}
	if !cfg.VPS[0].UsesXrayVLESS() {
		t.Fatal("advanced VLESS config did not select Xray-backed provider")
	}
}
