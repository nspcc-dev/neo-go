package state

import (
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Contract holds information about a smart contract in the NEO blockchain.
type Contract struct {
	Script      []byte
	ParamList   []smartcontract.ParamType
	ReturnType  smartcontract.ParamType
	Properties  smartcontract.PropertyState
	Name        string
	CodeVersion string
	Author      string
	Email       string
	Description string

	scriptHash util.Uint160
}

// DecodeBinary implements Serializable interface.
func (cs *Contract) DecodeBinary(br *io.BinReader) {
	cs.Script = br.ReadVarBytes()
	br.ReadArray(&cs.ParamList)
	cs.ReturnType = smartcontract.ParamType(br.ReadB())
	cs.Properties = smartcontract.PropertyState(br.ReadB())
	cs.Name = br.ReadString()
	cs.CodeVersion = br.ReadString()
	cs.Author = br.ReadString()
	cs.Email = br.ReadString()
	cs.Description = br.ReadString()
	cs.createHash()
}

// EncodeBinary implements Serializable interface.
func (cs *Contract) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarBytes(cs.Script)
	bw.WriteArray(cs.ParamList)
	bw.WriteB(byte(cs.ReturnType))
	bw.WriteB(byte(cs.Properties))
	bw.WriteString(cs.Name)
	bw.WriteString(cs.CodeVersion)
	bw.WriteString(cs.Author)
	bw.WriteString(cs.Email)
	bw.WriteString(cs.Description)
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
	return (cs.Properties & smartcontract.HasStorage) != 0
}

// HasDynamicInvoke checks whether the contract has dynamic invoke property set.
func (cs *Contract) HasDynamicInvoke() bool {
	return (cs.Properties & smartcontract.HasDynamicInvoke) != 0
}

// IsPayable checks whether the contract has payable property set.
func (cs *Contract) IsPayable() bool {
	return (cs.Properties & smartcontract.IsPayable) != 0
}
