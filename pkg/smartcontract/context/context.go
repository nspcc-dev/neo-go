package context

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ParameterContext represents smartcontract parameter's context.
type ParameterContext struct {
	// Type is a type of a verifiable item.
	Type string
	// Verifiable is an object which can be (de-)serialized.
	Verifiable io.Serializable
	// Items is a map from script hashes to context items.
	Items map[util.Uint160]*Item
}

type paramContext struct {
	Type  string                     `json:"type"`
	Hex   string                     `json:"hex"`
	Items map[string]json.RawMessage `json:"items"`
}

// MarshalJSON implements json.Marshaler interface.
func (c ParameterContext) MarshalJSON() ([]byte, error) {
	bw := io.NewBufBinWriter()
	c.Verifiable.EncodeBinary(bw.BinWriter)
	if bw.Err != nil {
		return nil, bw.Err
	}
	items := make(map[string]json.RawMessage, len(c.Items))
	for u := range c.Items {
		data, err := json.Marshal(c.Items[u])
		if err != nil {
			return nil, err
		}
		items["0x"+u.StringBE()] = data
	}
	pc := &paramContext{
		Type:  c.Type,
		Hex:   hex.EncodeToString(bw.Bytes()),
		Items: items,
	}
	return json.Marshal(pc)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (c *ParameterContext) UnmarshalJSON(data []byte) error {
	pc := new(paramContext)
	if err := json.Unmarshal(data, pc); err != nil {
		return err
	}
	data, err := hex.DecodeString(pc.Hex)
	if err != nil {
		return err
	}

	var verif io.Serializable
	switch pc.Type {
	case "Neo.Core.ContractTransaction":
		verif = new(transaction.Transaction)
	default:
		return fmt.Errorf("unsupported type: %s", c.Type)
	}
	br := io.NewBinReaderFromBuf(data)
	verif.DecodeBinary(br)
	if br.Err != nil {
		return br.Err
	}
	items := make(map[util.Uint160]*Item, len(pc.Items))
	for h := range pc.Items {
		u, err := util.Uint160DecodeStringBE(strings.TrimPrefix(h, "0x"))
		if err != nil {
			return err
		}
		item := new(Item)
		if err := json.Unmarshal(pc.Items[h], item); err != nil {
			return err
		}
		items[u] = item
	}
	c.Type = pc.Type
	c.Verifiable = verif
	c.Items = items
	return nil
}
