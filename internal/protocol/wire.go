package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/localzet/knotroute/internal/nodeid"
)

const (
	ProtocolVersion = 1
	MaxFrameSize    = 4 << 20

	FrameHello  byte = 1
	FrameLSA    byte = 2
	FramePacket byte = 3
	FramePing   byte = 4
	FramePong   byte = 5

	PacketOpen    byte = 1
	PacketOpenAck byte = 2
	PacketData    byte = 3
	PacketClose   byte = 4
	PacketError   byte = 5
	PacketReady   byte = 6
)

const packetHeaderSize = 1 + 1 + 1 + 1 + 32 + 32 + 16 + 16 + 8

type Packet struct {
	Version  byte
	Kind     byte
	TTL      byte
	Flags    byte
	Src      nodeid.ID
	Dst      nodeid.ID
	PacketID [16]byte
	StreamID [16]byte
	Seq      uint64
	Payload  []byte
}

func WriteFrame(w io.Writer, typ byte, payload []byte) error {
	if len(payload)+1 > MaxFrameSize {
		return fmt.Errorf("frame too large: %d", len(payload)+1)
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)+1))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if _, err := w.Write([]byte{typ}); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func ReadFrame(r io.Reader) (byte, []byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	size := int(binary.BigEndian.Uint32(header[:]))
	if size < 1 || size > MaxFrameSize {
		return 0, nil, fmt.Errorf("invalid frame size %d", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, nil, err
	}
	return buf[0], buf[1:], nil
}

func (p Packet) MarshalBinary() ([]byte, error) {
	if p.Version == 0 {
		p.Version = ProtocolVersion
	}
	if p.Version != ProtocolVersion {
		return nil, fmt.Errorf("unsupported packet version %d", p.Version)
	}
	out := make([]byte, packetHeaderSize+len(p.Payload))
	out[0], out[1], out[2], out[3] = p.Version, p.Kind, p.TTL, p.Flags
	off := 4
	copy(out[off:off+32], p.Src[:])
	off += 32
	copy(out[off:off+32], p.Dst[:])
	off += 32
	copy(out[off:off+16], p.PacketID[:])
	off += 16
	copy(out[off:off+16], p.StreamID[:])
	off += 16
	binary.BigEndian.PutUint64(out[off:off+8], p.Seq)
	off += 8
	copy(out[off:], p.Payload)
	return out, nil
}

func ParsePacket(data []byte) (Packet, error) {
	var p Packet
	if len(data) < packetHeaderSize {
		return p, errors.New("packet is truncated")
	}
	p.Version, p.Kind, p.TTL, p.Flags = data[0], data[1], data[2], data[3]
	if p.Version != ProtocolVersion {
		return p, fmt.Errorf("unsupported packet version %d", p.Version)
	}
	off := 4
	copy(p.Src[:], data[off:off+32])
	off += 32
	copy(p.Dst[:], data[off:off+32])
	off += 32
	copy(p.PacketID[:], data[off:off+16])
	off += 16
	copy(p.StreamID[:], data[off:off+16])
	off += 16
	p.Seq = binary.BigEndian.Uint64(data[off : off+8])
	off += 8
	p.Payload = bytes.Clone(data[off:])
	return p, nil
}
