package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/overlay"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = initCommand(os.Args[2:])
	case "run":
		err = runCommand(os.Args[2:])
	case "id":
		err = idCommand(os.Args[2:])
	case "doctor":
		err = doctorCommand(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("KnotRoute %s (%s/%s)\n", overlay.Version, runtime.GOOS, runtime.GOARCH)
	case "help", "--help", "-h":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Printf(`KnotRoute %s — encrypted multi-hop service overlay

Usage:
  knotroute init   [--config knotroute.json] [--force]
  knotroute run    [--config knotroute.json]
  knotroute id     [--config knotroute.json]
  knotroute doctor [--config knotroute.json] [--probe]
  knotroute version

Start with "knotroute init", edit the generated JSON, then run the node.
`, overlay.Version)
}

func initCommand(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	path := fs.String("config", "knotroute.json", "configuration file")
	force := fs.Bool("force", false, "replace an existing configuration and identity")
	listen := fs.String("listen", "0.0.0.0:7447", "overlay listener")
	dashboard := fs.String("dashboard", "127.0.0.1:8484", "dashboard listener, empty to disable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	abs, err := filepath.Abs(*path)
	if err != nil {
		return err
	}
	if !*force {
		if _, err := os.Stat(abs); err == nil {
			return fmt.Errorf("%s already exists (use --force to replace it)", abs)
		}
	}
	cfg := config.Default()
	cfg.IdentityFile = "identity.json"
	cfg.Listen = []string{*listen}
	cfg.Dashboard = *dashboard
	identityPath := filepath.Join(filepath.Dir(abs), cfg.IdentityFile)
	if !*force {
		if _, err := os.Stat(identityPath); err == nil {
			return fmt.Errorf("%s already exists (use --force to replace it)", identityPath)
		}
	}
	id, err := identity.Generate()
	if err != nil {
		return err
	}
	if err := id.Save(identityPath); err != nil {
		return err
	}
	if err := config.Save(abs, cfg); err != nil {
		return err
	}
	fmt.Println("Created", abs)
	fmt.Println("Identity", id.ID.String())
	fmt.Println("Edit peers, services and forwards, then run:")
	fmt.Printf("  knotroute run --config %q\n", abs)
	return nil
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	path := fs.String("config", "knotroute.json", "configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, id, err := loadNodeFiles(*path)
	if err != nil {
		return err
	}
	node, err := overlay.New(cfg, id)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := node.Start(ctx); err != nil {
		return err
	}
	fmt.Println("KnotRoute", overlay.Version)
	fmt.Println("Node:", id.ID.String())
	fmt.Println("Overlay listeners:")
	for _, address := range node.Addresses() {
		fmt.Println("  ", address)
	}
	if cfg.Dashboard != "" {
		fmt.Println("Dashboard: http://" + cfg.Dashboard)
	}
	fmt.Println("Press Ctrl+C to stop.")
	<-ctx.Done()
	node.Stop()
	return nil
}

func idCommand(args []string) error {
	fs := flag.NewFlagSet("id", flag.ContinueOnError)
	path := fs.String("config", "knotroute.json", "configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, id, err := loadNodeFiles(*path)
	if err != nil {
		return err
	}
	fmt.Println(id.ID.String())
	return nil
}

func doctorCommand(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	path := fs.String("config", "knotroute.json", "configuration file")
	probe := fs.Bool("probe", false, "try each configured local service target")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, id, err := loadNodeFiles(*path)
	if err != nil {
		return err
	}
	fmt.Println("[ok] configuration is valid")
	fmt.Println("[ok] identity:", id.ID.String())
	if info, statErr := os.Stat(cfg.IdentityFile); statErr == nil && runtime.GOOS != "windows" {
		if info.Mode().Perm()&0o077 != 0 {
			fmt.Printf("[warn] identity permissions are %o; recommended 600\n", info.Mode().Perm())
		} else {
			fmt.Println("[ok] identity permissions are restricted")
		}
	}
	for _, address := range cfg.Listen {
		listener, listenErr := net.Listen("tcp", address)
		if listenErr != nil {
			fmt.Printf("[fail] cannot bind overlay listener %s: %v\n", address, listenErr)
			return errors.New("one or more overlay listeners are unavailable")
		}
		_ = listener.Close()
		fmt.Println("[ok] overlay listener available:", address)
	}
	if cfg.Dashboard != "" {
		listener, listenErr := net.Listen("tcp", cfg.Dashboard)
		if listenErr != nil {
			fmt.Printf("[fail] cannot bind dashboard %s: %v\n", cfg.Dashboard, listenErr)
			return errors.New("dashboard listener is unavailable")
		}
		_ = listener.Close()
		fmt.Println("[ok] dashboard listener available:", cfg.Dashboard)
	}
	if *probe {
		dialer := net.Dialer{Timeout: 2 * time.Second}
		for _, service := range cfg.Services {
			conn, dialErr := dialer.Dial("tcp", service.Target)
			if dialErr != nil {
				fmt.Printf("[warn] service %s target %s is unavailable: %v\n", service.Name, service.Target, dialErr)
				continue
			}
			_ = conn.Close()
			fmt.Printf("[ok] service %s target is reachable\n", service.Name)
		}
	}
	fmt.Printf("[ok] %d seed peers, %d services, %d forwards\n", len(cfg.Peers), len(cfg.Services), len(cfg.Forwards))
	return nil
}

func loadNodeFiles(path string) (config.Config, *identity.Identity, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, nil, err
	}
	id, err := identity.Load(cfg.IdentityFile)
	if err != nil {
		return config.Config{}, nil, err
	}
	return cfg, id, nil
}
