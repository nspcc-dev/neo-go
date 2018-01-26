package network

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestNewMessage(t *testing.T) {
	payload := []byte{}
	m := newMessage(ModeTestNet, cmdVersion, payload)

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
	m := newMessage(ModeTestNet, cmdVersion, []byte{})

	buf := &bytes.Buffer{}
	if err := m.encode(buf); err != nil {
		t.Error(err)
	}

	if n := len(buf.Bytes()); n < minMessageSize {
		t.Fatalf("message should be at least %d bytes got %d", minMessageSize, n)
	}
	if n := len(buf.Bytes()); n > minMessageSize {
		t.Fatalf("message without a payload should be exact %d bytes got %d", minMessageSize, n)
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
	m := newMessage(ModeTestNet, cmdVersion, []byte{})
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
	ua := "/neo/0.0.1/"
	p := newVersionPayload(3000, ua, 0, true)
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
