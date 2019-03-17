package payload

import (
	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// GetBlocksMessage represnts a GetBlocks message on the neo-network
type GetBlocksMessage struct {
	*GetHeadersMessage
}

// NewGetBlocksMessage returns a GetBlocksMessage object
func NewGetBlocksMessage(start []util.Uint256, stop util.Uint256) (*GetBlocksMessage, error) {
	GetHeaders, err := newAbstractGetHeaders(start, stop, command.GetBlocks)

	if err != nil {
		return nil, err
	}
	return &GetBlocksMessage{
		GetHeaders,
	}, nil

}
