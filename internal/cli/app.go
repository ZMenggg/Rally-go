package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/ZMenggg/Rally/internal/config"
	"github.com/ZMenggg/Rally/internal/runner"
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
	// Check --config flag
	for i, arg := range args {
		if arg == "--config" || arg == "-c" {
			if i+1 < len(args) {
				return args[i+1], nil
			}
			return "", fmt.Errorf("--config requires a path")
		}
	}
	// Search default paths
	for _, p := range []string{"./rally.yaml", "/etc/rally.yaml"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no config found (try --config or place rally.yaml in current dir)")
}

func (a *App) printUsage() error {
	fmt.Println(`Rally — Multi-VPS bandwidth aggregation proxy

Usage:
  rally run [--config rally.yaml]     Start the proxy server
  rally check [--config rally.yaml]   Validate configuration
  rally list [--config rally.yaml]    List backends and health
  rally reload [--config rally.yaml]  Reload configuration
  rally version                       Print version

Examples:
  rally run -c ./rally.yaml
  rally check
  rally list
`)
	return nil
}

func (a *App) runServer(args []string) error {
	configPath, err := a.resolveConfig(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log.Printf("Rally starting with %d VPS backend(s)", len(cfg.VPS))
	r := runner.New(cfg)
	return r.Run()
}

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

func (a *App) reload(args []string) error {
	// TODO: send SIGHUP to running process
	fmt.Println("Reload not yet implemented in v0.1.0, use: docker restart rally")
	return nil
}
