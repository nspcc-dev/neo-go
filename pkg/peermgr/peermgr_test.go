package peermgr

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/command"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/stretchr/testify/assert"
)

type peer struct {
	quit             chan bool
	nonce            int
	disconnected     bool
	blockRequested   int
	headersRequested int
}

func (p *peer) Disconnect() {
	p.disconnected = true
	p.quit <- true
}
func (p *peer) RequestBlocks([]util.Uint256) error {
	p.blockRequested++
	return nil
}
func (p *peer) RequestHeaders(util.Uint256) error {
	p.headersRequested++
	return nil
}
func (p *peer) NotifyDisconnect() {
	<-p.quit
}

func TestAddPeer(t *testing.T) {
	pmgr := New()

	peerA := &peer{nonce: 1}
	peerB := &peer{nonce: 2}
	peerC := &peer{nonce: 3}

	pmgr.AddPeer(peerA)
	pmgr.AddPeer(peerB)
	pmgr.AddPeer(peerC)
	pmgr.AddPeer(peerC)

	assert.Equal(t, 3, pmgr.Len())
}

func TestRequestBlocks(t *testing.T) {
	pmgr := New()

	peerA := &peer{nonce: 1}
	peerB := &peer{nonce: 2}
	peerC := &peer{nonce: 3}

	pmgr.AddPeer(peerA)
	pmgr.AddPeer(peerB)
	pmgr.AddPeer(peerC)

	err := pmgr.RequestBlock(util.Uint256{})
	assert.Nil(t, err)

	err = pmgr.RequestBlock(util.Uint256{})
	assert.Nil(t, err)

	err = pmgr.RequestBlock(util.Uint256{})
	assert.Nil(t, err)

	// Since the peer manager did not get a MsgReceived
	// in between the block requests
	// a request should be sent to all peers

	assert.Equal(t, 1, peerA.blockRequested)
	assert.Equal(t, 1, peerB.blockRequested)
	assert.Equal(t, 1, peerC.blockRequested)

	// Since the peer manager still has not received a MsgReceived
	// another call to request blocks, will return a NoAvailablePeerError

	err = pmgr.RequestBlock(util.Uint256{})
	assert.Equal(t, ErrNoAvailablePeers, err)

	// If we tell the peer manager that peerA has given us a block
	// then send another BlockRequest. It will go to peerA
	// since the other two peers are still busy with their
	// block requests

	pmgr.MsgReceived(peerA, command.Block)
	err = pmgr.RequestBlock(util.Uint256{})
	assert.Nil(t, err)

	assert.Equal(t, 2, peerA.blockRequested)
	assert.Equal(t, 1, peerB.blockRequested)
	assert.Equal(t, 1, peerC.blockRequested)
}
func TestRequestHeaders(t *testing.T) {
	pmgr := New()

	peerA := &peer{nonce: 1}
	peerB := &peer{nonce: 2}
	peerC := &peer{nonce: 3}

	pmgr.AddPeer(peerA)
	pmgr.AddPeer(peerB)
	pmgr.AddPeer(peerC)

	err := pmgr.RequestHeaders(util.Uint256{})
	assert.Nil(t, err)

	err = pmgr.RequestHeaders(util.Uint256{})
	assert.Nil(t, err)

	err = pmgr.RequestHeaders(util.Uint256{})
	assert.Nil(t, err)

	// Since the peer manager did not get a MsgReceived
	// in between the header requests
	// a request should be sent to all peers

	assert.Equal(t, 1, peerA.headersRequested)
	assert.Equal(t, 1, peerB.headersRequested)
	assert.Equal(t, 1, peerC.headersRequested)

	// Since the peer manager still has not received a MsgReceived
	// another call to request header, will return a NoAvailablePeerError

	err = pmgr.RequestHeaders(util.Uint256{})
	assert.Equal(t, ErrNoAvailablePeers, err)

	// If we tell the peer manager that peerA has given us a block
	// then send another BlockRequest. It will go to peerA
	// since the other two peers are still busy with their
	// block requests

	err = pmgr.MsgReceived(peerA, command.Headers)
	assert.Nil(t, err)
	err = pmgr.RequestHeaders(util.Uint256{})
	assert.Nil(t, err)

	assert.Equal(t, 2, peerA.headersRequested)
	assert.Equal(t, 1, peerB.headersRequested)
	assert.Equal(t, 1, peerC.headersRequested)
}

func TestUnknownPeer(t *testing.T) {
	pmgr := New()

	unknownPeer := &peer{
		disconnected: false,
		quit:         make(chan bool),
	}

	err := pmgr.MsgReceived(unknownPeer, command.Block)
	assert.Equal(t, true, unknownPeer.disconnected)
	assert.Equal(t, ErrUnknownPeer, err)
}

func TestNotifyDisconnect(t *testing.T) {
	pmgr := New()

	peerA := &peer{
		nonce: 1,
		quit:  make(chan bool),
	}

	pmgr.AddPeer(peerA)

	if pmgr.Len() != 1 {
		t.Fail()
	}

	peerA.Disconnect()

	if pmgr.Len() != 0 {
		t.Fail()
	}
}
