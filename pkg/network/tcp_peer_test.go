package network

import (
	"net"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/stretchr/testify/require"
)

func connReadStub(conn net.Conn) {
	b := make([]byte, 1024)
	var err error
	for ; err == nil; _, err = conn.Read(b) {
	}
}

func TestPeerHandshake(t *testing.T) {
	server, client := net.Pipe()

	tcpS := NewTCPPeer(server)
	tcpC := NewTCPPeer(client)

	// Something should read things written into the pipe.
	go connReadStub(tcpS.conn)
	go connReadStub(tcpC.conn)

	// No handshake yet.
	require.Equal(t, false, tcpS.Handshaked())
	require.Equal(t, false, tcpC.Handshaked())

	// No ordinary messages can be written.
	require.Error(t, tcpS.WriteMsg(&Message{}))
	require.Error(t, tcpC.WriteMsg(&Message{}))

	// Try to mess with VersionAck on both client and server, it should fail.
	require.Error(t, tcpS.SendVersionAck(&Message{}))
	require.Error(t, tcpS.HandleVersionAck())
	require.Error(t, tcpC.SendVersionAck(&Message{}))
	require.Error(t, tcpC.HandleVersionAck())

	// No handshake yet.
	require.Equal(t, false, tcpS.Handshaked())
	require.Equal(t, false, tcpC.Handshaked())

	// Now send and handle versions, but in a different order on client and
	// server.
	require.NoError(t, tcpC.SendVersion(&Message{}))
	require.Error(t, tcpC.HandleVersionAck()) // Didn't receive version yet.
	require.NoError(t, tcpS.HandleVersion(&payload.Version{}))
	require.Error(t, tcpS.SendVersionAck(&Message{})) // Didn't send version yet.
	require.NoError(t, tcpC.HandleVersion(&payload.Version{}))
	require.NoError(t, tcpS.SendVersion(&Message{}))

	// No handshake yet.
	require.Equal(t, false, tcpS.Handshaked())
	require.Equal(t, false, tcpC.Handshaked())

	// These are sent/received and should fail now.
	require.Error(t, tcpC.SendVersion(&Message{}))
	require.Error(t, tcpS.HandleVersion(&payload.Version{}))
	require.Error(t, tcpC.HandleVersion(&payload.Version{}))
	require.Error(t, tcpS.SendVersion(&Message{}))

	// Now send and handle ACK, again in a different order on client and
	// server.
	require.NoError(t, tcpC.SendVersionAck(&Message{}))
	require.NoError(t, tcpS.HandleVersionAck())
	require.NoError(t, tcpC.HandleVersionAck())
	require.NoError(t, tcpS.SendVersionAck(&Message{}))

	// Handshaked now.
	require.Equal(t, true, tcpS.Handshaked())
	require.Equal(t, true, tcpC.Handshaked())

	// Subsequent ACKing should fail.
	require.Error(t, tcpC.SendVersionAck(&Message{}))
	require.Error(t, tcpS.HandleVersionAck())
	require.Error(t, tcpC.HandleVersionAck())
	require.Error(t, tcpS.SendVersionAck(&Message{}))

	// Now regular messaging can proceed.
	require.NoError(t, tcpS.WriteMsg(&Message{}))
	require.NoError(t, tcpC.WriteMsg(&Message{}))
}
