package context

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// ParameterContext represents smartcontract parameter's context.
type ParameterContext struct {
	// Type is a type of a verifiable item.
	Type string
	// Network is a network this context belongs to.
	Network netmode.Magic
	// Verifiable is an object which can be (de-)serialized.
	Verifiable crypto.VerifiableDecodable
	// Items is a map from script hashes to context items.
	Items map[util.Uint160]*Item
}

type paramContext struct {
	Type  string                     `json:"type"`
	Net   uint32                     `json:"network"`
	Data  []byte                     `json:"data"`
	Items map[string]json.RawMessage `json:"items"`
}

type sigWithIndex struct {
	index int
	sig   []byte
}

// NewParameterContext returns ParameterContext with the specified type and item to sign.
func NewParameterContext(typ string, network netmode.Magic, verif crypto.VerifiableDecodable) *ParameterContext {
	return &ParameterContext{
		Type:       typ,
		Network:    network,
		Verifiable: verif,
		Items:      make(map[util.Uint160]*Item),
	}
}

// GetWitness returns invocation and verification scripts for the specified contract.
func (c *ParameterContext) GetWitness(h util.Uint160) (*transaction.Witness, error) {
	item, ok := c.Items[h]
	if !ok {
		return nil, errors.New("witness not found")
	}
	bw := io.NewBufBinWriter()
	for i := range item.Parameters {
		if item.Parameters[i].Type != smartcontract.SignatureType {
			return nil, errors.New("only signature parameters are supported")
		} else if item.Parameters[i].Value == nil {
			return nil, errors.New("nil parameter")
		}
		emit.Bytes(bw.BinWriter, item.Parameters[i].Value.([]byte))
	}
	return &transaction.Witness{
		InvocationScript:   bw.Bytes(),
		VerificationScript: item.Script,
	}, nil
}

// AddSignature adds a signature for the specified contract and public key.
func (c *ParameterContext) AddSignature(h util.Uint160, ctr *wallet.Contract, pub *keys.PublicKey, sig []byte) error {
	item := c.getItemForContract(h, ctr)
	if _, pubs, ok := vm.ParseMultiSigContract(ctr.Script); ok {
		if item.GetSignature(pub) != nil {
			return errors.New("signature is already added")
		}
		pubBytes := pub.Bytes()
		var contained bool
		for i := range pubs {
			if bytes.Equal(pubBytes, pubs[i]) {
				contained = true
				break
			}
		}
		if !contained {
			return errors.New("public key is not present in script")
		}
		item.AddSignature(pub, sig)
		if len(item.Signatures) == len(ctr.Parameters) {
			indexMap := map[string]int{}
			for i := range pubs {
				indexMap[hex.EncodeToString(pubs[i])] = i
			}
			sigs := make([]sigWithIndex, 0, len(item.Signatures))
			for pub, sig := range item.Signatures {
				sigs = append(sigs, sigWithIndex{index: indexMap[pub], sig: sig})
			}
			sort.Slice(sigs, func(i, j int) bool {
				return sigs[i].index < sigs[j].index
			})
			for i := range sigs {
				item.Parameters[i] = smartcontract.Parameter{
					Type:  smartcontract.SignatureType,
					Value: sigs[i].sig,
				}
			}
		}
		return nil
	}

	index := -1
	for i := range ctr.Parameters {
		if ctr.Parameters[i].Type == smartcontract.SignatureType {
			if index >= 0 {
				return errors.New("multiple signature parameters in non-multisig contract")
			}
			index = i
		}
	}
	if index != -1 {
		item.Parameters[index].Value = sig
	} else if !ctr.Deployed {
		return errors.New("missing signature parameter")
	}
	return nil
}

func (c *ParameterContext) getItemForContract(h util.Uint160, ctr *wallet.Contract) *Item {
	item, ok := c.Items[ctr.ScriptHash()]
	if ok {
		return item
	}
	params := make([]smartcontract.Parameter, len(ctr.Parameters))
	for i := range params {
		params[i].Type = ctr.Parameters[i].Type
	}
	script := ctr.Script
	if ctr.Deployed {
		script = nil
	}
	item = &Item{
		Script:     script,
		Parameters: params,
		Signatures: make(map[string][]byte),
	}
	c.Items[h] = item
	return item
}

// MarshalJSON implements json.Marshaler interface.
func (c ParameterContext) MarshalJSON() ([]byte, error) {
	verif, err := c.Verifiable.EncodeHashableFields()
	if err != nil {
		return nil, fmt.Errorf("failed to encode hashable fields")
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
		Net:   uint32(c.Network),
		Data:  verif,
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

	var verif crypto.VerifiableDecodable
	switch pc.Type {
	case "Neo.Core.ContractTransaction":
		tx := new(transaction.Transaction)
		verif = tx
	default:
		return fmt.Errorf("unsupported type: %s", c.Type)
	}
	err := verif.DecodeHashableFields(pc.Data)
	if err != nil {
		return err
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
	c.Network = netmode.Magic(pc.Net)
	c.Verifiable = verif
	c.Items = items
	return nil
}
