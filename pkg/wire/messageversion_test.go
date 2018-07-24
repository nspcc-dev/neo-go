package wire

import (
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidNewVersionMessage(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, defaultVersion)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	assert.Equal(t, expectedIP, message.IP.String())
	assert.Equal(t, uint16(expectedPort), message.Port)
	assert.Equal(t, defaultVersion, message.Version)
}
func TestEncode(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, defaultVersion)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	buf := new(bytes.Buffer)
	if err := message.EncodePayload(buf); err != nil {
		assert.Fail(t, err.Error())
	}
	assert.Equal(t, len(message.UserAgent)+minMsgVersionSize, int(buf.Len()))
}
func TestLenIsCorrect(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, defaultVersion)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	assert.Equal(t, len(message.UserAgent)+minMsgVersionSize, int(message.PayloadLength()))
}
