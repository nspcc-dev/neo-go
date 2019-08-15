package peer

import (
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

// OnGetData is called when a GetData message is received
func (p *Peer) OnGetData(msg *payload.GetDataMessage) {
	p.inch <- func() {
		if p.config.OnInv != nil {
			p.config.OnGetData(p, msg)
		}
	}
}

//OnTX is called when a TX message is received
func (p *Peer) OnTX(msg *payload.TXMessage) {
	p.inch <- func() {
		p.inch <- func() {
			if p.config.OnTx != nil {
				p.config.OnTx(p, msg)
			}
		}
	}
}

// OnInv is called when a Inv message is received
func (p *Peer) OnInv(msg *payload.InvMessage) {
	p.inch <- func() {
		if p.config.OnInv != nil {
			p.config.OnInv(p, msg)
		}
	}
}

// OnGetHeaders is called when a GetHeaders message is received
func (p *Peer) OnGetHeaders(msg *payload.GetHeadersMessage) {
	p.inch <- func() {
		if p.config.OnGetHeaders != nil {
			p.config.OnGetHeaders(p, msg)
		}
	}
}

// OnAddr is called when a Addr message is received
func (p *Peer) OnAddr(msg *payload.AddrMessage) {
	p.inch <- func() {
		if p.config.OnAddr != nil {
			p.config.OnAddr(p, msg)
		}
	}
}

// OnGetAddr is called when a GetAddr message is received
func (p *Peer) OnGetAddr(msg *payload.GetAddrMessage) {
	p.inch <- func() {
		if p.config.OnGetAddr != nil {
			p.config.OnGetAddr(p, msg)
		}
	}
}

// OnGetBlocks is called when a GetBlocks message is received
func (p *Peer) OnGetBlocks(msg *payload.GetBlocksMessage) {
	p.inch <- func() {
		if p.config.OnGetBlocks != nil {
			p.config.OnGetBlocks(p, msg)
		}
	}
}

// OnBlocks is called when a Blocks message is received
func (p *Peer) OnBlocks(msg *payload.BlockMessage) {
	p.Detector.RemoveMessage(msg.Command())
	p.inch <- func() {
		if p.config.OnBlock != nil {
			p.config.OnBlock(p, msg)
		}
	}
}

// OnHeaders is called when a Headers message is received
func (p *Peer) OnHeaders(msg *payload.HeadersMessage) {
	p.Detector.RemoveMessage(msg.Command())
	p.inch <- func() {
		if p.config.OnHeader != nil {
			p.config.OnHeader(p, msg)
		}
	}
}

// OnVersion Listener will be called
// during the handshake, any error checking should be done here for the versionMessage.
// This should only ever be called during the handshake. Any other place and the peer will disconnect.
func (p *Peer) OnVersion(msg *payload.VersionMessage) error {
	// todo: figure out why this check should be here
	//if msg.Nonce == p.config.Nonce {
	//	p.conn.Close()
	//	return errors.New("self connection, disconnecting Peer")
	//}
	p.versionKnown = true
	p.port = msg.Port
	p.services = msg.Services
	p.userAgent = string(msg.UserAgent)
	p.createdAt = time.Now()
	p.relay = msg.Relay
	p.startHeight = msg.StartHeight
	return nil
}
