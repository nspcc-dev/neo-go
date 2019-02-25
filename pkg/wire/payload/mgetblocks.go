package payload

import (
	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type GetBlocksMessage struct {
	*GetHeadersMessage
}

func NewGetBlocksMessage(start []util.Uint256, stop util.Uint256) (*GetBlocksMessage, error) {
	GetHeaders, err := newAbstractGetHeaders(start, stop, command.GetBlocks)

	if err != nil {
		return nil, err
	}
	return &GetBlocksMessage{
		GetHeaders,
	}, nil

}
