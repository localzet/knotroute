package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/localzet/knotroute/internal/certauth"
	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/invite"
	"github.com/localzet/knotroute/internal/naming"
	"github.com/localzet/knotroute/internal/networkid"
	"github.com/localzet/knotroute/internal/overlay"
	"github.com/localzet/knotroute/internal/serviceidentity"
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
	case "address":
		err = addressCommand(os.Args[2:])
	case "resolve":
		err = resolveCommand(os.Args[2:])
	case "alias":
		err = aliasCommand(os.Args[2:])
	case "doctor":
		err = doctorCommand(os.Args[2:])
	case "ca":
		err = caCommand(os.Args[2:])
	case "network":
		err = networkCommand(os.Args[2:])
	case "invite":
		err = inviteCommand(os.Args[2:])
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
  knotroute address [--config knotroute.json] [--service name]
  knotroute resolve [--config knotroute.json] <name.knot>
  knotroute alias export --config knotroute.json --name localzet [--out localzet.knot-alias.json]
  knotroute alias import --config knotroute.json --file localzet.knot-alias.json
  knotroute doctor [--config knotroute.json] [--probe]
  knotroute ca init|path|fingerprint|info|install|uninstall|rotate [--config knotroute.json]
  knotroute network create
  knotroute invite export|import [--config knotroute.json]
  knotroute version

