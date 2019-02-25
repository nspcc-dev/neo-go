package peer

import (
	"fmt"
	"net"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/util/ip"
)

func (p *Peer) Handshake() error {

	handshakeErr := make(chan error, 1)
	go func() {
		if p.inbound {
			handshakeErr <- p.inboundHandShake()
		} else {
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

	// This is purely here for Logs
	if p.inbound {
		fmt.Println("inbound handshake with", p.RemoteAddr().String(), "successful")
	} else {

		fmt.Println("outbound handshake with", p.RemoteAddr().String(), "successful")
	}
	return nil
}

// If this peer has an inbound conn (conn that is going into another peer)
// then he has dialed and so, we must read the version message
func (p *Peer) inboundHandShake() error {
	var err error
	if err := p.writeLocalVersionMSG(); err != nil {
		return err
	}
	if err := p.readRemoteVersionMSG(); err != nil {
		return err
	}
	verack, err := payload.NewVerackMessage()
	if err != nil {
		return err
	}
	err = p.Write(verack)
	return p.readVerack()
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

	err = p.readVerack()
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

	nonce := p.config.Nonce
	relay := p.config.Relay
	port := int(p.config.Port)
	ua := p.config.UserAgent
	sh := p.config.StartHeight()
	services := p.config.Services
	proto := p.config.ProtocolVer
	ip := iputils.GetLocalIP()
	tcpAddrMe := &net.TCPAddr{IP: ip, Port: port}

	messageVer, err := payload.NewVersionMessage(tcpAddrMe, sh, relay, proto, ua, nonce, services)

	if err != nil {
		return err
	}
	return p.Write(messageVer)
}

func (p *Peer) readRemoteVersionMSG() error {
	readmsg, err := wire.ReadMessage(p.conn, p.config.Net)
	if err != nil {
		return err
	}

	version, ok := readmsg.(*payload.VersionMessage)
	if !ok {
		return err
	}
	return p.OnVersion(version)
}

func (p *Peer) readVerack() error {
	readmsg, err := wire.ReadMessage(p.conn, p.config.Net)

	if err != nil {
		return err
	}

	_, ok := readmsg.(*payload.VerackMessage)

	if !ok {
		return err
	}
	// should only be accessed on one go-routine
	p.verackReceived = true

	return nil
}
