package payload

import (
	"bytes"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/stretchr/testify/assert"
)

func TestNewGetAddr(t *testing.T) {

	getAddrMessage, err := NewGetAddrMessage()
	assert.Equal(t, nil, err)

	assert.Equal(t, command.GetAddr, getAddrMessage.Command())

	buf := new(bytes.Buffer)

	assert.Equal(t, int(3806393949), int(checksum.FromBuf(buf)))
	assert.Equal(t, int(0), len(buf.Bytes()))
}
