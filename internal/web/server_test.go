package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ZMenggg/Rally-go/internal/config"
)

func TestIsPublicAddr(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:9090", false},
		{"localhost:9090", false},
		{":9090", true},
		{"0.0.0.0:9090", true},
		{"192.0.2.10:9090", true},
		{"[::1]:9090", false},
		{"[::]:9090", true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			if got := isPublicAddr(tt.addr); got != tt.want {
				t.Fatalf("isPublicAddr(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}

func TestStartRejectsPublicAddrWithoutToken(t *testing.T) {
	s := New(&config.Config{}, "")
	if err := s.Start("0.0.0.0:0"); err == nil {
		s.Stop()
		t.Fatal("Start() succeeded on public address without token")
	}
}

func TestStartConfiguresHTTPTimeouts(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	if srv.ReadTimeout != webReadTimeout {
		t.Fatalf("ReadTimeout = %v, want %v", srv.ReadTimeout, webReadTimeout)
	}
	if srv.IdleTimeout != webIdleTimeout {
		t.Fatalf("IdleTimeout = %v, want %v", srv.IdleTimeout, webIdleTimeout)
	}
}

func TestAuthMiddlewareAcceptsBasicPassword(t *testing.T) {
	s := New(&config.Config{}, "")
	s.authToken = "secret"
	handler := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("rally", "secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestMergeMaskedSecretsKeepsExistingPassword(t *testing.T) {
	oldCfg := &config.Config{VPS: []config.VPS{{
		Name:     "node-1",
		Password: "supersecret",
	}}}
	next := &config.Config{VPS: []config.VPS{{
		Name:     "node-1",
		Password: "s****t",
	}}}

	mergeMaskedSecrets(next, oldCfg)

	if got := next.VPS[0].Password; got != "supersecret" {
		t.Fatalf("password = %q, want original", got)
	}
}

func TestMergeMaskedSecretsDoesNotCopyPasswordByIndex(t *testing.T) {
	oldCfg := &config.Config{VPS: []config.VPS{{
		Name:     "old-name",
		Password: "supersecret",
	}}}
	next := &config.Config{VPS: []config.VPS{{
		Name: "new-name",
	}}}

	mergeMaskedSecrets(next, oldCfg)

	if got := next.VPS[0].Password; got != "" {
		t.Fatalf("password = %q, want empty", got)
	}
}

func TestCheckWriteRequestRejectsCrossSiteOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9090/api/reload", nil)
	req.Host = "127.0.0.1:9090"
	req.Header.Set("Origin", "http://evil.example")

	if err := checkWriteRequest(req); err == nil {
		t.Fatal("checkWriteRequest accepted cross-site origin")
	}
}

func TestCheckWriteRequestAcceptsSameOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9090/api/reload", nil)
	req.Host = "127.0.0.1:9090"
	req.Header.Set("Origin", "http://127.0.0.1:9090")

	if err := checkWriteRequest(req); err != nil {
		t.Fatalf("checkWriteRequest error = %v", err)
	}
}

func TestCheckWriteRequestRejectsBasicAuthWithoutOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9090/api/reload", nil)
	req.SetBasicAuth("rally", "secret")

	if err := checkWriteRequest(req); err == nil {
		t.Fatal("checkWriteRequest accepted Basic auth write without origin")
	}
}

func TestCheckWriteRequestAcceptsBearerWithoutOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9090/api/reload", nil)
	req.Header.Set("Authorization", "Bearer secret")

	if err := checkWriteRequest(req); err != nil {
		t.Fatalf("checkWriteRequest error = %v", err)
	}
}
