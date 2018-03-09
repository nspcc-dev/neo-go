package network

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/stretchr/testify/assert"
)

func TestMessageEncodeDecode(t *testing.T) {
	m := NewMessage(ModeTestNet, CMDVersion, nil)

	buf := &bytes.Buffer{}
	if err := m.encode(buf); err != nil {
		t.Error(err)
	}
	assert.Equal(t, len(buf.Bytes()), minMessageSize)

	md := &Message{}
	if err := md.decode(buf); err != nil {
		t.Error(err)
	}
	assert.Equal(t, m, md)
}

func TestMessageEncodeDecodeWithVersion(t *testing.T) {
	var (
		p = payload.NewVersion(12227, 2000, "/neo:2.6.0/", 0, true)
		m = NewMessage(ModeTestNet, CMDVersion, p)
	)

	buf := new(bytes.Buffer)
	if err := m.encode(buf); err != nil {
		t.Error(err)
	}

	mDecode := &Message{}
	if err := mDecode.decode(buf); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, m, mDecode)
}

func TestMessageInvalidChecksum(t *testing.T) {
	var (
		p = payload.NewVersion(1111, 3000, "/NEO:2.6.0/", 0, true)
		m = NewMessage(ModeTestNet, CMDVersion, p)
	)
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
