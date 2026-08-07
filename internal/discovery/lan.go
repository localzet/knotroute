package discovery

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
)

const LANGroup = "239.255.74.47:7448"

func RunLAN(ctx context.Context, id *identity.Identity, network networkid.ID, endpoints []string, interval time.Duration, found func(Candidate)) error {
	if interval < 5*time.Second {
		interval = 15 * time.Second
	}
	group, err := net.ResolveUDPAddr("udp4", LANGroup)
	if err != nil {
		return err
	}
	conn, err := net.ListenMulticastUDP("udp4", nil, group)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(128 << 10)
	send, err := net.DialUDP("udp4", nil, group)
	if err != nil {
		return err
	}
	defer send.Close()
	announce := func() {
		a := SignAnnouncement(id, network, endpoints, time.Now())
		raw, _ := json.Marshal(a)
		if len(raw) <= 32<<10 {
			_, _ = send.Write(raw)
		}
	}
	announce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	buf := make([]byte, 32<<10)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, readErr := conn.ReadFromUDP(buf)
		if readErr == nil {
			var a Announcement
			if json.Unmarshal(buf[:n], &a) == nil {
				nid, nw, e := a.Verify(time.Now())
				if e == nil && nw == network && nid != id.ID {
					found(Candidate{NodeID: nid.String(), Endpoints: normalizeEndpoints(a.Endpoints), SeenUnix: time.Now().Unix()})
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			announce()
		default:
		}
	}
}

func LANEndpoints(listen []string, advertise []string) []string {
	if len(advertise) > 0 {
		return normalizeEndpoints(advertise)
	}
	ports := map[string]struct{}{}
	for _, a := range listen {
		_, p, e := net.SplitHostPort(a)
		if e == nil && p != "0" {
			ports[p] = struct{}{}
		}
	}
	ips := []string{}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ipstr := a.String()
			if i := strings.IndexByte(ipstr, '/'); i >= 0 {
				ipstr = ipstr[:i]
			}
			ip := net.ParseIP(ipstr)
			if ip != nil && ip.To4() != nil {
				ips = append(ips, ip.String())
			}
		}
	}
	out := []string{}
	for p := range ports {
		for _, ip := range ips {
			out = append(out, net.JoinHostPort(ip, p))
		}
	}
	return normalizeEndpoints(out)
}
