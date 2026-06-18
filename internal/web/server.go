package web

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/ZMenggg/Rally-go/internal/config"
	"github.com/ZMenggg/Rally-go/internal/logger"
	"github.com/ZMenggg/Rally-go/internal/proxy"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"

	"time"
)

//go:embed frontend/index.html frontend/style.css frontend/app.js
var frontendFS embed.FS

// Server is the Web UI HTTP server.
type Server struct {
	cfg        *config.Config
	configPath string
	authToken  string
	mu         sync.RWMutex

	runnerStatus func() []BackendStatus
	runnerStats  func() []proxy.RatesSnapshot
	runnerReset  func()

	srv *http.Server
}

func New(cfg *config.Config, configPath string) *Server {
	return &Server{
		cfg:        cfg,
		configPath: configPath,
		authToken:  os.Getenv("RALLY_WEB_TOKEN"),
	}
}

func (s *Server) SetStatusFn(fn func() []BackendStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runnerStatus = fn
}

func (s *Server) SetStatsFn(fn func() []proxy.RatesSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runnerStats = fn
}

func (s *Server) SetResetFn(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runnerReset = fn
}

func (s *Server) UpdateConfig(cfg *config.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/config/raw", s.handleConfigRaw)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/reload", s.handleReload)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/stats/reset", s.handleStatsReset)

	mux.HandleFunc("/api/node/toggle", s.handleNodeToggle)
	mux.HandleFunc("/", s.handleStatic)

	handler := http.Handler(mux)
	if s.authToken == "" && isPublicAddr(addr) {
		return fmt.Errorf("refusing to start unauthenticated Web UI on public address %q; bind to 127.0.0.1 or set RALLY_WEB_TOKEN", addr)
	}
	if s.authToken != "" {
		handler = s.authMiddleware(handler)
	}

	s.srv = &http.Server{Addr: addr, Handler: handler}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("web ui listen: %w", err)
	}

	log.Printf("Web UI listening on http://%s", listener.Addr().String())
	if s.authToken == "" {
		log.Printf("Web UI auth disabled; keep it bound to localhost")
	}
	go s.srv.Serve(listener)
	return nil
}

func (s *Server) Stop() {
	if s.srv != nil {
		s.srv.Close()
	}
}

func isPublicAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return true
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return true
	}
	if host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return true
	}
	return !ip.IsLoopback()
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := authTokenFromRequest(r)
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.authToken)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="rally"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authTokenFromRequest(r *http.Request) string {
	if token := r.Header.Get("X-Rally-Token"); token != "" {
		return token
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	if _, password, ok := r.BasicAuth(); ok {
		return password
	}
	if cookie, err := r.Cookie("rally_token"); err == nil {
		return cookie.Value
	}
	return ""
}

// ─── Static ─────────────────────────────────────────────────────────────────

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		s.serveFile(w, "frontend/index.html", "text/html; charset=utf-8")
		return
	}
	switch r.URL.Path {
	case "/style.css":
		s.serveFile(w, "frontend/style.css", "text/css; charset=utf-8")
	case "/app.js":
		s.serveFile(w, "frontend/app.js", "application/javascript; charset=utf-8")
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveFile(w http.ResponseWriter, path, contentType string) {
	data, err := frontendFS.ReadFile(path)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

// ─── API: GET/PUT /api/config ───────────────────────────────────────────────

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getConfig(w, r)
	case http.MethodPut:
		s.putConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a sanitized copy (mask passwords)
	sanitized := *s.cfg
	sanitized.VPS = make([]config.VPS, len(s.cfg.VPS))
	for i, v := range s.cfg.VPS {
		sanitized.VPS[i] = v
		if v.Password != "" && len(v.Password) > 4 {
			sanitized.VPS[i].Password = v.Password[:1] + "****" + v.Password[len(v.Password)-1:]
		} else if v.Password != "" {
			sanitized.VPS[i].Password = "****"
		}
	}
	writeJSON(w, sanitized)
}

func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if cfg.Bind == "" {
		cfg.Bind = s.cfg.Bind
	}
	if cfg.Balance == "" {
		cfg.Balance = s.cfg.Balance
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = s.cfg.Log.Level
	}
	s.mu.Unlock()

	if err := config.Save(s.configPath, &cfg); err != nil {
		http.Error(w, "Save error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.cfg = &cfg
	s.mu.Unlock()

	logger.Info("Config saved via API (%d VPS backends)", len(cfg.VPS))
	// Auto-reload: send SIGHUP to self
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	writeJSON(w, map[string]string{"status": "ok"})
}

// ─── API: GET/PUT /api/config/raw ───────────────────────────────────────────

func (s *Server) handleConfigRaw(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getConfigRaw(w, r)
	case http.MethodPut:
		s.putConfigRaw(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfigRaw(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		http.Error(w, "Read error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func (s *Server) putConfigRaw(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Read error: "+err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadBytes(data)
	if err != nil {
		http.Error(w, "Invalid config: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := config.SaveBytes(s.configPath, data); err != nil {
		http.Error(w, "Write error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()

	logger.Info("Raw config saved via API (%d VPS backends)", len(cfg.VPS))
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	writeJSON(w, map[string]string{"status": "ok"})
}

// ─── API: POST /api/node/toggle ─────────────────────────────────────────────

func (s *Server) handleNodeToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	for i := range s.cfg.VPS {
		if s.cfg.VPS[i].Name == req.Name {
			enabled := req.Enabled
			s.cfg.VPS[i].Enabled = &enabled
			break
		}
	}
	cfgCopy := *s.cfg
	s.mu.Unlock()

	if err := config.Save(s.configPath, &cfgCopy); err != nil {
		http.Error(w, "Save error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info("Node %s toggled to enabled=%v", req.Name, req.Enabled)
	// Auto-reload: send SIGHUP to self so the Runner picks up the change
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	writeJSON(w, map[string]string{"status": "ok"})
}

// ─── API: GET /api/status ───────────────────────────────────────────────────

type statusResponse struct {
	Backends []BackendStatus `json:"backends"`
	Uptime   string          `json:"uptime"`
}

type BackendStatus struct {
	Enabled   bool   `json:"enabled"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Server    string `json:"server,omitempty"`
	Connected bool   `json:"connected"`
	Active    int64  `json:"active"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := s.cfg
	statusFn := s.runnerStatus
	s.mu.RUnlock()

	if statusFn != nil {
		realBackends := statusFn()
		if len(realBackends) > 0 {
			// Overlay config's enabled state
			nameEnabled := make(map[string]bool)
			configBackends := make(map[string]BackendStatus)
			for _, vps := range cfg.VPS {
				en := true
				if vps.Enabled != nil {
					en = *vps.Enabled
				}
				nameEnabled[vps.Name] = en
				configBackends[vps.Name] = BackendStatus{
					Enabled:   en,
					Name:      vps.Name,
					Type:      vps.Type,
					Server:    fmt.Sprintf("%s:%d", vps.Server, vps.Port),
					Connected: en,
					Active:    0,
				}
			}
			for i := range realBackends {
				if en, ok := nameEnabled[realBackends[i].Name]; ok {
					realBackends[i].Enabled = en
				}
				// Remove from configBackends so we know which ones are only in config
				delete(configBackends, realBackends[i].Name)
			}
			// Append config-only backends (disabled nodes not active in runner)
			for _, cb := range configBackends {
				realBackends = append(realBackends, cb)
			}
			writeJSON(w, statusResponse{
				Backends: realBackends,
				Uptime:   time.Now().Format(time.RFC3339),
			})
			return
		}
	}

	var backends []BackendStatus
	for _, vps := range cfg.VPS {
		enabled := true
		if vps.Enabled != nil {
			enabled = *vps.Enabled
		}
		backends = append(backends, BackendStatus{
			Enabled:   enabled,
			Name:      vps.Name,
			Type:      vps.Type,
			Server:    fmt.Sprintf("%s:%d", vps.Server, vps.Port),
			Connected: enabled,
			Active:    0,
		})
	}

	writeJSON(w, statusResponse{
		Backends: backends,
		Uptime:   time.Now().Format(time.RFC3339),
	})
}

// ─── API: POST /api/reload ──────────────────────────────────────────────────

// ─── API: GET /api/stats ────────────────────────────────────────────────

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	fn := s.runnerStats
	s.mu.RUnlock()

	if fn != nil {
		writeJSON(w, fn())
		return
	}
	writeJSON(w, []proxy.RatesSnapshot{})
}

// ─── API: POST /api/stats/reset ───────────────────────────────────────────

func (s *Server) handleStatsReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.RLock()
	fn := s.runnerReset
	s.mu.RUnlock()

	if fn != nil {
		fn()
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	configPath := s.configPath
	s.mu.RUnlock()

	cfg, err := config.Load(configPath)
	if err != nil {
		http.Error(w, "Reload error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()

	logger.Info("Config reloaded from %s (%d VPS backends)", configPath, len(cfg.VPS))

	// Trigger runner reload via SIGHUP
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)

	writeJSON(w, map[string]string{
		"status":  "ok",
		"message": "Config reloaded.",
	})
}

// ─── API: GET /api/logs ─────────────────────────────────────────────────────

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")

	if mode == "stream" {
		s.handleLogSSE(w, r)
		return
	}

	limit := 200
	entries := logger.G().Recent(limit)
	writeJSON(w, entries)
}

func (s *Server) handleLogSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	recent := logger.G().Recent(50)
	for _, entry := range recent {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	ch := make(chan logger.LogEntry, 100)
	unsub := logger.G().Subscribe(func(entry logger.LogEntry) {
		select {
		case ch <- entry:
		default:
		}
	})
	defer unsub()

	for {
		select {
		case entry := <-ch:
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
