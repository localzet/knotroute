package knotapp

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const maxControlFrame = 1 << 20

func writeJSON(w io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > maxControlFrame {
		return errors.New("Knot application control frame too large")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(raw)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}

func readJSON(r io.Reader, value any) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	size := int(binary.BigEndian.Uint32(header[:]))
	if size <= 0 || size > maxControlFrame {
		return fmt.Errorf("invalid Knot application control frame size %d", size)
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(r, raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}

func writeBlob(w io.Writer, raw []byte, max int64) error {
	if int64(len(raw)) > max {
		return fmt.Errorf("blob exceeds limit of %d bytes", max)
	}
	var header [8]byte
	binary.BigEndian.PutUint64(header[:], uint64(len(raw)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(raw)
	return err
}

func readBlob(r io.Reader, max int64) ([]byte, error) {
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint64(header[:])
	if size > uint64(max) {
		return nil, fmt.Errorf("blob exceeds limit of %d bytes", max)
	}
	raw := make([]byte, int(size))
	_, err := io.ReadFull(r, raw)
	return raw, err
}
