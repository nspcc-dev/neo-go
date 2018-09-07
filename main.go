package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/CityOfZion/neo-go/pkg/blockchain"
	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/peermanager"
	"github.com/CityOfZion/neo-go/pkg/syncmanager"
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

	conn, err := net.DialTimeout("tcp", "seed1.ngd.network:10333", dialTimeout)
	if err != nil {
		fmt.Println("Error dialing connection", err.Error())
		return
	}

	// setup DB
	db := database.New("test")
	// setup blockchain
	chain := blockchain.New(db, protocol.MainNet)

	var res util.Uint256

	if chain != nil {
		fmt.Println("Db Successfully initialised")
		table := database.NewTable(db, database.HEADER)
		resa, err := table.Get(database.LATESTHEADER)
		res, err = util.Uint256DecodeBytes(resa)
		if err != nil {
			fmt.Println("Failed to get LastHeader")
			return
		}
		fmt.Println(hex.EncodeToString(resa))
	} else {
		fmt.Println("Failed to add genesis block")
	}

	// setup peerManager
	pm := peermanager.New()

	// setup syncmanager
	sm := syncmanager.New(chain, pm, res)

	// This should be configured on the server, then we pass it around
	config := peer.LocalConfig{
		Net:         protocol.MainNet,
		UserAgent:   "DIG",
		Services:    protocol.NodePeerService,
		Nonce:       1200,
		ProtocolVer: 0,
		Relay:       false,
		Port:        10332,
		StartHeight: LocalHeight,
		OnHeader:    sm.OnHeaders,
		OnBlock:     sm.OnBlock,
	}

	p := peer.NewPeer(conn, false, config)
	err = p.Run()

	if err != nil {
		pm.AddPeer(&p)
	}

	// hash, err := util.Uint256DecodeString(chainparams.GenesisHash)
	// if err != nil {
	// 	fmt.Println("Error converting hex to hash", err)
	// 	return
	// }
	// fmt.Println(hash.Bytes())
	err = p.RequestHeaders(res)
	fmt.Println("For tests, we are only fetching first 2k batch")
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
	if len(msg.Headers) == 2000 { // reached tip
		lastHeader := msg.Headers[len(msg.Headers)-1]

		fmt.Println("Latest hash is", lastHeader.Hash.String())
		fmt.Println("Latest Header height is", lastHeader.Index)

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
