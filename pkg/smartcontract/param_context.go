package smartcontract

import "github.com/CityOfZion/neo-go/pkg/util"

// ParamType represent the Type of the contract parameter
type ParamType int

// A list of supported smart contract parameter types.
const (
	SignatureType ParamType = iota
	BoolType
	IntegerType
	Hash160Type
	Hash256Type
	ByteArrayType
	PublicKeyType
	StringType
	ArrayType
)

// Parameter represents a smart contract parameter.
type Parameter struct {
	// Type of the parameter
	Type ParamType `json:"type"`
	// The actual value of the parameter.
	Value interface{} `json:"value"`
}

// NewParameter returns a Parameter with proper initialized Value
// of the given ParamType.
func NewParameter(t ParamType) Parameter {
	return Parameter{
		Type:  t,
		Value: nil,
	}
}

// ContextItem represents a transaction context item.
type ContextItem struct {
	Script     util.Uint160
	Parameters []Parameter
	Signatures []Signature
}

// Signature represents a transaction signature.
type Signature struct {
	Data      []byte
	PublicKey []byte
}

// ParameterContext holds the parameter context.
type ParameterContext struct{}
