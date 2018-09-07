package peer_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/peer"
	"github.com/CityOfZion/neo-go/pkg/wire"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/stretchr/testify/assert"
)

func returnConfig() peer.LocalConfig {

	DefaultHeight := func() uint32 {
		return 10
	}

	OnAddr := func(p *peer.Peer, msg *payload.AddrMessage) {}
	OnHeader := func(p *peer.Peer, msg *payload.HeadersMessage) {}
	OnGetHeaders := func(msg *payload.GetHeadersMessage) {}
	OnInv := func(p *peer.Peer, msg *payload.InvMessage) {}
	OnGetData := func(msg *payload.GetDataMessage) {}
	OnBlock := func(p *peer.Peer, msg *payload.BlockMessage) {}
	OnGetBlocks := func(msg *payload.GetBlocksMessage) {}

	return peer.LocalConfig{
		Net:         protocol.MainNet,
		UserAgent:   "NEO-GO-Default",
		Services:    protocol.NodePeerService,
		Nonce:       1200,
		ProtocolVer: 0,
		Relay:       false,
		Port:        10332,
		// pointer to config will keep the startheight updated for each version
		//Message we plan to send
		StartHeight:  DefaultHeight,
		OnHeader:     OnHeader,
		OnAddr:       OnAddr,
		OnGetHeaders: OnGetHeaders,
		OnInv:        OnInv,
		OnGetData:    OnGetData,
		OnBlock:      OnBlock,
		OnGetBlocks:  OnGetBlocks,
	}
}

func TestHandshake(t *testing.T) {
	address := ":20338"
	go func() {

		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		p := peer.NewPeer(conn, true, returnConfig())
		err = p.Run()
		verack, err := payload.NewVerackMessage()
		if err != nil {
			t.Fail()
		}
		if err := p.Write(verack); err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, true, p.IsVerackReceived())

	}()

	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
		return
	}

	defer func() {
		listener.Close()
	}()

	for {

		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}

		tcpAddrMe := &net.TCPAddr{IP: net.ParseIP("82.2.97.142"), Port: 20338}
		nonce := uint32(100)
		messageVer, err := payload.NewVersionMessage(tcpAddrMe, 2595770, false, protocol.DefaultVersion, protocol.UserAgent, nonce, protocol.NodePeerService)

		if err != nil {
			t.Fatal(err)
		}

		if err := wire.WriteMessage(conn, protocol.MainNet, messageVer); err != nil {
			t.Fatal(err)
			return
		}

		readmsg, err := wire.ReadMessage(conn, protocol.MainNet)
		if err != nil {
			t.Fatal(err)
		}
		version, ok := readmsg.(*payload.VersionMessage)
		if !ok {
			t.Fatal(err)
		}

		assert.NotEqual(t, nil, version)

		messageVrck, err := payload.NewVerackMessage()
		if err != nil {
			t.Fatal(err)
		}

		assert.NotEqual(t, nil, messageVrck)

		if err := wire.WriteMessage(conn, protocol.MainNet, messageVrck); err != nil {
			t.Fatal(err)
		}

		readmsg, err = wire.ReadMessage(conn, protocol.MainNet)
		if err != nil {
			t.Fatal(err)
		}

		assert.NotEqual(t, nil, readmsg)

		verk, ok := readmsg.(*payload.VerackMessage)
		if !ok {
			t.Fatal(err)
		}
		assert.NotEqual(t, nil, verk)

		return
	}

}

func TestConfigurations(t *testing.T) {
	_, conn := net.Pipe()

	inbound := true

	config := returnConfig()

	p := peer.NewPeer(conn, inbound, config)

	// test inbound
	assert.Equal(t, inbound, p.Inbound())

	// handshake not done, should be false
	assert.Equal(t, false, p.IsVerackReceived())

	assert.Equal(t, config.Services, p.Services())

	assert.Equal(t, config.UserAgent, p.UserAgent())

	assert.Equal(t, config.Relay, p.CanRelay())

	assert.WithinDuration(t, time.Now(), p.CreatedAt(), 1*time.Second)

}

func TestHandshakeCancelled(t *testing.T) {
	// These are the conditions which should invalidate the handshake.
	// Make sure peer is disconnected.
}

func TestPeerDisconnect(t *testing.T) {
	// Make sure everything is shutdown
	// Make sure timer is shutdown in stall detector too. Should maybe put this part of test into stall detector.

	_, conn := net.Pipe()
	inbound := true
	config := returnConfig()
	p := peer.NewPeer(conn, inbound, config)
	fmt.Println("Calling disconnect")
	p.Disconnect()
	fmt.Println("Disconnect finished calling")
	verack, _ := payload.NewVerackMessage()

	fmt.Println(" We good here")

	err := p.Write(verack)

	assert.NotEqual(t, err, nil)

	// Check if Stall detector is still running
	_, ok := <-p.Detector.Quitch
	assert.Equal(t, ok, false)

}
