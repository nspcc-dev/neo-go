package peer

import (
	"fmt"
	"net"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

func (p *Peer) Handshake() error {

	handshakeErr := make(chan error, 1)
	go func() {
		if p.inbound {
			handshakeErr <- p.inboundHandShake()
		} else {
			fmt.Println("P is outbound, so doing outbound hs")
			handshakeErr <- p.outboundHandShake()
		}
	}()

	select {
	case err := <-handshakeErr:
		if err != nil {
			return err
		}
	case <-time.After(handshakeTimeout):
		return errHandShakeTimeout
	}
	fmt.Println("hanshake with", p.addr, "successful")
	return nil
}

// If this peer has an inbound conn (conn that is going into another peer)
// then he has dialed and so, we must read the version message
func (p *Peer) inboundHandShake() error {
	var err error
	if err := p.writeLocalVersionMSG(); err != nil {
		return err
	}
	err = p.readRemoteVersionMSG()
	if err != nil {
		return err
	}
	verack, err := payload.NewVerackMessage()
	if err != nil {
		return err
	}
	return p.Write(verack)
}
func (p *Peer) outboundHandShake() error {
	var err error
	err = p.readRemoteVersionMSG()

	if err != nil {
		return err
	}
	err = p.writeLocalVersionMSG()
	if err != nil {
		return err
	}
	verack, err := payload.NewVerackMessage()
	if err != nil {
		return err
	}
	return p.Write(verack)
}
func (p *Peer) writeLocalVersionMSG() error {

	nonce := p.config.nonce
	relay := p.config.relay
	port := int(p.config.port)
	ua := p.config.userAgent
	sh := p.config.startHeight
	services := p.config.services
	proto := p.config.protocolVer
	ip := GetLocalIP()
	tcpAddrMe := &net.TCPAddr{IP: ip, Port: port}

	messageVer, err := payload.NewVersionMessage(tcpAddrMe, sh, relay, proto, ua, nonce, services)

	if err != nil {
		return err
	}
	return p.Write(messageVer)
}

func (p *Peer) readRemoteVersionMSG() error {
	readmsg, err := wire.ReadMessage(p.conn, p.config.net)
	if err != nil {
		return err
	}

	version, ok := readmsg.(*payload.VersionMessage)
	if !ok {
		return err
	}
	// TODO: validation checks on version message
	// setMin of LR
	_ = version

	return nil
}
