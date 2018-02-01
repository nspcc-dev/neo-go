package network

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

func TestMessageEncodeDecode(t *testing.T) {
	m := newMessage(ModeTestNet, cmdVersion, nil)

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

func TestMessageEncodeDecodeWithVersion(t *testing.T) {
	p := payload.NewVersion(12227, 2000, "/neo:2.6.0/", 0, true)
	m := newMessage(ModeTestNet, cmdVersion, p)

	buf := new(bytes.Buffer)
	if err := m.encode(buf); err != nil {
		t.Error(err)
	}

	mDecode := &Message{}
	if err := mDecode.decode(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(m, mDecode) {
		t.Fatalf("expected both messages to be equal %v and %v", m, mDecode)
	}
}

func TestMessageInvalidChecksum(t *testing.T) {
	p := payload.NewVersion(1111, 3000, "/NEO:2.6.0/", 0, true)
	m := newMessage(ModeTestNet, cmdVersion, p)
	m.Checksum = 1337

	buf := new(bytes.Buffer)
	if err := m.encode(buf); err != nil {
		t.Error(err)
	}

	md := &Message{}
	if err := md.decode(buf); err == nil && err != errChecksumMismatch {
		t.Fatalf("decode should fail with %s", errChecksumMismatch)
	}
}
