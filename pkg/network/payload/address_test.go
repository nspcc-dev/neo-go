package payload

import (
	"bytes"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeAddress(t *testing.T) {
	var (
		e, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:2000")
		ts   = time.Now()
		addr = NewAddressAndTime(e, ts)
		buf  = new(bytes.Buffer)
	)

	assert.Equal(t, ts.UTC().Unix(), int64(addr.Timestamp))
	aatip := make(net.IP, 16)
	copy(aatip, addr.IP[:])
	assert.Equal(t, e.IP, aatip)
	assert.Equal(t, e.Port, int(addr.Port))
	err := addr.EncodeBinary(buf)
	assert.Nil(t, err)

	addrDecode := &AddressAndTime{}
	err = addrDecode.DecodeBinary(buf)
	assert.Nil(t, err)

	assert.Equal(t, addr, addrDecode)
}

func TestEncodeDecodeAddressList(t *testing.T) {
	var lenList uint8 = 4
	addrList := &AddressList{make([]*AddressAndTime, lenList)}
	for i := 0; i < int(lenList); i++ {
		e, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:200%d", i))
		addrList.Addrs[i] = NewAddressAndTime(e, time.Now())
	}

	buf := new(bytes.Buffer)
	err := addrList.EncodeBinary(buf)
	assert.Nil(t, err)

	addrListDecode := &AddressList{}
	err = addrListDecode.DecodeBinary(buf)
	assert.Nil(t, err)

	assert.Equal(t, addrList, addrListDecode)
}
