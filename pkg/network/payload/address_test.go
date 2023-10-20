package payload

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	// On Windows or macOS localhost can be resolved to 4-bytes IPv4.
	expected := make(net.IP, 16)
	copy(expected, e.IP[:])

	aatip := make(net.IP, 16)
	copy(aatip, addr.IP[:])

	assert.Equal(t, expected, aatip)
	assert.Equal(t, 1, len(addr.Capabilities))
	assert.Equal(t, capability.Capability{
		Type: capability.TCPServer,
		Data: &capability.Server{Port: uint16(e.Port)},
	}, addr.Capabilities[0])

	testserdes.EncodeDecodeBinary(t, addr, new(AddressAndTime))
}

func fillAddressList(al *AddressList) {
	for i := 0; i < len(al.Addrs); i++ {
		e, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:20%d", i))
		al.Addrs[i] = NewAddressAndTime(e, time.Now(), capability.Capabilities{
			{
				Type: capability.TCPServer,
				Data: &capability.Server{Port: 123},
			},
		})
	}
}

func TestEncodeDecodeAddressList(t *testing.T) {
	var lenList uint8 = 4
	addrList := NewAddressList(int(lenList))
	fillAddressList(addrList)
	testserdes.EncodeDecodeBinary(t, addrList, new(AddressList))
}

func TestEncodeDecodeBadAddressList(t *testing.T) {
	var newAL = new(AddressList)
	addrList := NewAddressList(MaxAddrsCount + 1)
	fillAddressList(addrList)

	bin, err := testserdes.EncodeBinary(addrList)
	require.NoError(t, err)
	err = testserdes.DecodeBinary(bin, newAL)
	require.Error(t, err)

	addrList = NewAddressList(0)
	bin, err = testserdes.EncodeBinary(addrList)
	require.NoError(t, err)
	err = testserdes.DecodeBinary(bin, newAL)
	require.Error(t, err)
}

func TestGetTCPAddress(t *testing.T) {
	t.Run("bad, no capability", func(t *testing.T) {
		p := &AddressAndTime{}
		copy(p.IP[:], net.IPv4(1, 1, 1, 1))
		p.Capabilities = append(p.Capabilities, capability.Capability{
			Type: capability.TCPServer,
			Data: &capability.Server{Port: 123},
		})
		s, err := p.GetTCPAddress()
		require.NoError(t, err)
		require.Equal(t, "1.1.1.1:123", s)
	})
	t.Run("bad, no capability", func(t *testing.T) {
		p := &AddressAndTime{}
		s, err := p.GetTCPAddress()
		fmt.Println(s, err)
	})
}
