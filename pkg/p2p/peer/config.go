package peer

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

// LocalConfig specifies the properties that should be available for each remote peer

type LocalConfig struct {
	net         protocol.Magic
	userAgent   string
	services    protocol.ServiceFlag
	nonce       uint32
	protocolVer protocol.Version
	relay       bool
	port        uint16
	// pointer to config will keep the startheight updated for each version
	//Message we plan to send
	startHeight uint32
	Listener
}

func DefaultConfig() LocalConfig {
	return LocalConfig{
		net:         protocol.MainNet,
		userAgent:   "NEO-GO-Default",
		services:    protocol.NodePeerService,
		nonce:       1200,
		protocolVer: 0,
		relay:       false,
		port:        10332,
		// pointer to config will keep the startheight updated for each version
		//Message we plan to send
		startHeight: 10,
	}
}

type Listener struct {
	BlockMessageListener
	TXMessageListener
	HeadersMessageListener
	AddressMessageListener
	InvMessageListener
}

// Will be handled by blockmanager
type BlockMessageListener interface {
	OnBlock(msg *payload.BlockMessage)
	OnGetBlocks(msg *payload.GetBlocksMessage)
	// If a node receives a getblocks message, he will reply with an InvMessage, type
}
type TXMessageListener interface {
	OnTX(msg *payload.TXMessage)
}

// Also handled by blockmanager
type HeadersMessageListener interface {
	OnHeader(msg *payload.HeadersMessage)
	OnGetHeaders(msg *payload.GetHeadersMessage) // returns HeaderMessage
}

// Will be handled by addressmanager
type AddressMessageListener interface {
	OnAddr(msg *payload.AddrMessage)
	OnGetAddr(msg *payload.GetAddrMessage) // returns Addr
}

type InvMessageListener interface {
	OnInv(msg *payload.InvMessage)         // mempool for transaction, blockmanager for block, consensus not sure as not implemented yet
	OnGetData(msg *payload.GetDataMessage) // mempool for transactions, bm for blocks
}
