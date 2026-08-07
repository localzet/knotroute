package overlay

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/serviceid"
)

func (n *Node) openLocalNamedService(ctx context.Context, name string) (net.Conn, error) {
	service, ok := n.service(name)
	if !ok {
		return nil, fmt.Errorf("service %q not found", name)
	}
	if !n.allowed(service, n.id.ID) {
		return nil, errors.New("local node is not allowed to access this service")
	}
	return n.dialLocalService(ctx, service)
}

func (n *Node) localPublishedService(id serviceid.ID) (config.Service, bool) {
	n.directory.mu.RLock()
	defer n.directory.mu.RUnlock()
	for _, service := range n.directory.local {
		if service.identity.ID == id {
			return service.cfg, true
		}
	}
	return config.Service{}, false
}

func (n *Node) dialLocalService(ctx context.Context, service config.Service) (net.Conn, error) {
	conn, err := n.dialServiceTarget(ctx, service.Target)
	if err != nil {
		return nil, fmt.Errorf("service target unavailable: %w", err)
	}
	return conn, nil
}
