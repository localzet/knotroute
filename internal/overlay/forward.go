package overlay

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/localzet/knotroute/internal/nodeid"
)

func (n *Node) startForwards() {
	n.forwardState = make([]ForwardStatus, len(n.cfg.Forwards))
	for i, f := range n.cfg.Forwards {
		state := ForwardStatus{Listen: f.Listen, Node: f.Node, Service: f.Service}
		listener, err := net.Listen("tcp", f.Listen)
		if err != nil {
			state.Error = err.Error()
			n.forwardState[i] = state
			n.addEvent("error", "forward "+f.Listen+": "+err.Error())
			continue
		}
		state.Active = true
		state.Listen = listener.Addr().String()
		n.forwardState[i] = state
		n.forwardListeners = append(n.forwardListeners, listener)
		destination, _ := nodeid.Parse(f.Node)
		n.wg.Add(1)
		go n.forwardLoop(listener, destination, f.Service)
	}
}

func (n *Node) forwardLoop(listener net.Listener, destination nodeid.ID, service string) {
	defer n.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if n.ctx.Err() != nil {
				return
			}
			n.addEvent("warn", "forward accept: "+err.Error())
			continue
		}
		n.wg.Add(1)
		go func(c net.Conn) {
			defer n.wg.Done()
			ctx, cancel := context.WithTimeout(n.ctx, 20*time.Second)
			defer cancel()
			if err := n.openWithConn(ctx, destination, service, c); err != nil {
				n.addEvent("warn", fmt.Sprintf("forward %s -> %s/%s failed: %v", listener.Addr(), destination.Short(), service, err))
				_ = c.Close()
			}
		}(conn)
	}
}
