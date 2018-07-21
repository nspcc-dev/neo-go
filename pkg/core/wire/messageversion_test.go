package wire

import (
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
func TestBufferLen(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, defaultVersion)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	assert.Equal(t, len(message.UserAgent)+27, uint32(message.PayloadLength()))

}
