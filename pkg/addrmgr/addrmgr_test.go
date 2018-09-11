package addrmgr_test

import (
	"crypto/rand"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"

	"github.com/CityOfZion/neo-go/pkg/addrmgr"
	"github.com/stretchr/testify/assert"
)

func TestNewAddrs(t *testing.T) {

	addrmgr := addrmgr.New()
	assert.NotEqual(t, nil, addrmgr)
}
func TestAddAddrs(t *testing.T) {

	addrmgr := addrmgr.New()

	ip := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	addr, _ := payload.NewNetAddr(0, ip, 1033, protocol.NodePeerService)

	ip = [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // same
	addr2, _ := payload.NewNetAddr(0, ip, 1033, protocol.NodePeerService)

	ip = [16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr3, _ := payload.NewNetAddr(0, ip, 1033, protocol.NodePeerService)

	addrs := []*payload.Net_addr{addr, addr2, addr, addr3}

	addrmgr.AddAddrs(addrs)

	assert.Equal(t, 2, len(addrmgr.Unconnected()))
	assert.Equal(t, 0, len(addrmgr.Good()))
	assert.Equal(t, 0, len(addrmgr.Bad()))
}

func TestFetchMoreAddress(t *testing.T) {

	addrmgr := addrmgr.New()

	addrs := []*payload.Net_addr{}

	ip := make([]byte, 16)

	for i := 0; i <= 2000; i++ { // Add more than maxAllowedAddrs
		rand.Read(ip)

		var nip [16]byte
		copy(nip[:], ip[:16])

		addr, _ := payload.NewNetAddr(0, nip, 1033, protocol.NodePeerService)
		addrs = append(addrs, addr)
	}

	addrmgr.AddAddrs(addrs)

	assert.Equal(t, false, addrmgr.FetchMoreAddresses())

}
func TestConnComplete(t *testing.T) {

	addrmgr := addrmgr.New()

	ip := [16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr, _ := payload.NewNetAddr(0, ip, 1033, protocol.NodePeerService)

	ip2 := [16]byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr2, _ := payload.NewNetAddr(0, ip2, 1033, protocol.NodePeerService)

	ip3 := [16]byte{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr3, _ := payload.NewNetAddr(0, ip3, 1033, protocol.NodePeerService)

	addrs := []*payload.Net_addr{addr, addr2, addr3}

	addrmgr.AddAddrs(addrs)

	assert.Equal(t, len(addrs), len(addrmgr.Unconnected()))

	// a successful connection
	addrmgr.ConnectionComplete(addr.IPPort(), true)
	addrmgr.ConnectionComplete(addr.IPPort(), true) // should have no change

	assert.Equal(t, len(addrs)-1, len(addrmgr.Unconnected()))
	assert.Equal(t, 1, len(addrmgr.Good()))

	// another successful connection
	addrmgr.ConnectionComplete(addr2.IPPort(), true)

	assert.Equal(t, len(addrs)-2, len(addrmgr.Unconnected()))
	assert.Equal(t, 2, len(addrmgr.Good()))

}
func TestAttempted(t *testing.T) {

	addrmgr := addrmgr.New()

	ip := [16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr, _ := payload.NewNetAddr(0, ip, 1033, protocol.NodePeerService)

	addrs := []*payload.Net_addr{addr}

	addrmgr.AddAddrs(addrs)

	addrmgr.Failed(addr.IPPort())

	assert.Equal(t, 1, len(addrmgr.Bad())) // newAddrs was attmepted and failed. Move to Bad

}
func TestAttemptedMoveFromGoodToBad(t *testing.T) {

	addrmgr := addrmgr.New()

	ip := [16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr, _ := payload.NewNetAddr(0, ip, 1043, protocol.NodePeerService)

	addrs := []*payload.Net_addr{addr}

	addrmgr.AddAddrs(addrs)

	addrmgr.ConnectionComplete(addr.IPPort(), true)
	addrmgr.ConnectionComplete(addr.IPPort(), true)
	addrmgr.ConnectionComplete(addr.IPPort(), true)

	assert.Equal(t, 1, len(addrmgr.Good()))

	addrmgr.Failed(addr.IPPort())
	addrmgr.Failed(addr.IPPort())
	addrmgr.Failed(addr.IPPort())
	addrmgr.Failed(addr.IPPort())
	addrmgr.Failed(addr.IPPort())
	addrmgr.Failed(addr.IPPort())
	addrmgr.Failed(addr.IPPort())
	addrmgr.Failed(addr.IPPort())
	addrmgr.Failed(addr.IPPort())
	// over threshhold, and will be classed as a badAddr L251

	assert.Equal(t, 0, len(addrmgr.Good()))
	assert.Equal(t, 1, len(addrmgr.Bad()))
}

func TestGetAddress(t *testing.T) {

	addrmgr := addrmgr.New()

	ip := [16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr, _ := payload.NewNetAddr(0, ip, 10333, protocol.NodePeerService)
	ip2 := [16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr2, _ := payload.NewNetAddr(0, ip2, 10334, protocol.NodePeerService)
	ip3 := [16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0} // different
	addr3, _ := payload.NewNetAddr(0, ip3, 10335, protocol.NodePeerService)

	addrs := []*payload.Net_addr{addr, addr2, addr3}

	addrmgr.AddAddrs(addrs)

	fetchAddr, err := addrmgr.NewAddr()
	assert.Equal(t, nil, err)

	ipports := []string{addr.IPPort(), addr2.IPPort(), addr3.IPPort()}

	assert.Contains(t, ipports, fetchAddr)

}
