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

	// pointer to config will keep the startheight updated
	StartHeight func() uint32

	// Response Handlers
	OnHeader     func(*Peer, *payload.HeadersMessage)
	OnGetHeaders func(*Peer, *payload.GetHeadersMessage)
	OnAddr       func(*Peer, *payload.AddrMessage)
	OnGetAddr    func(*Peer, *payload.GetAddrMessage)
	OnInv        func(*Peer, *payload.InvMessage)
	OnGetData    func(*Peer, *payload.GetDataMessage)
	OnBlock      func(*Peer, *payload.BlockMessage)
	OnGetBlocks  func(*Peer, *payload.GetBlocksMessage)
	OnTx         func(*Peer, *payload.TXMessage)
}
