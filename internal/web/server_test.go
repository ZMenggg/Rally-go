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
