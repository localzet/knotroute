package knotapp

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const DefaultObjectLimit int64 = 256 << 20

var objectEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

type ObjectID [32]byte

func ObjectIDFor(raw []byte) ObjectID { return sha256.Sum256(raw) }
func (id ObjectID) String() string {
	return "ko_" + strings.ToLower(objectEncoding.EncodeToString(id[:]))
}
func ParseObjectID(value string) (ObjectID, error) {
	var id ObjectID
	if !strings.HasPrefix(value, "ko_") {
		return id, errors.New("invalid object id prefix")
	}
	raw, err := objectEncoding.DecodeString(strings.ToUpper(strings.TrimPrefix(value, "ko_")))
	if err != nil || len(raw) != len(id) {
		return id, errors.New("invalid object id")
	}
	copy(id[:], raw)
	return id, nil
}

type ObjectStore interface {
	Get(ObjectID) ([]byte, error)
	Put([]byte) (ObjectID, error)
}

type FileObjectStore struct {
	Directory string
	MaxBytes  int64
}

func (s FileObjectStore) limit() int64 {
	if s.MaxBytes <= 0 {
		return DefaultObjectLimit
	}
	return s.MaxBytes
}
func (s FileObjectStore) Get(id ObjectID) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.Directory, id.String()))
}
func (s FileObjectStore) Put(raw []byte) (ObjectID, error) {
	var zero ObjectID
	if int64(len(raw)) > s.limit() {
		return zero, fmt.Errorf("object exceeds limit of %d bytes", s.limit())
	}
	if err := os.MkdirAll(s.Directory, 0o700); err != nil {
		return zero, err
	}
	id := ObjectIDFor(raw)
	path := filepath.Join(s.Directory, id.String())
	if existing, err := os.ReadFile(path); err == nil {
		if ObjectIDFor(existing) != id {
			return zero, errors.New("object store hash collision")
		}
		return id, nil
	}
	tmp, err := os.CreateTemp(s.Directory, ".object-*")
	if err != nil {
		return zero, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return zero, err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return zero, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return zero, err
	}
	if err := tmp.Close(); err != nil {
		return zero, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		if _, statErr := os.Stat(path); statErr == nil {
			return id, nil
		}
		return zero, err
	}
	return id, nil
}

type objectRequest struct {
	Version int    `json:"version"`
	Op      string `json:"op"`
	ID      string `json:"id,omitempty"`
}
type objectResponse struct {
	Version int    `json:"version"`
	OK      bool   `json:"ok"`
	ID      string `json:"id,omitempty"`
	Error   string `json:"error,omitempty"`
}

func ObjectServer(store ObjectStore, maxBytes int64) func(net.Conn) {
	if maxBytes <= 0 {
		maxBytes = DefaultObjectLimit
	}
	return func(conn net.Conn) {
		if store == nil {
			return
		}
		var request objectRequest
		if err := readJSON(conn, &request); err != nil || request.Version != 1 {
			return
		}
		switch request.Op {
		case "get":
			id, err := ParseObjectID(request.ID)
			if err != nil {
				_ = writeJSON(conn, objectResponse{Version: 1, Error: err.Error()})
				return
			}
			raw, err := store.Get(id)
			if err != nil || ObjectIDFor(raw) != id || int64(len(raw)) > maxBytes {
				if err == nil {
					err = errors.New("stored object failed integrity check")
				}
				_ = writeJSON(conn, objectResponse{Version: 1, Error: err.Error()})
				return
			}
			if writeJSON(conn, objectResponse{Version: 1, OK: true, ID: id.String()}) == nil {
				_ = writeBlob(conn, raw, maxBytes)
			}
		case "put":
			raw, err := readBlob(conn, maxBytes)
			if err != nil {
				_ = writeJSON(conn, objectResponse{Version: 1, Error: err.Error()})
				return
			}
			id, err := store.Put(raw)
			if err != nil {
				_ = writeJSON(conn, objectResponse{Version: 1, Error: err.Error()})
				return
			}
			_ = writeJSON(conn, objectResponse{Version: 1, OK: true, ID: id.String()})
		}
	}
}

type ObjectClient struct {
	Dialer   Dialer
	MaxBytes int64
}

func (c ObjectClient) limit() int64 {
	if c.MaxBytes <= 0 {
		return DefaultObjectLimit
	}
	return c.MaxBytes
}
func (c ObjectClient) Put(ctx context.Context, address string, raw []byte) (ObjectID, error) {
	var zero ObjectID
	if c.Dialer == nil {
		return zero, errors.New("object dialer is nil")
	}
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return zero, err
	}
	defer conn.Close()
	if err := writeJSON(conn, objectRequest{Version: 1, Op: "put"}); err != nil {
		return zero, err
	}
	if err := writeBlob(conn, raw, c.limit()); err != nil {
		return zero, err
	}
	var response objectResponse
	if err := readJSON(conn, &response); err != nil {
		return zero, err
	}
	if !response.OK {
		return zero, errors.New(response.Error)
	}
	id, err := ParseObjectID(response.ID)
	if err != nil {
		return zero, err
	}
	if id != ObjectIDFor(raw) {
		return zero, errors.New("object server returned wrong content hash")
	}
	return id, nil
}
func (c ObjectClient) Get(ctx context.Context, address string, id ObjectID) ([]byte, error) {
	if c.Dialer == nil {
		return nil, errors.New("object dialer is nil")
	}
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := writeJSON(conn, objectRequest{Version: 1, Op: "get", ID: id.String()}); err != nil {
		return nil, err
	}
	var response objectResponse
	if err := readJSON(conn, &response); err != nil {
		return nil, err
	}
	if !response.OK {
		return nil, errors.New(response.Error)
	}
	raw, err := readBlob(conn, c.limit())
	if err != nil {
		return nil, err
	}
	if ObjectIDFor(raw) != id {
		return nil, errors.New("object integrity check failed")
	}
	return raw, nil
}
