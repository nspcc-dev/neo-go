package server

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/peermgr"

	"github.com/CityOfZion/neo-go/pkg/chain"
	"github.com/CityOfZion/neo-go/pkg/connmgr"
	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/syncmgr"

	"github.com/CityOfZion/neo-go/pkg/database"

	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

// Server orchestrates all of the modules
type Server struct {
	net    protocol.Magic
	stopCh chan error

	// Modules
	db    database.Database
	smg   *syncmgr.Syncmgr
	cmg   *connmgr.Connmgr
	pmg   *peermgr.PeerMgr
	chain *chain.Chain

	peerCfg *peer.LocalConfig
}

//New creates a new server object for a particular network and sets up each module
func New(net protocol.Magic, port uint16) (*Server, error) {
	s := &Server{
		net:    net,
		stopCh: make(chan error, 0),
	}

	// Setup database
	db, err := setupDatabase(net)
	if err != nil {
		return nil, err
	}
	s.db = db

	// setup peermgr
	peermgr := setupPeerManager()
	s.pmg = peermgr

	// Setup chain
	chain, err := setupChain(db, net)
	if err != nil {
		return nil, err
	}
	s.chain = chain

	// Setup sync manager
	syncmgr, err := setupSyncManager(s)
	if err != nil {
		return nil, err
	}
	s.smg = syncmgr

	// Setup connection manager
	connmgr, err := setupConnManager(s, port)
	if err != nil {
		return nil, err
	}
	s.cmg = connmgr

	// Setup peer config
	peerCfg := setupPeerConfig(s, port, net)
	s.peerCfg = peerCfg

	return s, nil
}

// Run starts the daemon by connecting to previously nodes or connectng to seed nodes.
// This should be called once all modules have been setup
func (s *Server) Run() error {
	fmt.Println("Server is starting up")

	// start the connmgr
	err := s.cmg.Run()
	if err != nil {
		return err
	}

	// Attempt to connect to a peer
	err = s.cmg.NewRequest()
	if err != nil {
		return err
	}

	// Request header to start synchronisation
	bestHeader, err := s.chain.Db.GetLastHeader()
	if err != nil {
		return err
	}

	err = s.pmg.RequestHeaders(bestHeader.Hash)
	if err != nil {
		return err
	}
	fmt.Println("Server Successfully started")
	return s.wait()
}

func (s *Server) wait() error {
	err := <-s.stopCh
	return err
}

// Stop stops the server
func (s *Server) Stop(err error) error {
	fmt.Println("Server is shutting down")
	s.stopCh <- err
	return nil
}
