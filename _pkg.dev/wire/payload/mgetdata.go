package payload

import (
	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

// GetDataMessage represents a GetData message on the neo-network
type GetDataMessage struct {
	*InvMessage
}

//NewGetDataMessage returns a GetDataMessage object
func NewGetDataMessage(typ InvType) (*GetDataMessage, error) {
	getData, err := newAbstractInv(typ, command.GetData)
	return &GetDataMessage{
		getData,
	}, err
}
