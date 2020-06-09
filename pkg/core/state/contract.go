package state

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Contract holds information about a smart contract in the NEO blockchain.
type Contract struct {
	ID       int32
	Script   []byte
	Manifest manifest.Manifest

	scriptHash util.Uint160
}

// DecodeBinary implements Serializable interface.
func (cs *Contract) DecodeBinary(br *io.BinReader) {
	cs.ID = int32(br.ReadU32LE())
	cs.Script = br.ReadVarBytes()
	cs.Manifest.DecodeBinary(br)
	cs.createHash()
}

// EncodeBinary implements Serializable interface.
func (cs *Contract) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(uint32(cs.ID))
	bw.WriteVarBytes(cs.Script)
	cs.Manifest.EncodeBinary(bw)
}

// ScriptHash returns a contract script hash.
func (cs *Contract) ScriptHash() util.Uint160 {
	if cs.scriptHash.Equals(util.Uint160{}) {
		cs.createHash()
	}
	return cs.scriptHash
}

// createHash creates contract script hash.
func (cs *Contract) createHash() {
	cs.scriptHash = hash.Hash160(cs.Script)
}

// HasStorage checks whether the contract has storage property set.
func (cs *Contract) HasStorage() bool {
	return (cs.Manifest.Features & smartcontract.HasStorage) != 0
}

// IsPayable checks whether the contract has payable property set.
func (cs *Contract) IsPayable() bool {
	return (cs.Manifest.Features & smartcontract.IsPayable) != 0
}

type contractJSON struct {
	ID         int32              `json:"id"`
	Script     []byte             `json:"script"`
	Manifest   *manifest.Manifest `json:"manifest"`
	ScriptHash util.Uint160       `json:"hash"`
}

// MarshalJSON implements json.Marshaler.
func (cs *Contract) MarshalJSON() ([]byte, error) {
	return json.Marshal(&contractJSON{
		ID:         cs.ID,
		Script:     cs.Script,
		Manifest:   &cs.Manifest,
		ScriptHash: cs.ScriptHash(),
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (cs *Contract) UnmarshalJSON(data []byte) error {
	var cj contractJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		return err
	} else if cj.Manifest == nil {
		return errors.New("empty manifest")
	}
	cs.ID = cj.ID
	cs.Script = cj.Script
	cs.Manifest = *cj.Manifest
	cs.createHash()
	return nil
}
