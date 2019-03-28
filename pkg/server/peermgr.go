package server

import (
	"github.com/CityOfZion/neo-go/pkg/peermgr"
)

func setupPeerManager() *peermgr.PeerMgr {
	return peermgr.New()
}
