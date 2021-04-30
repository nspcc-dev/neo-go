package management

import "github.com/nspcc-dev/neo-go/pkg/interop"

// Contract represents deployed contract.
type Contract struct {
	ID            int
	UpdateCounter int
	Hash          interop.Hash160
	NEF           []byte
	Manifest      Manifest
}

// ParameterType represents smartcontract parameter type.
type ParameterType byte

// Various parameter types.
const (
	AnyType              ParameterType = 0x00
	BoolType             ParameterType = 0x10
	IntegerType          ParameterType = 0x11
	ByteArrayType        ParameterType = 0x12
	StringType           ParameterType = 0x13
	Hash160Type          ParameterType = 0x14
	Hash256Type          ParameterType = 0x15
	PublicKeyType        ParameterType = 0x16
	SignatureType        ParameterType = 0x17
	ArrayType            ParameterType = 0x20
	MapType              ParameterType = 0x22
	InteropInterfaceType ParameterType = 0x30
	VoidType             ParameterType = 0xff
)

// Manifest represents contract's manifest.
type Manifest struct {
	Name               string
	Groups             []Group
	Features           map[string]string
	SupportedStandards []string
	ABI                ABI
	Permissions        []Permission
	Trusts             []interop.Hash160
	Extra              interface{}
}

// ABI represents contract's ABI.
type ABI struct {
	Methods []Method
	Events  []Event
}

// Method represents contract method.
type Method struct {
	Name       string
	Params     []Parameter
	ReturnType ParameterType
	Offset     int
	Safe       bool
}

// Event represents contract event.
type Event struct {
	Name   string
	Params []Parameter
}

// Parameter represents method parameter.
type Parameter struct {
	Name string
	Type ParameterType
}

// Permission represents contract permission.
type Permission struct {
	Contract interop.Hash160
	Methods  []string
}

// Group represents manifest group.
type Group struct {
	PublicKey interop.PublicKey
	Signature interop.Signature
}
