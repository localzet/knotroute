package overlay

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/localzet/knotroute/internal/config"
	proxyserver "github.com/localzet/knotroute/internal/proxy"
)

//go:embed assets/*
var dashboardAssets embed.FS

func (n *Node) startDashboard() error {
	sub, err := fs.Sub(dashboardAssets, "assets")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, n.Status())
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": Version})
	})
	mux.HandleFunc("GET /api/config", n.localManagement(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, n.cfg)
	}))
	mux.HandleFunc("PUT /api/config", n.localManagement(func(w http.ResponseWriter, r *http.Request) {
		if n.cfg.Path == "" {
			http.Error(w, "configuration path is unavailable", http.StatusConflict)
			return
		}
		body := http.MaxBytesReader(w, r.Body, 1<<20)
		decoder := json.NewDecoder(body)
		decoder.DisallowUnknownFields()
		var next config.Config
		if err := decoder.Decode(&next); err != nil {
			http.Error(w, "invalid configuration: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := ensureSingleJSON(decoder); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		next.Normalize()
		if err := next.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.SaveAtomic(n.cfg.Path, next); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		n.addEvent("info", "configuration updated; restart requested")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restart_required": true})
	}))
	mux.HandleFunc("POST /api/reload", n.localManagement(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
		go func() { time.Sleep(150 * time.Millisecond); n.RequestRestart() }()
	}))
	mux.HandleFunc("POST /api/shutdown", n.localManagement(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
		go func() { time.Sleep(150 * time.Millisecond); n.RequestShutdown() }()
	}))
	mux.HandleFunc("GET /proxy.pac", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		_, _ = io.WriteString(w, proxyserver.PAC(n.httpProxyAddress()))
	})
	static := http.FileServer(http.FS(sub))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assets are not content-hashed. Force revalidation so a browser left open
		// across a desktop upgrade cannot keep executing an old dashboard bundle.
		w.Header().Set("Cache-Control", "no-cache, max-age=0")
		static.ServeHTTP(w, r)
	}))
	listener, err := net.Listen("tcp", n.cfg.Dashboard)
	if err != nil {
		return fmt.Errorf("dashboard listen %s: %w", n.cfg.Dashboard, err)
	}
	n.dashboardServer = &http.Server{
		Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		if err := n.dashboardServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			n.addEvent("error", "dashboard: "+err.Error())
		}
	}()
	n.addEvent("info", "dashboard available at http://"+listener.Addr().String())
	return nil
}

func (n *Node) httpProxyAddress() string {
	for _, address := range n.proxyAddresses {
		if strings.HasPrefix(address, "http://") {
			return strings.TrimPrefix(address, "http://")
		}
	}
	return n.cfg.Proxy.HTTP
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func ensureSingleJSON(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	}
	return errors.New("request body must contain exactly one JSON value")
}

func (n *Node) localManagement(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !net.ParseIP(host).IsLoopback() {
			http.Error(w, "management API is local-only", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			allowedHTTP := "http://" + r.Host
			allowedHTTPS := "https://" + r.Host
			if origin != allowedHTTP && origin != allowedHTTPS {
				http.Error(w, "origin rejected", http.StatusForbidden)
				return
			}
		}
		if r.Method != http.MethodGet && r.Header.Get("Content-Type") != "" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "application/json required", http.StatusUnsupportedMediaType)
			return
		}
		next(w, r)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
