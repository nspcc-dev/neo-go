package payload

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
)

func TestEncodeDecodeAddr(t *testing.T) {
	e, err := util.EndpointFromString("127.0.0.1:2000")
	if err != nil {
		t.Fatal(err)
	}

	addr := NewAddrWithTime(e)
	buf := new(bytes.Buffer)
	if err := addr.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	addrDecode := &AddrWithTime{}
	if err := addrDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(addr, addrDecode) {
		t.Fatalf("expected both addr payloads to be equal: %v and %v", addr, addrDecode)
	}
}

func TestEncodeDecodeAddressList(t *testing.T) {
	var lenList uint8 = 4
	addrs := make([]*AddrWithTime, lenList)
	for i := 0; i < int(lenList); i++ {
		e, _ := util.EndpointFromString(fmt.Sprintf("127.0.0.1:200%d", i))
		addrs[i] = NewAddrWithTime(e)
	}

	buf := new(bytes.Buffer)
	addrList := &AddressList{addrs}
	if err := addrList.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	addrListDecode := &AddressList{}
	if err := addrListDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(addrList, addrListDecode) {
		t.Fatalf("expected both address list payloads to be equal: %v and %v", addrList, addrListDecode)
	}
}
