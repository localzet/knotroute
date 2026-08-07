package knotapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

type Dialer interface {
	Dial(context.Context, string) (net.Conn, error)
}

type RPCHandler interface {
	Call(context.Context, string, json.RawMessage) (any, error)
}

type RPCHandlerFunc func(context.Context, string, json.RawMessage) (any, error)

func (f RPCHandlerFunc) Call(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	return f(ctx, method, raw)
}

type rpcEnvelope struct {
	Version int             `json:"version"`
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Method  string          `json:"method,omitempty"`
	Body    json.RawMessage `json:"body,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type RPCClient struct {
	Dialer Dialer
}

func (c RPCClient) Call(ctx context.Context, address, method string, request, response any) error {
	if c.Dialer == nil {
		return errors.New("RPC dialer is nil")
	}
	if method == "" {
		return errors.New("RPC method is empty")
	}
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return err
	}
	defer conn.Close()
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	id := randomID()
	if err := writeJSON(conn, rpcEnvelope{Version: 1, Type: "request", ID: id, Method: method, Body: body}); err != nil {
		return err
	}
	var reply rpcEnvelope
	if err := readJSON(conn, &reply); err != nil {
		return err
	}
	if reply.Version != 1 || reply.Type != "response" || reply.ID != id {
		return errors.New("invalid RPC response")
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	if response == nil || len(reply.Body) == 0 {
		return nil
	}
	return json.Unmarshal(reply.Body, response)
}

func RPCServer(handler RPCHandler) func(net.Conn) {
	return func(conn net.Conn) {
		if handler == nil {
			return
		}
		var writeMu sync.Mutex
		for {
			var request rpcEnvelope
			if err := readJSON(conn, &request); err != nil {
				if !errors.Is(err, io.EOF) {
					return
				}
				return
			}
			if request.Version != 1 || request.Type != "request" || request.ID == "" || request.Method == "" {
				return
			}
			result, err := handler.Call(context.Background(), request.Method, request.Body)
			reply := rpcEnvelope{Version: 1, Type: "response", ID: request.ID}
			if err != nil {
				reply.Error = err.Error()
			} else if result != nil {
				reply.Body, err = json.Marshal(result)
				if err != nil {
					reply.Error = fmt.Sprintf("encode response: %v", err)
				}
			}
			writeMu.Lock()
			err = writeJSON(conn, reply)
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func randomID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "rpc"
	}
	return hex.EncodeToString(raw[:])
}
