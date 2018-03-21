package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/smartcontract"
)

// PublishTX represents a publish transaction.
// This is deprecated and should no longer be used.
type PublishTX struct {
	Script      []byte
	ParamList   []smartcontract.ParamType
	ReturnType  smartcontract.ParamType
	NeedStorage bool
	Name        string
	Author      string
	Email       string
	Description string
}

// DecodeBinary implements the Payload interface.
func (tx *PublishTX) DecodeBinary(r io.Reader) error {
	return nil
}

// EncodeBinary implements the Payload interface.
func (tx *PublishTX) EncodeBinary(w io.Writer) error {
	return nil
}
