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
