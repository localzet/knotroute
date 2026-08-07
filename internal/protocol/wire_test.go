package protocol

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/localzet/knotroute/internal/nodeid"
)

func TestPacketRoundTrip(t *testing.T) {
	var p Packet
	p.Version = ProtocolVersion
	p.Kind = PacketData
	p.TTL = 12
	_, _ = rand.Read(p.Src[:])
	_, _ = rand.Read(p.Dst[:])
	_, _ = rand.Read(p.PacketID[:])
	_, _ = rand.Read(p.StreamID[:])
	p.Seq = 42
	p.Payload = []byte("encrypted payload")
	raw, err := p.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParsePacket(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Src != nodeid.ID(p.Src) || got.Dst != nodeid.ID(p.Dst) || got.Seq != p.Seq || !bytes.Equal(got.Payload, p.Payload) {
		t.Fatal("packet changed during roundtrip")
	}
}

func TestFrameRoundTrip(t *testing.T) {
	var b bytes.Buffer
	if err := WriteFrame(&b, FramePing, []byte("x")); err != nil {
		t.Fatal(err)
	}
	typ, payload, err := ReadFrame(&b)
	if err != nil {
		t.Fatal(err)
	}
	if typ != FramePing || string(payload) != "x" {
		t.Fatal("bad frame")
	}
}
