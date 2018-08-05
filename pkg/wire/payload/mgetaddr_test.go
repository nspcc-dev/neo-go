package payload

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/stretchr/testify/assert"
)

func TestNewGetAddr(t *testing.T) {

	getAddrMessage, err := NewGetAddrMessage()
	assert.Equal(t, nil, err)

	assert.Equal(t, command.GetAddr, getAddrMessage.Command())
	assert.Equal(t, int(3806393949), int(getAddrMessage.Checksum()))
	assert.Equal(t, int(0), int(getAddrMessage.PayloadLength()))
}
