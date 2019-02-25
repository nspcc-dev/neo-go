package payload

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"

	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/stretchr/testify/assert"
)

func TestAddrMessageEncodeDecode(t *testing.T) {

	ip := []byte(net.ParseIP("127.0.0.1").To16())

	var ipByte [16]byte
	copy(ipByte[:], ip)

	netaddr, err := NewNetAddr(uint32(time.Now().Unix()), ipByte, 8080, protocol.NodePeerService)
	addrmsg, err := NewAddrMessage()
	addrmsg.AddNetAddr(netaddr)

	buf := new(bytes.Buffer)
	err = addrmsg.EncodePayload(buf)
	expected := checksum.FromBuf(buf)

	addrmsgDec, err := NewAddrMessage()
	r := bytes.NewReader(buf.Bytes())
	err = addrmsgDec.DecodePayload(r)

	buf = new(bytes.Buffer)
	err = addrmsgDec.EncodePayload(buf)
	have := checksum.FromBuf(buf)

	assert.Equal(t, nil, err)
	assert.Equal(t, expected, have)
}
