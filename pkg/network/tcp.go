package network

import (
	"io"
	"log"
	"net"
)

func listenTCP(s *Server, port string) error {
	ln, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go handleConnection(s, conn, true)
	}
}

func connectToSeeds(s *Server, addrs []string) {
	for _, addr := range addrs {
		go func(addr string) {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				log.Printf("failed to connect to remote node %s: %s", addr, err)
				if conn != nil {
					conn.Close()
				}
				return
			}
			go handleConnection(s, conn, false)
		}(addr)
	}
}

func handleConnection(s *Server, conn net.Conn, initiated bool) {
	peer := NewPeer(conn, initiated)
	s.register <- peer

	// remove the peer from connected peers and cleanup the connection.
	defer func() {
		s.unregister <- peer
		conn.Close()
	}()

	// Start a goroutine that will handle all writes to the registered peer.
	go peer.writeLoop()

	// Read from the connection and decode it into an RPCMessage and
	// tell the server there is message available for proccesing.
	msg := &Message{}
	for {
		if err := msg.decode(conn); err != nil {
			// remote connection probably closed.
			if err == io.EOF {
				s.logger.Printf("conn read error: %s", err)
				break
			}
			// remove this node on any decode errors.
			s.logger.Printf("RPC :: decode error %s", err)
			break
		}
		s.message <- messageTuple{peer, msg}
	}
}
