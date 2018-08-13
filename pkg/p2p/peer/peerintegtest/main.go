package main

import (
	"fmt"
	"net"

	"github.com/CityOfZion/neo-go/pkg/p2p/peer"
)

func main() {
	conn, err := net.Dial("tcp", ":20338")
	if err != nil {
		fmt.Println("error on connection", err.Error())
	}

	config := peer.DefaultConfig()
	p := peer.NewPeer(conn, false, config)
	fmt.Println("Starting handshake")
	err = p.Handshake()
	go p.StartProtocol()
	p.ReadLoop()

	fmt.Println("Hanshake err", err)
}
