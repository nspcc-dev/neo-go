package payload

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeAddress(t *testing.T) {
	var (
		e, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:2000")
		ts   = time.Now()
		addr = NewAddressAndTime(e, ts)
		buf  = io.NewBufBinWriter()
	)

	assert.Equal(t, ts.UTC().Unix(), int64(addr.Timestamp))
	aatip := make(net.IP, 16)
	copy(aatip, addr.IP[:])
	assert.Equal(t, e.IP, aatip)
	assert.Equal(t, e.Port, int(addr.Port))
	err := addr.EncodeBinary(buf.BinWriter)
	assert.Nil(t, err)

	b := buf.Bytes()
	r := io.NewBinReaderFromBuf(b)
	addrDecode := &AddressAndTime{}
	err = addrDecode.DecodeBinary(r)
	assert.Nil(t, err)

	assert.Equal(t, addr, addrDecode)
}

func TestEncodeDecodeAddressList(t *testing.T) {
	var lenList uint8 = 4
	addrList := NewAddressList(int(lenList))
	for i := 0; i < int(lenList); i++ {
		e, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:200%d", i))
		addrList.Addrs[i] = NewAddressAndTime(e, time.Now())
	}

	buf := io.NewBufBinWriter()
	err := addrList.EncodeBinary(buf.BinWriter)
	assert.Nil(t, err)

	b := buf.Bytes()
	r := io.NewBinReaderFromBuf(b)
	addrListDecode := &AddressList{}
	err = addrListDecode.DecodeBinary(r)
	assert.Nil(t, err)

	assert.Equal(t, addrList, addrListDecode)
}
