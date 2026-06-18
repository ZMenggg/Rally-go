package proxy

import "testing"

func TestParseUUIDRejectsInvalidHex(t *testing.T) {
	if _, err := parseUUID("zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"); err == nil {
		t.Fatal("parseUUID() accepted invalid hex")
	}
}

func TestParseUUIDRejectsInvalidHyphenPositions(t *testing.T) {
	if _, err := parseUUID("550e8400e29b-41d4-a716-446655440000"); err == nil {
		t.Fatal("parseUUID() accepted invalid hyphen positions")
	}
}

func TestParseUUIDAcceptsCompactHex(t *testing.T) {
	if _, err := parseUUID("550e8400e29b41d4a716446655440000"); err != nil {
		t.Fatalf("parseUUID() error = %v", err)
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{name: "valid", port: "443"},
		{name: "zero", port: "0"},
		{name: "too large", port: "65536", wantErr: true},
		{name: "not numeric", port: "https", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePort(%q) error = %v, wantErr %v", tt.port, err, tt.wantErr)
			}
		})
	}
}

func TestTrojanPasswordHash(t *testing.T) {
	got := trojanPasswordHash("password")
	want := "d63dc919e201d7bc4c825630d2cf25fdc93d4b2f0d46706d29038d01"
	if got != want {
		t.Fatalf("trojanPasswordHash() = %q, want %q", got, want)
	}
}

func TestBasicVLESSRejectsFlow(t *testing.T) {
	_, err := NewVLESSProvider("node", "example.com:443", "550e8400-e29b-41d4-a716-446655440000", "xtls-rprx-vision", "example.com")
	if err == nil {
		t.Fatal("NewVLESSProvider accepted flow in basic mode")
	}
}

func TestBuildXrayVLESSConfigReality(t *testing.T) {
	cfg, err := buildXrayVLESSConfig(VLESSOptions{
		Name:        "node",
		Server:      "example.com:443",
		UUID:        "550e8400-e29b-41d4-a716-446655440000",
		Flow:        "xtls-rprx-vision",
		SNI:         "www.example.com",
		Network:     "tcp",
		Security:    "reality",
		Fingerprint: "chrome",
		PublicKey:   "abc123",
		ShortID:     "deadbeef",
		SpiderX:     "/",
	}, "127.0.0.1:23456")
	if err != nil {
		t.Fatalf("buildXrayVLESSConfig() error = %v", err)
	}
	outbounds := cfg["outbounds"].([]interface{})
	outbound := outbounds[0].(map[string]interface{})
	stream := outbound["streamSettings"].(map[string]interface{})
	if got := stream["security"]; got != "reality" {
		t.Fatalf("security = %v, want reality", got)
	}
	reality := stream["realitySettings"].(map[string]interface{})
	if got := reality["publicKey"]; got != "abc123" {
		t.Fatalf("publicKey = %v, want abc123", got)
	}
}
