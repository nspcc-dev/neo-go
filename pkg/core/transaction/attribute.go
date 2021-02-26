package transaction

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Attribute represents a Transaction attribute.
type Attribute struct {
	Type  AttrType
	Value interface {
		io.Serializable
		// toJSONMap is used for embedded json struct marshalling.
		// Anonymous interface fields are not considered anonymous by
		// json lib and marshaling Value together with type makes code
		// harder to follow.
		toJSONMap(map[string]interface{})
	}
}

// attrJSON is used for JSON I/O of Attribute.
type attrJSON struct {
	Type string `json:"type"`
}

// DecodeBinary implements Serializable interface.
func (attr *Attribute) DecodeBinary(br *io.BinReader) {
	attr.Type = AttrType(br.ReadB())

	switch t := attr.Type; t {
	case HighPriority:
		return
	case OracleResponseT:
		attr.Value = new(OracleResponse)
	case NotValidBeforeT:
		attr.Value = new(NotValidBefore)
	case ConflictsT:
		attr.Value = new(Conflicts)
	case NotaryAssistedT:
		attr.Value = new(NotaryAssisted)
	default:
		if t >= ReservedLowerBound && t <= ReservedUpperBound {
			attr.Value = new(Reserved)
			break
		}
		if br.Err == nil {
			br.Err = fmt.Errorf("failed decoding TX attribute usage: 0x%2x", int(attr.Type))
		}
		return
	}
	attr.Value.DecodeBinary(br)
}

// EncodeBinary implements Serializable interface.
func (attr *Attribute) EncodeBinary(bw *io.BinWriter) {
	bw.WriteB(byte(attr.Type))
	switch t := attr.Type; t {
	case HighPriority:
	case OracleResponseT, NotValidBeforeT, ConflictsT, NotaryAssistedT:
		attr.Value.EncodeBinary(bw)
	default:
		if t >= ReservedLowerBound && t <= ReservedUpperBound {
			attr.Value.EncodeBinary(bw)
			break
		}
		bw.Err = fmt.Errorf("failed encoding TX attribute usage: 0x%2x", attr.Type)
	}
}

// MarshalJSON implements the json Marshaller interface.
func (attr *Attribute) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{"type": attr.Type.String()}
	if attr.Value != nil {
		attr.Value.toJSONMap(m)
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (attr *Attribute) UnmarshalJSON(data []byte) error {
	aj := new(attrJSON)
	err := json.Unmarshal(data, aj)
	if err != nil {
		return err
	}
	switch aj.Type {
	case HighPriority.String():
		attr.Type = HighPriority
		return nil
	case OracleResponseT.String():
		attr.Type = OracleResponseT
		// Note: because `type` field will not be present in any attribute
		// value, we can unmarshal the same data. The overhead is minimal.
		attr.Value = new(OracleResponse)
	case NotValidBeforeT.String():
		attr.Type = NotValidBeforeT
		attr.Value = new(NotValidBefore)
	case ConflictsT.String():
		attr.Type = ConflictsT
		attr.Value = new(Conflicts)
	case NotaryAssistedT.String():
		attr.Type = NotaryAssistedT
		attr.Value = new(NotaryAssisted)
	default:
		return errors.New("wrong Type")
	}
	return json.Unmarshal(data, attr.Value)
}
