/*
Package neorpc contains a set of types used for JSON-RPC communication with Neo servers.
It defines basic request/response types as well as a set of errors and additional
parameters used for specific requests/responses.
*/
package neorpc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// JSONRPCVersion is the only JSON-RPC protocol version supported.
	JSONRPCVersion = "2.0"
)

type (
	// Request represents JSON-RPC request. It's generic enough to be used in many
	// generic JSON-RPC communication scenarios, yet at the same time it's
	// tailored for NeoGo RPC Client needs.
	Request struct {
		// JSONRPC is the protocol version, only valid when it contains JSONRPCVersion.
		JSONRPC string `json:"jsonrpc"`
		// Method is the method being called.
		Method string `json:"method"`
		// Params is a set of method-specific parameters passed to the call. They
		// can be anything as long as they can be marshaled to JSON correctly and
		// used by the method implementation on the server side. While JSON-RPC
		// technically allows it to be an object, all Neo calls expect params
		// to be an array.
		Params []interface{} `json:"params"`
		// ID is an identifier associated with this request. JSON-RPC itself allows
		// any strings to be used for it as well, but NeoGo RPC client uses numeric
		// identifiers.
		ID uint64 `json:"id"`
	}

	// Header is a generic JSON-RPC 2.0 response header (ID and JSON-RPC version).
	Header struct {
		ID      json.RawMessage `json:"id"`
		JSONRPC string          `json:"jsonrpc"`
	}

	// HeaderAndError adds an Error (that can be empty) to the Header, it's used
	// to construct type-specific responses.
	HeaderAndError struct {
		Header
		Error *Error `json:"error,omitempty"`
	}

	// Raw represents a standard raw JSON-RPC 2.0
	// response: http://www.jsonrpc.org/specification#response_object.
	Response struct {
		HeaderAndError
		Result json.RawMessage `json:"result,omitempty"`
	}

	// Notification is a type used to represent wire format of events, they're
	// special in that they look like requests but they don't have IDs and their
	// "method" is actually an event name.
	Notification struct {
		JSONRPC string        `json:"jsonrpc"`
		Event   EventID       `json:"method"`
		Payload []interface{} `json:"params"`
	}

	// BlockFilter is a wrapper structure for the block event filter. The only
	// allowed filter is primary index.
	BlockFilter struct {
		Primary int `json:"primary"`
	}
	// TxFilter is a wrapper structure for the transaction event filter. It
	// allows to filter transactions by senders and signers.
	TxFilter struct {
		Sender *util.Uint160 `json:"sender,omitempty"`
		Signer *util.Uint160 `json:"signer,omitempty"`
	}
	// NotificationFilter is a wrapper structure representing a filter used for
	// notifications generated during transaction execution. Notifications can
	// be filtered by contract hash and by name.
	NotificationFilter struct {
		Contract *util.Uint160 `json:"contract,omitempty"`
		Name     *string       `json:"name,omitempty"`
	}
	// ExecutionFilter is a wrapper structure used for transaction execution
	// events. It allows to choose failing or successful transactions based
	// on their VM state.
	ExecutionFilter struct {
		State string `json:"state"`
	}
	// SignerWithWitness represents transaction's signer with the corresponding witness.
	SignerWithWitness struct {
		transaction.Signer
		transaction.Witness
	}
)

// signerWithWitnessAux is an auxiliary struct for JSON marshalling. We need it because of
// DisallowUnknownFields JSON marshaller setting.
type signerWithWitnessAux struct {
	Account            string                    `json:"account"`
	Scopes             transaction.WitnessScope  `json:"scopes"`
	AllowedContracts   []util.Uint160            `json:"allowedcontracts,omitempty"`
	AllowedGroups      []*keys.PublicKey         `json:"allowedgroups,omitempty"`
	Rules              []transaction.WitnessRule `json:"rules,omitempty"`
	InvocationScript   []byte                    `json:"invocation,omitempty"`
	VerificationScript []byte                    `json:"verification,omitempty"`
}

// MarshalJSON implements the json.Marshaler interface.
func (s *SignerWithWitness) MarshalJSON() ([]byte, error) {
	signer := &signerWithWitnessAux{
		Account:            s.Account.StringLE(),
		Scopes:             s.Scopes,
		AllowedContracts:   s.AllowedContracts,
		AllowedGroups:      s.AllowedGroups,
		Rules:              s.Rules,
		InvocationScript:   s.InvocationScript,
		VerificationScript: s.VerificationScript,
	}
	return json.Marshal(signer)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *SignerWithWitness) UnmarshalJSON(data []byte) error {
	aux := new(signerWithWitnessAux)
	err := json.Unmarshal(data, aux)
	if err != nil {
		return fmt.Errorf("not a signer: %w", err)
	}
	acc, err := util.Uint160DecodeStringLE(strings.TrimPrefix(aux.Account, "0x"))
	if err != nil {
		acc, err = address.StringToUint160(aux.Account)
	}
	if err != nil {
		return fmt.Errorf("not a signer: %w", err)
	}
	s.Signer = transaction.Signer{
		Account:          acc,
		Scopes:           aux.Scopes,
		AllowedContracts: aux.AllowedContracts,
		AllowedGroups:    aux.AllowedGroups,
		Rules:            aux.Rules,
	}
	s.Witness = transaction.Witness{
		InvocationScript:   aux.InvocationScript,
		VerificationScript: aux.VerificationScript,
	}
	return nil
}
