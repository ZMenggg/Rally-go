package cli

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/ZMenggg/Rally-go/internal/config"
	"github.com/ZMenggg/Rally-go/internal/logger"
	"github.com/ZMenggg/Rally-go/internal/proxy"
	"github.com/ZMenggg/Rally-go/internal/runner"
	"github.com/ZMenggg/Rally-go/internal/web"
)

// App is the CLI application.
type App struct{}

// NewApp creates a new CLI app.
func NewApp() *App {
	return &App{}
}

// Run parses args and executes the appropriate command.
func (a *App) Run(args []string) error {
	if len(args) < 2 {
		return a.printUsage()
	}

	cmd := args[1]
	switch cmd {
	case "run":
		return a.runServer(args[2:])
	case "web":
		return a.runWeb(args[2:])
	case "check":
		return a.check(args[2:])
	case "list":
		return a.listBackends(args[2:])
	case "reload":
		return a.reload(args[2:])
	case "version":
		fmt.Println("Rally v0.1.0")
		return nil
	default:
		return a.printUsage()
	}
}

func (a *App) resolveConfig(args []string) (string, error) {
	for i, arg := range args {
		if arg == "--config" || arg == "-c" {
			if i+1 < len(args) {
				return args[i+1], nil
			}
			return "", fmt.Errorf("--config requires a path")
		}
	}
	for _, p := range []string{"./rally.yaml", "/etc/rally.yaml"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no config found (try --config or place rally.yaml in current dir)")
}

func (a *App) parseFlags(args []string) (rest []string, flags map[string]string) {
	flags = make(map[string]string)
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if len(args[i]) > 2 && args[i][:2] == "--" {
			key := args[i][2:]
			if i+1 < len(args) && args[i+1][0] != '-' {
				flags[key] = args[i+1]
				i++
			} else {
				flags[key] = "true"
			}
		} else {
			rest = append(rest, args[i])
		}
	}
	return
}

func (a *App) printUsage() error {
	fmt.Print(`Rally — Multi-VPS bandwidth aggregation proxy

Usage:
  rally run [--config rally.yaml] [--web [addr]]   Start the proxy server
  rally web [--config rally.yaml] [--addr 127.0.0.1:9090] Start Web UI only
  rally check [--config rally.yaml]                  Validate configuration
  rally list [--config rally.yaml]                   List backends and health
  rally reload [--config rally.yaml]                 Reload configuration (sends SIGHUP)
  rally version                                      Print version

Examples:
  rally run -c ./rally.yaml
  rally run -c ./rally.yaml --web :9090
  rally web --addr :9090
  rally check
`)
	return nil
}

// ─── run ─────────────────────────────────────────────────────────────────────

func (a *App) runServer(args []string) error {
	rest, flags := a.parseFlags(args)
	configPath, err := a.resolveConfig(rest)
	if err != nil {
		return err
	}

	// Load initial config
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Init logger
	logFile := cfg.Log.Output
	if err := logger.Init(cfg.Log.Level, logFile); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	// Start Web UI if --web flag is set
	var ws *web.Server
	if webAddr, ok := flags["web"]; ok {
		addr := webAddr
		if addr == "true" {
			addr = "127.0.0.1:9090"
		}
		ws = web.New(cfg, configPath)
		if err := ws.Start(addr); err != nil {
			return fmt.Errorf("web ui: %w", err)
		}
		defer ws.Stop()
	}

	r := runner.New(cfg)

	// Handle SIGHUP for hot reload
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	writePIDFile("/tmp/rally.pid")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("SIGHUP handler panicked: %v", r)
			}
		}()
		for range sigCh {
			logger.Info("Received SIGHUP, reloading config...")
			newCfg, err := config.Load(configPath)
			if err != nil {
				logger.Error("Reload config failed: %v", err)
				continue
			}
			if ws != nil {
				ws.UpdateConfig(newCfg)
			}
			if err := r.ReloadConfig(newCfg); err != nil {
				logger.Error("Reload failed: %v", err)
				continue
			}
			logger.Info("Config reloaded successfully")
		}
	}()

	// Pass real status to Web UI
	if ws != nil {
		ws.SetStatusFn(func() []web.BackendStatus {
			b := r.Balancer()
			if b == nil {
				return nil
			}
			currentCfg, err := config.Load(configPath)
			if err != nil {
				logger.Warn("status config refresh failed: %v", err)
				currentCfg = cfg
			}
			nameToVPS := make(map[string]config.VPS)
			for _, v := range currentCfg.VPS {
				nameToVPS[v.Name] = v
			}
			info := b.Info()
			var out []web.BackendStatus
			for _, be := range info {
				vps := nameToVPS[be.Name]
				out = append(out, web.BackendStatus{
					Name:      be.Name,
					Type:      vps.Type,
					Server:    net.JoinHostPort(vps.Server, strconv.Itoa(vps.Port)),
					Enabled:   true,
					Connected: be.Connected,
					Active:    be.Active,
				})
			}
			return out
		})
		ws.SetStatsFn(func() []proxy.RatesSnapshot {
			f := r.Forwarder()
			if f == nil {
				return nil
			}
			return f.AllStats()
		})
		ws.SetResetFn(func() {
			f := r.Forwarder()
			if f == nil {
				return
			}
			f.ResetStats()
		})
	}
	err = r.Run()
	defer r.Close()
	if err != nil {
		logger.Error("Server stopped: %v", err)
		return err
	}
	// SIGHUP graceful reload — ReloadConfig goroutine keeps the server alive.
	// Block here so main() doesn't exit.
	select {}
}

