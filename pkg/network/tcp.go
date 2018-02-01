package network

import (
	"bytes"
	"fmt"
	"net"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

func listenTCP(s *Server, port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	s.listener = ln

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		go handleConnection(s, conn)
	}
}

func connectToRemoteNode(s *Server, address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		s.logger.Printf("failed to connect to remote node %s", address)
		if conn != nil {
			conn.Close()
		}
		return
	}
	go handleConnection(s, conn)
}

func connectToSeeds(s *Server, addrs []string) {
	for _, addr := range addrs {
		go connectToRemoteNode(s, addr)
	}
}

func handleConnection(s *Server, conn net.Conn) {
	peer := NewTCPPeer(conn, s)
	s.register <- peer

	// remove the peer from connected peers and cleanup the connection.
	defer func() {
		// all cleanup will happen in the server's loop when unregister is received.
		s.unregister <- peer
	}()

	// Start a goroutine that will handle all outgoing messages.
	go peer.writeLoop()
	// Start a goroutine that will handle all incomming messages.
	go handleMessage(s, peer)

	// Read from the connection and decode it into a Message ready for processing.
	buf := make([]byte, 1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			s.logger.Printf("conn read error: %s", err)
			break
		}

		msg := &Message{}
		if err := msg.decode(bytes.NewReader(buf)); err != nil {
			s.logger.Printf("decode error %s", err)
			break
		}

		peer.receive <- msg
	}
}

// handleMessage hands the message received from a TCP connection over to the server.
func handleMessage(s *Server, p *TCPPeer) {
	// Disconnect the peer when we break out of the loop.
	defer func() {
		p.disconnect()
	}()

	for {
		msg := <-p.receive
		command := msg.commandType()

		s.logger.Printf("IN :: %d :: %s :: %v", p.id(), command, msg)

		switch command {
		case cmdVersion:
			resp := s.handleVersionCmd(msg, p)
			p.nonce = msg.Payload.(*payload.Version).Nonce
			p.send <- resp

			// after sending our version we want a "verack" and nothing else.
			msg := <-p.receive
			if msg.commandType() != cmdVerack {
				break
			}
			// we can start the protocol now.
			go s.sendLoop(p)
		case cmdAddr:
			s.handleAddrCmd(msg, p)
		case cmdGetAddr:
			s.handleGetaddrCmd(msg, p)
		case cmdInv:
			resp := s.handleInvCmd(msg, p)
			p.send <- resp
		case cmdBlock:
		case cmdConsensus:
		case cmdTX:
		case cmdVerack:
			// disconnect the peer, verack should already be handled.
			break
		case cmdGetHeaders:
		case cmdGetBlocks:
		case cmdGetData:
		case cmdHeaders:
		}
	}
}

// TCPPeer represents a remote node, backed by TCP transport.
type TCPPeer struct {
	s *Server
	// nonce (id) of the peer.
	nonce uint32
	// underlying TCP connection
	conn net.Conn
	// host and port information about this peer.
	endpoint util.Endpoint
	// channel to coordinate messages writen back to the connection.
	send chan *Message
	// channel to receive from underlying connection.
	receive chan *Message
}

// NewTCPPeer returns a pointer to a TCP Peer.
func NewTCPPeer(conn net.Conn, s *Server) *TCPPeer {
	e, _ := util.EndpointFromString(conn.RemoteAddr().String())

	return &TCPPeer{
		conn:     conn,
		send:     make(chan *Message),
		receive:  make(chan *Message),
		endpoint: e,
		s:        s,
	}
}

func (p *TCPPeer) callVersion(msg *Message) {
	p.send <- msg
}

// id implements the peer interface
func (p *TCPPeer) id() uint32 {
	return p.nonce
}

// endpoint implements the peer interface
func (p *TCPPeer) addr() util.Endpoint {
	return p.endpoint
}

// callGetaddr will send the "getaddr" command to the remote.
func (p *TCPPeer) callGetaddr(msg *Message) {
	p.send <- msg
}

// disconnect closes the send channel and the underlying connection.
func (p *TCPPeer) disconnect() {
	close(p.send)
	p.conn.Close()
}

// writeLoop writes messages to the underlying TCP connection.
// A goroutine writeLoop is started for each connection.
// There should be at most one writer to a connection executing
// all writes from this goroutine.
func (p *TCPPeer) writeLoop() {
	// clean up the connection.
	defer func() {
		p.conn.Close()
	}()

	for {
		msg := <-p.send

		p.s.logger.Printf("OUT :: %s :: %+v", msg.commandType(), msg.Payload)

		// should we disconnect here?
		if err := msg.encode(p.conn); err != nil {
			p.s.logger.Printf("encode error: %s", err)
		}
	}
}
