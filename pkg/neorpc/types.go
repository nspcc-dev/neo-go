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
		Params []any `json:"params"`
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
		Error *Error `json:"error,omitzero"`
	}

	// Response represents a standard raw JSON-RPC 2.0
	// response: http://www.jsonrpc.org/specification#response_object.
	Response struct {
		HeaderAndError
		Result json.RawMessage `json:"result,omitzero"`
	}

	// Notification is a type used to represent wire format of events, they're
	// special in that they look like requests but they don't have IDs and their
	// "method" is actually an event name.
	Notification struct {
		JSONRPC string  `json:"jsonrpc"`
		Event   EventID `json:"method"`
		Payload []any   `json:"params"`
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
	Scopes             json.RawMessage           `json:"scopes"`
	AllowedContracts   []util.Uint160            `json:"allowedcontracts,omitzero"`
	AllowedGroups      []*keys.PublicKey         `json:"allowedgroups,omitzero"`
	Rules              []transaction.WitnessRule `json:"rules,omitzero"`
	InvocationScript   []byte                    `json:"invocation,omitzero"`
	VerificationScript []byte                    `json:"verification,omitzero"`
}

// MarshalJSON implements the json.Marshaler interface.
func (s *SignerWithWitness) MarshalJSON() ([]byte, error) {
	sc, err := s.Scopes.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal scopes: %w", err)
	}
	signer := &signerWithWitnessAux{
		Account:            `0x` + s.Account.StringLE(),
		Scopes:             sc,
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
	if len(aux.AllowedContracts) > transaction.MaxAttributes {
		return fmt.Errorf("invalid number of AllowedContracts: got %d, allowed %d at max", len(aux.AllowedContracts), transaction.MaxAttributes)
	}
	if len(aux.AllowedGroups) > transaction.MaxAttributes {
		return fmt.Errorf("invalid number of AllowedGroups: got %d, allowed %d at max", len(aux.AllowedGroups), transaction.MaxAttributes)
	}
	if len(aux.Rules) > transaction.MaxAttributes {
		return fmt.Errorf("invalid number of Rules: got %d, allowed %d at max", len(aux.Rules), transaction.MaxAttributes)
	}
	acc, err := util.Uint160DecodeStringLE(strings.TrimPrefix(aux.Account, "0x"))
	if err != nil {
		acc, err = address.StringToUint160(aux.Account)
	}
	if err != nil {
		return fmt.Errorf("not a signer: %w", err)
	}
	var (
		jStr   string
		jByte  byte
		scopes transaction.WitnessScope
	)
	if len(aux.Scopes) != 0 {
		if err := json.Unmarshal(aux.Scopes, &jStr); err == nil {
			scopes, err = transaction.ScopesFromString(jStr)
			if err != nil {
				return fmt.Errorf("failed to retrieve scopes from string: %w", err)
			}
		} else {
			err := json.Unmarshal(aux.Scopes, &jByte)
			if err != nil {
				return fmt.Errorf("failed to unmarshal scopes from byte: %w", err)
			}
			scopes, err = transaction.ScopesFromByte(jByte)
			if err != nil {
				return fmt.Errorf("failed to retrieve scopes from byte: %w", err)
			}
		}
	}
	s.Signer = transaction.Signer{
		Account:          acc,
		Scopes:           scopes,
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

// EventID implements EventContainer interface and returns notification ID.
func (n *Notification) EventID() EventID {
	return n.Event
}

// EventPayload implements EventContainer interface and returns notification
// object.
func (n *Notification) EventPayload() any {
	return n.Payload[0]
}
