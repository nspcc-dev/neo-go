package server

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"

	"github.com/CityOfZion/neo-go/pkg/connmgr"

	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	iputils "github.com/CityOfZion/neo-go/pkg/wire/util/ip"
)

func setupConnManager(s *Server, port uint16) *connmgr.Connmgr {
	cfg := connmgr.Config{
		GetAddress:   s.getAddress,
		OnAccept:     s.onAccept,
		OnConnection: s.onConnection,
		AddressPort:  iputils.GetLocalIP().String() + ":" + strconv.FormatUint(uint64(port), 10),
	}
	return connmgr.New(cfg)
}

func (s *Server) onConnection(conn net.Conn, addr string) {
	fmt.Println("We have connected successfully to: ", addr)

	p := peer.NewPeer(conn, false, *s.peerCfg)
	err := p.Run()
	if err != nil {
		fmt.Println("Error running peer" + err.Error())
		return
	}

	s.pmg.AddPeer(p)

	byt, err := hex.DecodeString("d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")
	if err != nil {
		fmt.Println("Error getting hash " + err.Error())
	}
	lh, err := util.Uint256DecodeBytes(byt)
	if err != nil {
		fmt.Println("Error getting hash " + err.Error())
	}
	err = p.RequestHeaders(lh.Reverse())
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) onAccept(conn net.Conn) {
	fmt.Println("A peer with address: ", conn.RemoteAddr().String(), "has connect to us")

	p := peer.NewPeer(conn, true, *s.peerCfg)
	err := p.Run()
	if err != nil {
		fmt.Println("Error running peer" + err.Error())
		return
	}
	s.pmg.AddPeer(p)
}
