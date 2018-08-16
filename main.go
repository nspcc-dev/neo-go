package main

import (
	"fmt"
	"net"

	"github.com/CityOfZion/neo-go/pkg/p2p/peer"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

func main() {
	// peersConnectToMe()
	connectingToPeers()
}

func peersConnectToMe() {
	listener, err := net.Listen("tcp", ":20338")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer func() {
		listener.Close()
		fmt.Println("Listener closed")
	}()

	for {
		conn, err := listener.Accept()
		fmt.Println("Connectioin accepted")
		if err != nil {
			fmt.Println("Error connecting to peer, ", err)
		}

		p := peer.NewPeer(conn, true, peer.DefaultConfig())
		err = p.Handshake()
		go p.StartProtocol()

		getHeaders, err := payload.NewGetHeadersMessage([]util.Uint256{}, util.Uint256{})
		err = p.Write(getHeaders)
		if err != nil {
			fmt.Println("Error writing message ", err)
		}

		p.ReadLoop()

		return

	}
}
func connectingToPeers() {
	conn, err := net.Dial("tcp", "seed2.neo.org:10333")
	if err != nil {
		fmt.Println("Error dialing connection", err.Error())
		return
	}
	p := peer.NewPeer(conn, false, peer.DefaultConfig())
	err = p.Handshake()
	go p.StartProtocol()
	go p.ReadLoop()

	hash, err := util.Uint256DecodeString("d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")
	// hash2, err := util.Uint256DecodeString("ff8fe95efc5d1cc3a22b17503aecaf289cef68f94b79ddad6f613569ca2342d8")
	err = p.RequestHeaders(hash)
	if err != nil {
		fmt.Println(err.Error())
	}

	<-make(chan struct{})

}
