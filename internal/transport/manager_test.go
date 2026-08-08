package transport

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/config"
)

func TestSOCKS5Transport(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()

	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	go runTestSOCKS(proxy)

	m := New(config.Transport{Mode: "proxy", FallbackDirect: false, Endpoints: []config.TransportEndpoint{{Name: "xray", Type: "socks5", Endpoint: proxy.Addr().String(), Enabled: true}}})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := m.DialContext(ctx, echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "hello" {
		t.Fatalf("unexpected echo: %q", buf)
	}
	st := m.Status()
	if st.LastSelected != "xray" || len(st.Attempts) != 1 || !st.Attempts[0].Succeeded {
		t.Fatalf("unexpected status: %+v", st)
	}
}

func runTestSOCKS(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			r := bufio.NewReader(c)
			v, _ := r.ReadByte()
			if v != 5 {
				return
			}
			n, _ := r.ReadByte()
			methods := make([]byte, int(n))
			_, _ = io.ReadFull(r, methods)
			_, _ = c.Write([]byte{5, 0})
			h := make([]byte, 4)
			if _, err := io.ReadFull(r, h); err != nil || h[1] != 1 {
				return
			}
			var host string
			switch h[3] {
			case 1:
				b := make([]byte, 4)
				_, _ = io.ReadFull(r, b)
				host = net.IP(b).String()
			case 3:
				n, _ := r.ReadByte()
				b := make([]byte, int(n))
				_, _ = io.ReadFull(r, b)
				host = string(b)
			case 4:
				b := make([]byte, 16)
				_, _ = io.ReadFull(r, b)
				host = net.IP(b).String()
			default:
				return
			}
			p := make([]byte, 2)
			_, _ = io.ReadFull(r, p)
			target := net.JoinHostPort(host, fmtPort(binary.BigEndian.Uint16(p)))
			up, err := net.Dial("tcp", target)
			if err != nil {
				_, _ = c.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
				return
			}
			defer up.Close()
			_, _ = c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
			done := make(chan struct{}, 2)
			go func() { _, _ = io.Copy(up, r); done <- struct{}{} }()
			go func() { _, _ = io.Copy(c, up); done <- struct{}{} }()
			<-done
		}()
	}
}

func fmtPort(p uint16) string {
	if p == 0 {
		return "0"
	}
	var b [5]byte
	i := len(b)
	for p > 0 {
		i--
		b[i] = byte('0' + p%10)
		p /= 10
	}
	return string(b[i:])
}
