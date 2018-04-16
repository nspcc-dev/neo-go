package core

import (
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// ContractState holds information about a smart contract in the NEO blockchain.
type ContractState struct {
	Script           []byte
	ParamList        []smartcontract.ParamType
	ReturnType       smartcontract.ParamType
	Properties       []int
	Name             string
	CodeVersion      string
	Author           string
	Email            string
	Description      string
	HasStorage       bool
	HasDynamicInvoke bool

	scriptHash util.Uint160
}
