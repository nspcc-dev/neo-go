package payload

import (
	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

type GetDataMessage struct {
	*InvMessage
}

func NewGetDataMessage(typ InvType) (*GetDataMessage, error) {
	getData, err := newAbstractInv(typ, command.GetData)
	return &GetDataMessage{
		getData,
	}, err
}
