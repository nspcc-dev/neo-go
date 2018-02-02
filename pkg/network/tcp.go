package network

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
		peer.disconnect()
	}()

	// Start a goroutine that will handle all outgoing messages.
	go peer.writeLoop()
	// Start a goroutine that will handle all incomming messages.
	go handleMessage(s, peer)

	// Read from the connection and decode it into a Message ready for processing.
	buf := make([]byte, 1024)
	for {
		_, err := conn.Read(buf)
		if err == io.EOF {
			return
		}
		if err != nil {
			s.logger.Printf("conn read error: %s", err)
			return
		}

		msg := &Message{}
		if err := msg.decode(bytes.NewReader(buf)); err != nil {
			s.logger.Printf("decode error %s", err)
			return
		}

		peer.receive <- msg
	}
}

// handleMessage hands the message received from a TCP connection over to the server.
func handleMessage(s *Server, p *TCPPeer) {
	var err error

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
			if err = s.handleVersionCmd(msg, p); err != nil {
				return
			}
			p.nonce = msg.Payload.(*payload.Version).Nonce

			// When a node receives a connection request, it declares its version immediately.
			// There will be no other communication until both sides are getting versions of each other.
			// When a node receives the version message, it replies to a verack as a response immediately.
			// NOTE: The current official NEO nodes dont mimic this behaviour. There is small chance that the
			// official nodes will not respond directly with a verack after we sended our version.
			// is this a bug? - anthdm 02/02/2018
			msgVerack := <-p.receive
			if msgVerack.commandType() != cmdVerack {
				err = errors.New("expected verack message after version")
			}

			// start the protocol
			go s.sendLoop(p)
		case cmdAddr:
			err = s.handleAddrCmd(msg, p)
		case cmdGetAddr:
			err = s.handleGetaddrCmd(msg, p)
		case cmdInv:
			err = s.handleInvCmd(msg, p)
		case cmdBlock:
			err = s.handleBlockCmd(msg, p)
		case cmdConsensus:
		case cmdTX:
		case cmdVerack:
			// If we receive a verack here we disconnect. We already handled the verack
			// when we sended our version.
			err = errors.New("received verack twice")
		case cmdGetHeaders:
		case cmdGetBlocks:
		case cmdGetData:
		case cmdHeaders:
		}

		// catch all errors here and disconnect.
		if err != nil {
			s.logger.Printf("processing message failed: %s", err)
			return
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
	quit    chan bool
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
		quit:     make(chan bool),
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

func (p *TCPPeer) callVerack(msg *Message) {
	p.send <- msg
}

func (p *TCPPeer) callGetdata(msg *Message) {
	p.send <- msg
}

// disconnect disconnects the peer, cleaning up all its resources.
// 3 goroutines needs to be cleanup (writeLoop, handleConnection and handleMessage)
func (p *TCPPeer) disconnect() {
	select {
	case <-p.send:
	case <-p.receive:
	default:
		close(p.send)
		close(p.receive)
		p.s.unregister <- p
		p.conn.Close()
	}
}

// writeLoop writes messages to the underlying TCP connection.
// A goroutine writeLoop is started for each connection.
// There should be at most one writer to a connection executing
// all writes from this goroutine.
func (p *TCPPeer) writeLoop() {
	// clean up the connection.
	defer func() {
		p.disconnect()
	}()

	for {
		msg := <-p.send
		if msg == nil {
			return
		}

		p.s.logger.Printf("OUT :: %s :: %+v", msg.commandType(), msg.Payload)

		// should we disconnect here?
		if err := msg.encode(p.conn); err != nil {
			p.s.logger.Printf("encode error: %s", err)
		}
	}
}
