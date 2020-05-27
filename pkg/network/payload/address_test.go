package payload

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeAddress(t *testing.T) {
	var (
		e, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:2000")
		ts   = time.Now()
		addr = NewAddressAndTime(e, ts, capability.Capabilities{
			{
				Type: capability.TCPServer,
				Data: &capability.Server{Port: uint16(e.Port)},
			},
		})
	)

	assert.Equal(t, ts.UTC().Unix(), int64(addr.Timestamp))
	aatip := make(net.IP, 16)
	copy(aatip, addr.IP[:])
	assert.Equal(t, e.IP, aatip)
	assert.Equal(t, 1, len(addr.Capabilities))
	assert.Equal(t, capability.Capability{
		Type: capability.TCPServer,
		Data: &capability.Server{Port: uint16(e.Port)},
	}, addr.Capabilities[0])

	testserdes.EncodeDecodeBinary(t, addr, new(AddressAndTime))
}

func TestEncodeDecodeAddressList(t *testing.T) {
	var lenList uint8 = 4
	addrList := NewAddressList(int(lenList))
	for i := 0; i < int(lenList); i++ {
		e, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:200%d", i))
		addrList.Addrs[i] = NewAddressAndTime(e, time.Now(), capability.Capabilities{
			{
				Type: capability.TCPServer,
				Data: &capability.Server{Port: 123},
			},
		})
	}

	testserdes.EncodeDecodeBinary(t, addrList, new(AddressList))
}
