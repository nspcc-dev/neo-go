package network

import (
	"bytes"
	"net"

	"github.com/anthdm/neo-go/pkg/network/payload"
)

func listenTCP(s *Server, port string) error {
	ln, err := net.Listen("tcp", port)
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

	// Start a goroutine that will handle all writes to the registered peer.
	go peer.writeLoop()

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
		handleMessage(msg, s, peer)
	}
}

func handleMessage(msg *Message, s *Server, p *TCPPeer) {
	command := msg.commandType()

	s.logger.Printf("%d :: IN :: %s :: %v", p.id(), command, msg)

	switch command {
	case cmdVersion:
		resp := s.handleVersionCmd(msg, p)
		p.isVerack = true
		p.nonce = msg.Payload.(*payload.Version).Nonce
		p.send <- resp
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
		go s.sendLoop(p)
	case cmdGetHeaders:
	case cmdGetBlocks:
	case cmdGetData:
	case cmdHeaders:
	default:
	}
}
