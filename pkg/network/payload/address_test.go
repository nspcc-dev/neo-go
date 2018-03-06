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

	if err := addr.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	addrDecode := &AddressAndTime{}
	if err := addrDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

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
	if err := addrList.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	addrListDecode := &AddressList{}
	if err := addrListDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, addrList, addrListDecode)
}
