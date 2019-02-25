package payload

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/stretchr/testify/assert"
)

func TestGetDataCommandType(t *testing.T) {
	getData, err := NewGetDataMessage(InvTypeBlock)

	assert.Equal(t, err, nil)
	assert.Equal(t, command.GetData, getData.Command())
}

func TestOtherFunctions(t *testing.T) {
	// Other capabilities are tested in the inherited struct
}
