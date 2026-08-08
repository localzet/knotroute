package transport

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/config"
)

type Attempt struct {
	Name      string        `json:"name"`
	Type      string        `json:"type"`
	Target    string        `json:"target"`
	Duration  time.Duration `json:"duration"`
	Succeeded bool          `json:"succeeded"`
	Error     string        `json:"error,omitempty"`
	At        time.Time     `json:"at"`
}

type Status struct {
	Mode         string    `json:"mode"`
	LastSelected string    `json:"last_selected,omitempty"`
	Attempts     []Attempt `json:"attempts"`
}

type Manager struct {
	cfg config.Transport
	mu  sync.RWMutex
	st  Status
}

func New(cfg config.Transport) *Manager {
	cfg.Normalize()
	return &Manager{cfg: cfg, st: Status{Mode: cfg.Mode, Attempts: []Attempt{}}}
}

func (m *Manager) DialContext(ctx context.Context, target string) (net.Conn, error) {
	plans := m.plans()
	if len(plans) == 0 {
		return nil, errors.New("no KnotRoute transport is enabled")
	}
	var errs []string
	for _, plan := range plans {
		started := time.Now()
		conn, err := plan.dial(ctx, target)
		m.record(Attempt{Name: plan.name, Type: plan.kind, Target: target, Duration: time.Since(started), Succeeded: err == nil, Error: errorText(err), At: time.Now().UTC()})
		if err == nil {
			m.mu.Lock()
			m.st.LastSelected = plan.name
			m.mu.Unlock()
			return conn, nil
		}
		errs = append(errs, plan.name+": "+err.Error())
	}
	return nil, fmt.Errorf("all transports failed for %s: %s", target, strings.Join(errs, "; "))
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := m.st
	out.Attempts = append([]Attempt(nil), m.st.Attempts...)
	return out
}

func (m *Manager) record(a Attempt) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.st.Attempts = append(m.st.Attempts, a)
	if len(m.st.Attempts) > 32 {
		m.st.Attempts = append([]Attempt(nil), m.st.Attempts[len(m.st.Attempts)-32:]...)
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type dialPlan struct {
	name string
	kind string
	dial func(context.Context, string) (net.Conn, error)
}

func (m *Manager) plans() []dialPlan {
	cfg := m.cfg
	endpoints := append([]config.TransportEndpoint(nil), cfg.Endpoints...)
	sort.SliceStable(endpoints, func(i, j int) bool { return endpoints[i].Priority < endpoints[j].Priority })
	plans := make([]dialPlan, 0, len(endpoints)+1)
	direct := dialPlan{name: "direct", kind: "direct", dial: dialDirect}
	if cfg.Mode == "direct" {
		return []dialPlan{direct}
	}
	if cfg.DirectFirst {
		plans = append(plans, direct)
	}
	for _, ep := range endpoints {
		if !ep.Enabled {
			continue
		}
		ep := ep
		switch ep.Type {
		case "socks5":
			plans = append(plans, dialPlan{name: ep.Name, kind: ep.Type, dial: func(ctx context.Context, target string) (net.Conn, error) {
				return dialSOCKS5(ctx, ep, target)
			}})
		}
	}
	if !cfg.DirectFirst && cfg.FallbackDirect {
		plans = append(plans, direct)
	}
	if len(plans) == 0 && cfg.FallbackDirect {
		plans = append(plans, direct)
	}
	return plans
}

func dialDirect(ctx context.Context, target string) (net.Conn, error) {
	d := net.Dialer{Timeout: 8 * time.Second, KeepAlive: 20 * time.Second}
	return d.DialContext(ctx, "tcp", target)
}

func dialSOCKS5(ctx context.Context, ep config.TransportEndpoint, target string) (net.Conn, error) {
	d := net.Dialer{Timeout: 8 * time.Second, KeepAlive: 20 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", ep.Endpoint)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = conn.Close()
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	if value, has := ctx.Deadline(); has && value.Before(deadline) {
		deadline = value
	}
	_ = conn.SetDeadline(deadline)

	methods := []byte{0x00}
	if ep.Username != "" || ep.Password != "" {
		methods = append(methods, 0x02)
	}
	greeting := append([]byte{0x05, byte(len(methods))}, methods...)
	if _, err := conn.Write(greeting); err != nil {
		return nil, err
	}
	var choice [2]byte
	if _, err := io.ReadFull(conn, choice[:]); err != nil {
		return nil, err
	}
	if choice[0] != 0x05 || choice[1] == 0xff {
		return nil, errors.New("SOCKS5 proxy rejected authentication methods")
	}
	if choice[1] == 0x02 {
		if len(ep.Username) > 255 || len(ep.Password) > 255 {
			return nil, errors.New("SOCKS5 credentials are too long")
		}
		auth := []byte{0x01, byte(len(ep.Username))}
		auth = append(auth, []byte(ep.Username)...)
		auth = append(auth, byte(len(ep.Password)))
		auth = append(auth, []byte(ep.Password)...)
		if _, err := conn.Write(auth); err != nil {
			return nil, err
		}
		var resp [2]byte
		if _, err := io.ReadFull(conn, resp[:]); err != nil {
			return nil, err
		}
		if resp[1] != 0x00 {
			return nil, errors.New("SOCKS5 username/password authentication failed")
		}
	} else if choice[1] != 0x00 {
		return nil, fmt.Errorf("SOCKS5 proxy selected unsupported auth method 0x%02x", choice[1])
	}

	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 target: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("invalid SOCKS5 target port")
	}
	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			req = append(req, 0x01)
			req = append(req, v4...)
		} else {
			req = append(req, 0x04)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) == 0 || len(host) > 255 {
			return nil, errors.New("invalid SOCKS5 target hostname")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, host...)
	}
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	req = append(req, p[:]...)
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, err
	}
	if header[0] != 0x05 || header[1] != 0x00 {
		return nil, fmt.Errorf("SOCKS5 CONNECT failed with code 0x%02x", header[1])
	}
	var addrLen int
	switch header[3] {
	case 0x01:
		addrLen = 4
	case 0x04:
		addrLen = 16
	case 0x03:
		length, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		addrLen = int(length)
	default:
		return nil, errors.New("SOCKS5 proxy returned invalid address type")
	}
	if _, err := io.CopyN(io.Discard, reader, int64(addrLen+2)); err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	ok = true
	return conn, nil
}
