package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	FrameCircuit   byte = 7
	CircuitVersion byte = 1
	CircuitCreate  byte = 1
	CircuitCreated byte = 2
	CircuitRelay   byte = 3
	CircuitDestroy byte = 4

	RelayExtend    byte = 1
	RelayExtended  byte = 2
	RelayOpen      byte = 3
	RelayOpenOK    byte = 4
	RelayOpenError byte = 5
	RelayData      byte = 6
	RelayClose     byte = 7
)

type CircuitCell struct {
	Version   byte
	Kind      byte
	CircuitID uint64
	Payload   []byte
}

func (c CircuitCell) MarshalBinary() ([]byte, error) {
	if c.Version == 0 {
		c.Version = CircuitVersion
	}
	if c.Version != CircuitVersion {
		return nil, fmt.Errorf("unsupported circuit version %d", c.Version)
	}
	out := make([]byte, 10+len(c.Payload))
	out[0] = c.Version
	out[1] = c.Kind
	binary.BigEndian.PutUint64(out[2:10], c.CircuitID)
	copy(out[10:], c.Payload)
	return out, nil
}
func ParseCircuitCell(b []byte) (CircuitCell, error) {
	if len(b) < 10 {
		return CircuitCell{}, errors.New("circuit cell truncated")
	}
	c := CircuitCell{Version: b[0], Kind: b[1], CircuitID: binary.BigEndian.Uint64(b[2:10]), Payload: append([]byte(nil), b[10:]...)}
	if c.Version != CircuitVersion {
		return CircuitCell{}, fmt.Errorf("unsupported circuit version %d", c.Version)
	}
	return c, nil
}
func RelayPayload(cmd byte, body []byte) []byte { return append([]byte{cmd}, body...) }
func ParseRelayPayload(b []byte) (byte, []byte, error) {
	if len(b) < 1 {
		return 0, nil, errors.New("relay payload empty")
	}
	return b[0], b[1:], nil
}