// ─── web ─────────────────────────────────────────────────────────────────────

func (a *App) runWeb(args []string) error {
	configPath, err := a.resolveConfig(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logFile := cfg.Log.Output
	if err := logger.Init(cfg.Log.Level, logFile); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	addr := "127.0.0.1:9090"
	for i, arg := range args {
		if arg == "--addr" && i+1 < len(args) {
			addr = args[i+1]
		}
	}

	logger.Info("Rally Web UI starting, config=%s", configPath)
	ws := web.New(cfg, configPath)
	if err := ws.Start(addr); err != nil {
		return fmt.Errorf("web ui: %w", err)
	}

	// Handle SIGHUP
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)
	writePIDFile("/tmp/rally.pid")
	go func() {
		for range sigCh {
			logger.Info("Received SIGHUP, reloading config...")
			newCfg, err := config.Load(configPath)
			if err != nil {
				logger.Error("Reload config failed: %v", err)
				continue
			}
			ws.UpdateConfig(newCfg)
			logger.Info("Config reloaded successfully")
		}
	}()

	select {}
}

func writePIDFile(path string) {
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d", os.Getpid())), 0600); err != nil {
		logger.Warn("write PID file failed: %v", err)
	}
}

// ─── check ───────────────────────────────────────────────────────────────────

func (a *App) check(args []string) error {
	configPath, err := a.resolveConfig(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	fmt.Printf("Config OK: %d VPS backend(s), bind=%s, balance=%s\n",
		len(cfg.VPS), cfg.Bind, cfg.Balance)
	for _, v := range cfg.VPS {
		fmt.Printf("  - %s (%s:%d, type=%s)\n", v.Name, v.Server, v.Port, v.Type)
	}
	return nil
}

// ─── list ────────────────────────────────────────────────────────────────────

func (a *App) listBackends(args []string) error {
	configPath, err := a.resolveConfig(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	fmt.Println("Backends:")
	for _, v := range cfg.VPS {
		fmt.Printf("  - %s\t%s:%d\t%s\n", v.Name, v.Server, v.Port, v.Type)
	}
	return nil
}

// ─── reload ──────────────────────────────────────────────────────────────────

func (a *App) reload(args []string) error {
	// Send SIGHUP to running rally process
	// Find PID from a lock file or use killall
	pidFile := "/tmp/rally.pid"
	if data, err := os.ReadFile(pidFile); err == nil {
		var pid int
		fmt.Sscanf(string(data), "%d", &pid)
		if p, err := os.FindProcess(pid); err == nil {
			return p.Signal(syscall.SIGHUP)
		}
	}
	// Fallback: try looking for rally process
	fmt.Println("Looking for running rally process...")
	return fmt.Errorf("could not find running rally process. Try: killall -HUP rally")
}
