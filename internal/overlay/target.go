package overlay

import (
	"context"
	"net"
	"time"
)

// dialServiceTarget is the single local application-target dial path used by
// direct streams, onion circuits, rendezvous services, and local fast paths.
// Keeping it centralized prevents those transports from drifting apart.
func (n *Node) dialServiceTarget(ctx context.Context, target string) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return dialer.DialContext(ctx, "tcp", target)
}
