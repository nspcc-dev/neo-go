package payload

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/command"

	"github.com/stretchr/testify/assert"
)

func TestNewVerack(t *testing.T) {

	verackMessage, err := NewVerackMessage()

	assert.Equal(t, nil, err)
	assert.Equal(t, command.Verack, verackMessage.Command())
	assert.Equal(t, int(3806393949), int(verackMessage.Checksum()))
}
