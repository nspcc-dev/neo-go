package payload

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeAddress(t *testing.T) {
	var (
		e    = util.NewEndpoint("127.0.0.1:2000")
		addr = NewAddressAndTime(e, time.Now())
		buf  = new(bytes.Buffer)
	)

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
		e := util.NewEndpoint(fmt.Sprintf("127.0.0.1:200%d", i))
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