Start with "knotroute init", edit the generated JSON, then run the node.
`, overlay.Version)
}

func networkCommand(args []string) error {
	if len(args) != 1 || args[0] != "create" {
		return errors.New("network expects create")
	}
	id, err := networkid.Random()
	if err != nil {
		return err
	}
	fmt.Println(id.String())
	return nil
}

func inviteCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("invite expects export or import")
	}
	switch args[0] {
	case "export":
		fs := flag.NewFlagSet("invite export", flag.ContinueOnError)
		path := fs.String("config", "knotroute.json", "configuration file")
		out := fs.String("out", "network.knotinvite", "output invite file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		cfg, id, err := loadNodeFiles(*path)
		if err != nil {
			return err
		}
		network, err := cfg.Network()
		if err != nil {
			return err
		}
		record, err := invite.New(id, network, cfg.Discovery.Beacons, cfg.Peers, time.Now())
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*out, append(raw, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Println("Created", *out)
		return nil
	case "import":
		fs := flag.NewFlagSet("invite import", flag.ContinueOnError)
		path := fs.String("config", "knotroute.json", "configuration file")
		file := fs.String("file", "", "invite file")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *file == "" {
			return errors.New("--file is required")
		}
		raw, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		var record invite.Invite
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		network, err := record.Verify(time.Now())
		if err != nil {
			return err
		}
		cfg, err := config.Load(*path)
		if err != nil {
			return err
		}
		cfg.NetworkID = network.String()
		cfg.Discovery.Beacons = append([]string(nil), record.Beacons...)
		cfg.Peers = append([]config.Peer(nil), record.Peers...)
		cfg.Path = ""
		if err := config.SaveAtomic(*path, cfg); err != nil {
			return err
		}
		fmt.Println("Imported network", network.String())
		return nil
	default:
		return fmt.Errorf("unknown invite command %q", args[0])
	}
}

func caCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("ca expects init, path, fingerprint, info, install, uninstall, or rotate")
	}
	action := args[0]
	fs := flag.NewFlagSet("ca "+action, flag.ContinueOnError)
	path := fs.String("config", "knotroute.json", "configuration file")
	yes := fs.Bool("yes", false, "confirm destructive CA rotation")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}
	if !cfg.CA.Enabled {
		return errors.New("local CA is disabled in the configuration")
	}
	profile := certauth.Profile{
		ValidityDays: cfg.CA.ValidityDays,
		Subject: certauth.Subject{
			CommonName: cfg.CA.Subject.CommonName, Organization: append([]string(nil), cfg.CA.Subject.Organization...),
			OrganizationalUnit: append([]string(nil), cfg.CA.Subject.OrganizationalUnit...), Country: append([]string(nil), cfg.CA.Subject.Country...),
			Province: append([]string(nil), cfg.CA.Subject.Province...), Locality: append([]string(nil), cfg.CA.Subject.Locality...),
			StreetAddress: append([]string(nil), cfg.CA.Subject.StreetAddress...), PostalCode: append([]string(nil), cfg.CA.Subject.PostalCode...),
		},
	}
	if action == "rotate" {
		if !*yes {
			return errors.New("ca rotate replaces the root private key; pass --yes after updating ca.subject in the configuration")
		}
		if current, loadErr := certauth.LoadOrCreateWithProfile(cfg.CA.Directory, profile); loadErr == nil {
			_ = certauth.UninstallUserRoot(current)
		}
		authority, rotateErr := certauth.Regenerate(cfg.CA.Directory, profile)
		if rotateErr != nil {
			return rotateErr
		}
		info := authority.Info()
		fmt.Println("Rotated", info.RootPath)
		fmt.Println("Subject:", info.Subject)
		fmt.Println("SHA-256:", info.Fingerprint)
		fmt.Println("Reinstall this root on every client that should trust .knot HTTPS.")
		return nil
	}
	authority, err := certauth.LoadOrCreateWithProfile(cfg.CA.Directory, profile)
	if err != nil {
		return err
	}
	switch action {
	case "init":
		fmt.Println(authority.RootPath())
		return nil
	case "path":
		fmt.Println(authority.RootPath())
		return nil
	case "fingerprint":
		fmt.Println(authority.Fingerprint())
		return nil
	case "info":
		info := authority.Info()
		fmt.Println("Path:", info.RootPath)
		fmt.Println("Subject:", info.Subject)
		fmt.Println("Issuer:", info.Issuer)
		fmt.Println("Serial:", info.Serial)
		fmt.Println("SHA-256:", info.Fingerprint)
		fmt.Println("Not before:", info.NotBefore.Format(time.RFC3339))
		fmt.Println("Not after:", info.NotAfter.Format(time.RFC3339))
		return nil
	case "install":
		if err := certauth.InstallUserRoot(authority); err != nil {
			return err
		}
		fmt.Println("Installed", authority.RootPath())
		return nil
	case "uninstall":
		if err := certauth.UninstallUserRoot(authority); err != nil {
			return err
		}
		fmt.Println("Removed", authority.Fingerprint())
		return nil
	default:
		return fmt.Errorf("unknown ca command %q", action)
	}
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for {
		cfg, id, err := loadNodeFiles(*path)
		if err != nil {
			return err
		}
		node, err := overlay.New(cfg, id)
		if err != nil {
			return err
		}
		if err := node.Start(ctx); err != nil {
			return err
		}
		fmt.Println("KnotRoute", overlay.Version)
		fmt.Println("Node:", id.ID.String())
		fmt.Println("Address:", node.Domain())
		fmt.Println("Overlay listeners:")
		for _, address := range node.Addresses() {
			fmt.Println("  ", address)
		}
		if cfg.Proxy.SOCKS != "" {
			fmt.Println("SOCKS5: socks5://" + cfg.Proxy.SOCKS)
		}
		if cfg.Proxy.HTTP != "" {
			fmt.Println("HTTP proxy: http://" + cfg.Proxy.HTTP)
		}
		if cfg.Dashboard != "" {
			fmt.Println("Dashboard: http://" + cfg.Dashboard)
		}
		fmt.Println("Press Ctrl+C to stop.")
		select {
		case <-ctx.Done():
			node.Stop()
			return nil
		case <-node.ShutdownRequested():
			node.Stop()
			return nil
		case <-node.RestartRequested():
			node.Stop()
			fmt.Println("Reloading configuration...")
		}
	}
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

func addressCommand(args []string) error {
	fs := flag.NewFlagSet("address", flag.ContinueOnError)
	path := fs.String("config", "knotroute.json", "configuration file")
	service := fs.String("service", "", "optional service name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, id, err := loadNodeFiles(*path)
	if err != nil {
		return err
	}
	if *service == "" {
		fmt.Println(naming.CanonicalDomain(id.ID))
		return nil
	}
	for _, svc := range cfg.Services {
		if svc.Name != *service {
			continue
		}
		if svc.Publish {
			identityPath := svc.IdentityFile
			if identityPath == "" {
				identityPath = filepath.Join(filepath.Dir(cfg.Path), "services", svc.Name+".identity.json")
			}
			sid, err := serviceidentity.LoadOrCreate(identityPath)
			if err != nil {
				return err
			}
			fmt.Println(naming.ServiceCanonicalDomain(sid.ID))
			return nil
		}
		break
	}
	domain, err := naming.ServiceDomain(*service, id.ID)
	if err != nil {
		return err
	}
	fmt.Println(domain)
	return nil
}

func resolveCommand(args []string) error {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	path := fs.String("config", "knotroute.json", "configuration file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("resolve expects exactly one .knot name")
	}
	cfg, _, err := loadNodeFiles(*path)
	if err != nil {
		return err
	}
	resolved, err := naming.ResolveHost(fs.Arg(0), cfg.Aliases)
	if err != nil {
		return err
	}
	if resolved.Kind == naming.AddressService {
		fmt.Printf("kind=service\nservice_id=%s\ncanonical=%t\n", resolved.ServiceID.String(), resolved.Canonical)
	} else {
		fmt.Printf("kind=node\nnode=%s\nservice=%s\ncanonical=%t\n", resolved.Node.String(), resolved.Service, resolved.Canonical)
	}
	return nil
}

func aliasCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("alias expects export or import")
	}
	switch args[0] {
	case "export":
		fs := flag.NewFlagSet("alias export", flag.ContinueOnError)
		path := fs.String("config", "knotroute.json", "configuration file")
		name := fs.String("name", "", "alias name")
		description := fs.String("description", "", "description")
		out := fs.String("out", "", "output file; stdout when empty")
		validity := fs.Duration("valid-for", 365*24*time.Hour, "record validity; 0 means no expiry")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*name) == "" {
			return errors.New("--name is required")
		}
		_, id, err := loadNodeFiles(*path)
		if err != nil {
			return err
		}
		record, err := naming.SignAliasRecord(id, *name, *description, time.Now(), *validity)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if *out == "" {
			_, err = os.Stdout.Write(data)
			return err
		}
		return os.WriteFile(*out, data, 0o644)
	case "import":
		fs := flag.NewFlagSet("alias import", flag.ContinueOnError)
		path := fs.String("config", "knotroute.json", "configuration file")
		file := fs.String("file", "", "signed alias record")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *file == "" {
			return errors.New("--file is required")
		}
		data, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		var record naming.AliasRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return err
		}
		alias, err := record.Verify(time.Now())
		if err != nil {
			return err
		}
		cfg, err := config.Load(*path)
		if err != nil {
			return err
		}
		replaced := false
		for i := range cfg.Aliases {
			if strings.EqualFold(cfg.Aliases[i].Name, alias.Name) {
				cfg.Aliases[i] = alias
				replaced = true
				break
			}
		}
		if !replaced {
			cfg.Aliases = append(cfg.Aliases, alias)
		}
		cfg.Path = ""
		if err := config.SaveAtomic(*path, cfg); err != nil {
			return err
		}
		fmt.Printf("Imported %s.knot -> %s\n", alias.Name, alias.Node)
		return nil
	default:
		return fmt.Errorf("unknown alias command %q", args[0])
	}
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
