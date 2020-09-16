package transaction

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Attribute represents a Transaction attribute.
type Attribute struct {
	Type AttrType
}

// attrJSON is used for JSON I/O of Attribute.
type attrJSON struct {
	Type string `json:"type"`
}

// DecodeBinary implements Serializable interface.
func (attr *Attribute) DecodeBinary(br *io.BinReader) {
	attr.Type = AttrType(br.ReadB())

	switch attr.Type {
	case HighPriority:
	default:
		br.Err = fmt.Errorf("failed decoding TX attribute usage: 0x%2x", int(attr.Type))
		return
	}
}

// EncodeBinary implements Serializable interface.
func (attr *Attribute) EncodeBinary(bw *io.BinWriter) {
	bw.WriteB(byte(attr.Type))
	switch attr.Type {
	case HighPriority:
	default:
		bw.Err = fmt.Errorf("failed encoding TX attribute usage: 0x%2x", attr.Type)
	}
}

// MarshalJSON implements the json Marshaller interface.
func (attr *Attribute) MarshalJSON() ([]byte, error) {
	return json.Marshal(attrJSON{
		Type: attr.Type.String(),
	})
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (attr *Attribute) UnmarshalJSON(data []byte) error {
	aj := new(attrJSON)
	err := json.Unmarshal(data, aj)
	if err != nil {
		return err
	}
	switch aj.Type {
	case "HighPriority":
		attr.Type = HighPriority
	default:
		return errors.New("wrong Type")

	}
	return nil
}
