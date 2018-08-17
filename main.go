package main

import (
	"fmt"
	"net"
	"time"

	"github.com/CityOfZion/neo-go/pkg/chainparams"
	"github.com/CityOfZion/neo-go/pkg/p2p/peer"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/CityOfZion/neo-go/pkg/wire/util/io"
)

func main() {
	// peersConnectToMe()
	connectingToPeers()
}

func connectingToPeers() {

	dialTimeout := 5 * time.Second

	conn, err := net.DialTimeout("tcp", "seed2.neo.org:10333", dialTimeout)
	if err != nil {
		fmt.Println("Error dialing connection", err.Error())
		return
	}

	config := peer.LocalConfig{
		Net:         protocol.MainNet,
		UserAgent:   "NEO-G",
		Services:    protocol.NodePeerService,
		Nonce:       1200,
		ProtocolVer: 0,
		Relay:       false,
		Port:        10332,
		StartHeight: LocalHeight,
		OnHeader:    OnHeader,
	}

	p := peer.NewPeer(conn, false, config)
	err = p.Run()

	hash, err := util.Uint256DecodeString(chainparams.GenesisHash)
	// hash2, err := util.Uint256DecodeString("ff8fe95efc5d1cc3a22b17503aecaf289cef68f94b79ddad6f613569ca2342d8")
	err = p.RequestHeaders(hash)
	if err != nil {
		fmt.Println(err.Error())
	}

	<-make(chan struct{})

}

func OnHeader(peer *peer.Peer, msg *payload.HeadersMessage) {

	for _, header := range msg.Headers {
		if err := fileutils.UpdateFile("headers.txt", []byte(header.Hash.String())); err != nil {
			fmt.Println("Error writing headers to file")
			break
		}
	}
	if len(msg.Headers) > 100 {
		lastHeader := msg.Headers[len(msg.Headers)-1]
		fmt.Println("Latest hash is", lastHeader.Hash.String())
		err := peer.RequestHeaders(lastHeader.Hash.Reverse())
		if err != nil {
			fmt.Println("Error getting more headers", err)
		}
	}
}

func LocalHeight() uint32 {
	return 10
}

// func peersConnectToMe() {
// 	listener, err := net.Listen("tcp", ":20338")
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}

// 	defer func() {
// 		listener.Close()
// 		fmt.Println("Listener closed")
// 	}()

// 	for {
// 		conn, err := listener.Accept()
// 		fmt.Println("Connectioin accepted")
// 		if err != nil {
// 			fmt.Println("Error connecting to peer, ", err)
// 		}

// 		p := peer.NewPeer(conn, true, peer.DefaultConfig())
// 		err = p.Handshake()
// 		go p.StartProtocol()

// 		getHeaders, err := payload.NewGetHeadersMessage([]util.Uint256{}, util.Uint256{})
// 		err = p.Write(getHeaders)
// 		if err != nil {
// 			fmt.Println("Error writing message ", err)
// 		}

// 		go p.ReadLoop()
// 		go p.WriteLoop()
// 	}
// }
