package overlay

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(n.Status())
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.Handle("/", http.FileServer(http.FS(sub)))
	listener, err := net.Listen("tcp", n.cfg.Dashboard)
	if err != nil {
		return fmt.Errorf("dashboard listen %s: %w", n.cfg.Dashboard, err)
	}
	n.dashboardServer = &http.Server{
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
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

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
