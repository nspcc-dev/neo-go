package manifest

import (
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Group represents a group of smartcontracts identified by a public key.
// Every SC in a group must provide signature of it's hash to prove
// it belongs to a group.
type Group struct {
	PublicKey *keys.PublicKey `json:"pubkey"`
	Signature []byte          `json:"signature"`
}

// Groups is just an array of Group.
type Groups []Group

type groupAux struct {
	PublicKey string `json:"pubkey"`
	Signature []byte `json:"signature"`
}

// IsValid checks whether group's signature corresponds to the given hash.
func (g *Group) IsValid(h util.Uint160) error {
	if !g.PublicKey.Verify(g.Signature, hash.Sha256(h.BytesBE()).BytesBE()) {
		return errors.New("incorrect group signature")
	}
	return nil
}

// AreValid checks for groups correctness and uniqueness.
func (g Groups) AreValid(h util.Uint160) error {
	for i := range g {
		err := g[i].IsValid(h)
		if err != nil {
			return err
		}
	}
	if len(g) < 2 {
		return nil
	}
	pkeys := make(keys.PublicKeys, len(g))
	for i := range g {
		pkeys[i] = g[i].PublicKey
	}
	sort.Sort(pkeys)
	for i := range pkeys {
		if i == 0 {
			continue
		}
		if pkeys[i].Cmp(pkeys[i-1]) == 0 {
			return errors.New("duplicate group keys")
		}
	}
	return nil
}

// MarshalJSON implements json.Marshaler interface.
func (g *Group) MarshalJSON() ([]byte, error) {
	aux := &groupAux{
		PublicKey: hex.EncodeToString(g.PublicKey.Bytes()),
		Signature: g.Signature,
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (g *Group) UnmarshalJSON(data []byte) error {
	aux := new(groupAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	b, err := hex.DecodeString(aux.PublicKey)
	if err != nil {
		return err
	}
	pub := new(keys.PublicKey)
	if err := pub.DecodeBytes(b); err != nil {
		return err
	}
	g.PublicKey = pub
	if len(aux.Signature) != keys.SignatureLen {
		return errors.New("wrong signature length")
	}
	g.Signature = aux.Signature
	return nil
}

// ToStackItem converts Group to stackitem.Item.
func (g *Group) ToStackItem() stackitem.Item {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray(g.PublicKey.Bytes()),
		stackitem.NewByteArray(g.Signature),
	})
}

// FromStackItem converts stackitem.Item to Group.
func (g *Group) FromStackItem(item stackitem.Item) error {
	if item.Type() != stackitem.StructT {
		return errors.New("invalid Group stackitem type")
	}
	group := item.Value().([]stackitem.Item)
	if len(group) != 2 {
		return errors.New("invalid Group stackitem length")
	}
	pKey, err := group[0].TryBytes()
	if err != nil {
		return err
	}
	g.PublicKey, err = keys.NewPublicKeyFromBytes(pKey, elliptic.P256())
	if err != nil {
		return err
	}
	sig, err := group[1].TryBytes()
	if err != nil {
		return err
	}
	if len(sig) != keys.SignatureLen {
		return errors.New("wrong signature length")
	}
	g.Signature = sig
	return nil
}
