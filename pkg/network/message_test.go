package network

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestNewMessage(t *testing.T) {
	payload := []byte{}
	m := newMessage(ModeTestNet, "version", payload)

	if have, want := m.Length, uint32(0); want != have {
		t.Errorf("want %d have %d", want, have)
	}
	if have, want := len(m.Command), 12; want != have {
		t.Errorf("want %d have %d", want, have)
	}

	sum := sumSHA256(sumSHA256(payload))[:4]
	sumuint32 := binary.LittleEndian.Uint32(sum)
	if have, want := m.Checksum, sumuint32; want != have {
		t.Errorf("want %d have %d", want, have)
	}
}
func TestMessageEncodeDecode(t *testing.T) {
	m := newMessage(ModeTestNet, "version", []byte{})

	buf := &bytes.Buffer{}
	if err := m.encode(buf); err != nil {
		t.Error(err)
	}

	md := &Message{}
	if err := md.decode(buf); err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(m, md) {
		t.Errorf("both messages should be equal: %v != %v", m, md)
	}
}

func TestMessageInvalidChecksum(t *testing.T) {
	m := newMessage(ModeTestNet, "version", []byte{})
	m.Checksum = 1337

	buf := &bytes.Buffer{}
	if err := m.encode(buf); err != nil {
		t.Error(err)
	}

	md := &Message{}
	if err := md.decode(buf); err == nil {
		t.Error("decode should failed with checkum mismatch error")
	}
}

func TestNewVersionPayload(t *testing.T) {
	p := newVersionPayload(3000, "/neo/", 0, true)
	b, err := p.encode()
	if err != nil {
		t.Fatal(err)
	}

	pd := &Version{}
	if err := pd.decode(b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, pd) {
		t.Errorf("both payloads should be equal: %v != %v", p, pd)
	}
}
