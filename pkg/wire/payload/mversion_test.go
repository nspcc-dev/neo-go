package payload

import (
	"bytes"
	"net"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/stretchr/testify/assert"
)

func TestValidNewVersionMessage(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, protocol.DefaultVersion)

	assert.Equal(t, nil, err)
	assert.Equal(t, expectedIP, message.IP.String())
	assert.Equal(t, uint16(expectedPort), message.Port)
	assert.Equal(t, protocol.DefaultVersion, message.Version)
}
func TestEncode(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, protocol.DefaultVersion)

	buf := new(bytes.Buffer)
	err = message.EncodePayload(buf)

	assert.Equal(t, nil, err)
	assert.Equal(t, len(message.UserAgent)+minMsgVersionSize, int(buf.Len()))
}
func TestLenIsCorrect(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}
	message, err := NewVersionMessage(tcpAddrMe, 0, true, protocol.DefaultVersion)

	assert.Equal(t, nil, err)
	assert.Equal(t, len(message.UserAgent)+minMsgVersionSize, int(message.PayloadLength()))
}
