package peer

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

// LocalConfig specifies the properties that should be available for each remote peer

type LocalConfig struct {
	Net         protocol.Magic
	UserAgent   string
	Services    protocol.ServiceFlag
	Nonce       uint32
	ProtocolVer protocol.Version
	Relay       bool
	Port        uint16
	// pointer to config will keep the startheight updated for each version
	//Message we plan to send
	StartHeight  func() uint32
	OnHeader     func(*Peer, *payload.HeadersMessage)
	OnGetHeaders func(msg *payload.GetHeadersMessage) // returns HeaderMessage
	OnAddr       func(*Peer, *payload.AddrMessage)
	OnGetAddr    func(*Peer, *payload.GetAddrMessage)
	OnInv        func(*Peer, *payload.InvMessage)
	OnGetData    func(msg *payload.GetDataMessage)
	OnBlock      func(*Peer, *payload.BlockMessage)
	OnGetBlocks  func(msg *payload.GetBlocksMessage)
}

// func DefaultConfig() LocalConfig {
// 	return LocalConfig{
// 		Net:         protocol.MainNet,
// 		UserAgent:   "NEO-GO-Default",
// 		Services:    protocol.NodePeerService,
// 		Nonce:       1200,
// 		ProtocolVer: 0,
// 		Relay:       false,
// 		Port:        10332,
// 		// pointer to config will keep the startheight updated for each version
// 		//Message we plan to send
// 		StartHeight: DefaultHeight,
// 	}
// }

// func DefaultHeight() uint32 {
// 	return 10
// }
