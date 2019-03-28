package server

import (
	"math/rand"

	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

func setupPeerConfig(s *Server, port uint16, net protocol.Magic) *peer.LocalConfig {
	return &peer.LocalConfig{
		Net:         net,
		UserAgent:   "NEO-GO",
		Services:    protocol.NodePeerService,
		Nonce:       rand.Uint32(),
		ProtocolVer: 0,
		Relay:       false,
		Port:        port,
		StartHeight: s.chain.CurrentHeight,
		OnHeader:    s.onHeader,
		OnBlock:     s.onBlock,
	}
}
