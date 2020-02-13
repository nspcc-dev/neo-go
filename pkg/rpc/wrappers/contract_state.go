package wrappers

import (
	"github.com/CityOfZion/neo-go/pkg/core/state"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// ContractState wrapper used for the representation of
// state.Contract on the RPC Server.
type ContractState struct {
	Version     byte                      `json:"version"`
	ScriptHash  util.Uint160              `json:"hash"`
	Script      []byte                    `json:"script"`
	ParamList   []smartcontract.ParamType `json:"parameters"`
	ReturnType  smartcontract.ParamType   `json:"returntype"`
	Name        string                    `json:"name"`
	CodeVersion string                    `json:"code_version"`
	Author      string                    `json:"author"`
	Email       string                    `json:"email"`
	Description string                    `json:"description"`
	Properties  Properties                `json:"properties"`
}

// Properties response wrapper.
type Properties struct {
	HasStorage       bool `json:"storage"`
	HasDynamicInvoke bool `json:"dynamic_invoke"`
	IsPayable        bool `json:"is_payable"`
}

// NewContractState creates a new Contract wrapper.
func NewContractState(c *state.Contract) ContractState {
	scriptHash, err := util.Uint160DecodeBytesBE(c.ScriptHash().BytesLE())
	if err != nil {
		scriptHash = c.ScriptHash()
	}

	properties := Properties{
		HasStorage:       c.HasStorage(),
		HasDynamicInvoke: c.HasDynamicInvoke(),
		IsPayable:        c.IsPayable(),
	}

	return ContractState{
		Version:     0,
		ScriptHash:  scriptHash,
		Script:      c.Script,
		ParamList:   c.ParamList,
		ReturnType:  c.ReturnType,
		Properties:  properties,
		Name:        c.Name,
		CodeVersion: c.CodeVersion,
		Author:      c.Author,
		Email:       c.Email,
		Description: c.Description,
	}
}
