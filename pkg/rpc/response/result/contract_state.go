package result

import (
	"encoding/hex"
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

// contractState is an auxilliary struct for proper Script marshaling.
type contractState struct {
	Version     byte                      `json:"version"`
	ScriptHash  util.Uint160              `json:"hash"`
	Script      string                    `json:"script"`
	ParamList   []smartcontract.ParamType `json:"parameters"`
	ReturnType  smartcontract.ParamType   `json:"returntype"`
	Name        string                    `json:"name"`
	CodeVersion string                    `json:"code_version"`
	Author      string                    `json:"author"`
	Email       string                    `json:"email"`
	Description string                    `json:"description"`
	Properties  Properties                `json:"properties"`
}

// NewContractState creates a new Contract wrapper.
func NewContractState(c *state.Contract) ContractState {
	properties := Properties{
		HasStorage:       c.HasStorage(),
		HasDynamicInvoke: c.HasDynamicInvoke(),
		IsPayable:        c.IsPayable(),
	}

	return ContractState{
		Version:     0,
		ScriptHash:  c.ScriptHash(),
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

// MarshalJSON implements json.Marshaler interface.
func (c ContractState) MarshalJSON() ([]byte, error) {
	s := &contractState{
		Version:     c.Version,
		ScriptHash:  c.ScriptHash,
		Script:      hex.EncodeToString(c.Script),
		ParamList:   c.ParamList,
		ReturnType:  c.ReturnType,
		Name:        c.Name,
		CodeVersion: c.CodeVersion,
		Author:      c.Author,
		Email:       c.Email,
		Description: c.Description,
		Properties: Properties{
			HasStorage:       c.Properties.HasStorage,
			HasDynamicInvoke: c.Properties.HasDynamicInvoke,
			IsPayable:        c.Properties.IsPayable,
		},
	}
	return json.Marshal(s)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (c *ContractState) UnmarshalJSON(data []byte) error {
	s := new(contractState)
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}

	script, err := hex.DecodeString(s.Script)
	if err != nil {
		return err
	}

	c.Version = s.Version
	c.ScriptHash = s.ScriptHash
	c.Script = script
	c.ParamList = s.ParamList
	c.ReturnType = s.ReturnType
	c.Name = s.Name
	c.CodeVersion = s.CodeVersion
	c.Author = s.Author
	c.Email = s.Email
	c.Description = s.Description
	c.Properties = Properties{
		HasStorage:       s.Properties.HasStorage,
		HasDynamicInvoke: s.Properties.HasDynamicInvoke,
		IsPayable:        s.Properties.IsPayable,
	}

	return nil
}
