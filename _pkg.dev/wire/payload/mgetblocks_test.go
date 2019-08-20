package payload

import (
	"crypto/sha256"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/stretchr/testify/assert"
)

func TestGetBlocksCommandType(t *testing.T) {
	var (
		start = []util.Uint256{
			sha256.Sum256([]byte("a")),
			sha256.Sum256([]byte("b")),
			sha256.Sum256([]byte("c")),
			sha256.Sum256([]byte("d")),
		}
		stop = sha256.Sum256([]byte("e"))
	)

	getBlocks, err := NewGetBlocksMessage(start, stop)

	assert.Equal(t, err, nil)
	assert.Equal(t, command.GetBlocks, getBlocks.Command())
}
