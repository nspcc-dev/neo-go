package wire

import (
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This is quite hard to test because the message uses time.Now()
// TODO: Test each field expect time.Now(), just make sure it is a uint32
func TestWriteMessageLen(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, defaultVersion)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	buf := new(bytes.Buffer)
	if err := WriteMessage(buf, Production, message); err != nil {
		assert.Fail(t, err.Error())
	}
	assert.Equal(t, 61, len(buf.Bytes()))
}
func TestReadMessage(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}

	message, err := NewVersionMessage(tcpAddrMe, 23, true, defaultVersion)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	buf := new(bytes.Buffer)
	if err := WriteMessage(buf, Production, message); err != nil {
		assert.Fail(t, err.Error())
	}

	readmsg, err := ReadMessage(buf, Production)

	if err != nil {
		assert.Fail(t, err.Error())
	}
	version := readmsg.(*VersionMessage)
	assert.Equal(t, 23, int(version.StartHeight))
	// If MessageReading was unsuccessfull it will return a nil object
}
