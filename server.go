package main

import (
	"errors"
	"fmt"
	"net"

	"github.com/CityOfZion/neo-go/pkg/blockchain"
	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/syncmanager"

	"github.com/CityOfZion/neo-go/pkg/connmgr"
	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/CityOfZion/neo-go/pkg/wire/util/io"
)

// this file will act as a stub server
// Will create a server package

type Server struct {
	chain *blockchain.Chain
	db    *database.LDB // TODO(Kev) change to database.Database
	sm    *syncmanager.Syncmanager
	cm    *connmgr.Connmgr

	peercfg peer.LocalConfig

	latestHash util.Uint256
}

func (s *Server) setupConnMgr() error {
	// Connection Manager - Integrate
	s.cm = connmgr.New(connmgr.Config{
		GetAddress:   nil,
		OnConnection: s.OnConn,
		OnAccept:     nil,
		Port:         "10333",
	})

	return nil
}
func (s *Server) setupDatabase() error {
	// Database -- Integrate
	s.db = database.New("test")
	return nil
}
func (s *Server) setupChain() error {
	// Blockchain - Integrate
	s.chain = blockchain.New(s.db, protocol.MainNet)

	if s.chain != nil {
		table := database.NewTable(s.db, database.HEADER)
		resa, err := table.Get(database.LATESTHEADER)
		s.latestHash, err = util.Uint256DecodeBytes(resa)
		if err != nil {
			return errors.New("Failed to get LastHeader " + err.Error())
		}
	} else {
		return errors.New("Failed to add genesis block")
	}
	return nil
}
func (s *Server) setupSyncManager() error {
	// Sync Manager - Integrate
	s.sm = syncmanager.New(syncmanager.Config{
		Chain:    s.chain,
		BestHash: s.latestHash,
	})
	return nil
}
func (s *Server) setupPeerConfig() error {
	// Peer config struct - Integrate
	s.peercfg = peer.LocalConfig{
		Net:         protocol.MainNet,
		UserAgent:   "DIG",
		Services:    protocol.NodePeerService,
		Nonce:       1200,
		ProtocolVer: 0,
		Relay:       false,
		Port:        10332,
		StartHeight: LocalHeight,
		OnHeader:    s.sm.OnHeaders,
		OnBlock:     s.sm.OnBlock,
	}
	return nil
}

func (s *Server) Run() error {

	// Add all other run based methods for modules

	// Connmgr - Run
	s.cm.Run()
	// Initial hardcoded nodes to connect to
	err := s.cm.Connect(&connmgr.Request{
		Addr: "seed1.ngd.network:10333",
	})
	return err
}

func main() {

	setup()
}

func setup() {

	server := Server{}
	fmt.Println(server.sm)

	err := server.setupConnMgr()
	err = server.setupDatabase()
	err = server.setupChain()
	err = server.setupSyncManager()
	err = server.setupPeerConfig()

	fmt.Println(server.sm)

	err = server.Run()
	if err != nil {
		fmt.Println(err)
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

// OnConn is called when a successful connection has been made
func (s *Server) OnConn(conn net.Conn, addr string) {
	fmt.Println(conn.RemoteAddr().String())
	fmt.Println(addr)

	p := peer.NewPeer(conn, false, s.peercfg)
	err := p.Run()

	if err != nil {
		fmt.Println("Error running peer" + err.Error())
	}

	if err == nil {
		s.sm.AddPeer(&p)
	}

	// This is here just to quickly test the system
	err = p.RequestHeaders(s.latestHash)
	fmt.Println("For tests, we are only fetching first 2k batch")
	if err != nil {
		fmt.Println(err.Error())
	}
}
