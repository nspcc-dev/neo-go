package network

import (
	"bytes"
	"errors"
	"fmt"
	"net"

	"github.com/CityOfZion/neo-go/pkg/core"
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
	for {
		msg := &Message{}
		if err := msg.decode(conn); err != nil {
			s.logger.Printf("decode error: %s", err)
			break
		}

		peer.receive <- msg
	}
}

// handleMessage multiplexes the message received from a TCP connection to a server command.
func handleMessage(s *Server, p *TCPPeer) {
	var err error

	for {
		msg := <-p.receive
		command := msg.commandType()

		// s.logger.Printf("IN :: %d :: %s :: %v", p.id(), command, msg)

		switch command {
		case cmdVersion:
			version := msg.Payload.(*payload.Version)
			if err = s.handleVersionCmd(version, p); err != nil {
				break
			}
			p.nonce = version.Nonce
			p.pVersion = version

			// When a node receives a connection request, it declares its version immediately.
			// There will be no other communication until both sides are getting versions of each other.
			// When a node receives the version message, it replies to a verack as a response immediately.
			// NOTE: The current official NEO nodes dont mimic this behaviour. There is small chance that the
			// official nodes will not respond directly with a verack after we sended our version.
			// is this a bug? - anthdm 02/02/2018
			msgVerack := <-p.receive
			if msgVerack.commandType() != cmdVerack {
				err = errors.New("expected verack after sended out version")
				break
			}

			// start the protocol
			go s.startProtocol(p)
		case cmdAddr:
			addrList := msg.Payload.(*payload.AddressList)
			err = s.handleAddrCmd(addrList, p)
		case cmdGetAddr:
			err = s.handleGetaddrCmd(msg, p)
		case cmdInv:
			inv := msg.Payload.(*payload.Inventory)
			err = s.handleInvCmd(inv, p)
		case cmdBlock:
			block := msg.Payload.(*core.Block)
			err = s.handleBlockCmd(block, p)
		case cmdConsensus:
		case cmdTX:
		case cmdVerack:
			// If we receive a verack here we disconnect. We already handled the verack
			// when we sended our version.
			err = errors.New("verack already received")
		case cmdGetHeaders:
		case cmdGetBlocks:
		case cmdGetData:
		case cmdHeaders:
			headers := msg.Payload.(*payload.Headers)
			err = s.handleHeadersCmd(headers, p)
		default:
			// This command is unknown by the server.
			err = fmt.Errorf("unknown command received %v", msg.Command)
			break
		}

		// catch all errors here and disconnect.
		if err != nil {
			s.logger.Printf("processing message failed: %s", err)
			break
		}
	}

	// Disconnect the peer when breaked out of the loop.
	p.disconnect()
}

type sendTuple struct {
	msg *Message
	err chan error
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
	send chan sendTuple
	// channel to receive from underlying connection.
	receive chan *Message
	// the version sended out by the peer when connected.
	pVersion *payload.Version
}

// NewTCPPeer returns a pointer to a TCP Peer.
func NewTCPPeer(conn net.Conn, s *Server) *TCPPeer {
	e, _ := util.EndpointFromString(conn.RemoteAddr().String())

	return &TCPPeer{
		conn:     conn,
		send:     make(chan sendTuple),
		receive:  make(chan *Message),
		endpoint: e,
		s:        s,
	}
}

func (p *TCPPeer) callVersion(msg *Message) error {
	t := sendTuple{
		msg: msg,
		err: make(chan error),
	}

	p.send <- t

	return <-t.err
}

func (p *TCPPeer) version() *payload.Version {
	return p.pVersion
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
func (p *TCPPeer) callGetaddr(msg *Message) error {
	t := sendTuple{
		msg: msg,
		err: make(chan error),
	}

	p.send <- t

	return <-t.err
}

// callGetblocks will send the "getblocks" command to the remote.
func (p *TCPPeer) callGetblocks(msg *Message) error {
	t := sendTuple{
		msg: msg,
		err: make(chan error),
	}

	p.send <- t

	return <-t.err
}

// callGetheaders will send the "getheaders" command to the remote.
func (p *TCPPeer) callGetheaders(msg *Message) error {
	t := sendTuple{
		msg: msg,
		err: make(chan error),
	}

	p.send <- t

	return <-t.err
}

func (p *TCPPeer) callVerack(msg *Message) error {
	t := sendTuple{
		msg: msg,
		err: make(chan error),
	}

	p.send <- t

	return <-t.err
}

func (p *TCPPeer) callGetdata(msg *Message) error {
	t := sendTuple{
		msg: msg,
		err: make(chan error),
	}

	p.send <- t

	return <-t.err
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

	// resuse this buffer
	buf := new(bytes.Buffer)
	for {
		t := <-p.send
		if t.msg == nil {
			break // send probably closed.
		}

		// p.s.logger.Printf("OUT :: %s :: %+v", t.msg.commandType(), t.msg.Payload)

		if err := t.msg.encode(buf); err != nil {
			t.err <- err
		}
		_, err := p.conn.Write(buf.Bytes())
		t.err <- err

		buf.Reset()
	}
}
