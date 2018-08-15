package wire

import (
	"bytes"
	"net"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/stretchr/testify/assert"
)

// This is quite hard to test because the message uses time.Now()
// TODO: Test each field expect time.Now(), just make sure it is a uint32
func TestWriteMessageLen(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}

	message, err := payload.NewVersionMessage(tcpAddrMe, 0, true, protocol.DefaultVersion, protocol.UserAgent, 100, protocol.NodePeerService)
	if err != nil {
		assert.Fail(t, err.Error())
	}

	buf := new(bytes.Buffer)
	if err := WriteMessage(buf, protocol.MainNet, message); err != nil {
		assert.Fail(t, err.Error())
	}
	// This test will fail, if useragent is changed in protocol
	assert.Equal(t, 60, len(buf.Bytes()))
}
func TestReadMessage(t *testing.T) {

	expectedIP := "127.0.0.1"
	expectedPort := 8333
	tcpAddrMe := &net.TCPAddr{IP: net.ParseIP(expectedIP), Port: expectedPort}

	message, err := payload.NewVersionMessage(tcpAddrMe, 23, true, protocol.DefaultVersion, protocol.UserAgent, 100, protocol.NodePeerService)
	if err != nil {
		assert.Fail(t, err.Error())
	}
	buf := new(bytes.Buffer)
	if err := WriteMessage(buf, protocol.MainNet, message); err != nil {
		assert.Fail(t, err.Error())
	}

	readmsg, err := ReadMessage(buf, protocol.MainNet)

	if err != nil {
		assert.Fail(t, err.Error())
	}
	version := readmsg.(*payload.VersionMessage)
	assert.Equal(t, 23, int(version.StartHeight))
	// If MessageReading was unsuccessfull it will return a nil object
}
