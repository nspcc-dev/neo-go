package ledger

import "github.com/nspcc-dev/neo-go/pkg/interop"

// TransactionSigner represent the signer of a NEO transaction. It's similar to
// Signer class in Neo .net framework.
type TransactionSigner struct {
	// Account represents the account (160 bit BE value in a 20 byte slice) of
	// the given signer.
	Account interop.Hash160
	// Scopes represents a set of witness flags for the given signer.
	Scopes SignerScope
	// Contracts represents the set of contract hashes (160 bit BE value in a 20
	// byte slice) allowed to be called by the signer. It is only non-empty if
	// CustomContracts scope flag is set.
	AllowedContracts []interop.Hash160
	// AllowedGroups represents the set of contract groups (ecdsa public key
	// bytes in a 33 byte slice) allowed to be called by the signer. It is only
	// non-empty if CustomGroups scope flag is set.
	AllowedGroups []interop.PublicKey
	// Rules represents a rule-based witness scope of the given signer. It is
	// only non-empty if Rules scope flag is set.
	Rules []WitnessRule
}

// SignerScope represents a signer's witness scope.
type SignerScope byte

// Various witness scopes.
const (
	// None specifies that no contract was witnessed. Only signs the transaction
	// and pays GAS fee if a sender.
	None SignerScope = 0
	// CalledByEntry means that the witness is valid only when the witness
	// checking contract is called from the entry script.
	CalledByEntry SignerScope = 0x01
	// CustomContracts define custom hash for contract-specific witness.
	CustomContracts SignerScope = 0x10
	// CustomGroups define custom public key for group members.
	CustomGroups SignerScope = 0x20
	// Rules is a set of conditions with boolean operators.
	Rules SignerScope = 0x40
	// Global allows this witness in all contexts. This cannot be combined with
	// other flags.
	Global SignerScope = 0x80
)

// WitnessRule represents a single rule for Rules witness scope.
type WitnessRule struct {
	// Action denotes whether the witness condition should be accepted or denied.
	Action WitnessAction
	// Condition holds a set of nested witness rules. Max nested depth is 2.
	Condition WitnessCondition
}

// WitnessAction represents an action to perform in WitnessRule if
// witness condition matches.
type WitnessAction byte

// Various rule-based witness actions.
const (
	// WitnessDeny rejects current witness if condition is met.
	WitnessDeny WitnessAction = 0
	// WitnessAllow approves current witness if condition is met.
	WitnessAllow WitnessAction = 1
)

// WitnessCondition represents a single witness condition for a rule-based
// witness. Its type can always be safely accessed, but trying to access its
// value causes runtime exception for those types that don't have value
// (currently, it's only CalledByEntry witness condition).
type WitnessCondition struct {
	Type WitnessConditionType
	// Depends on the witness condition Type, its value can be asserted to the
	// certain structure according to the following rule:
	// WitnessBoolean -> bool
	// WitnessNot ->  []WitnessCondition with one element
	// WitnessAnd -> []WitnessCondition
	// WitnessOr -> []WitnessCondition
	// WitnessScriptHash -> interop.Hash160
	// WitnessGroup -> interop.PublicKey
	// WitnessCalledByContract -> interop.Hash160
	// WitnessCalledByGroup -> interop.PublicKey
	// WitnessCalledByEntry -> doesn't have value, thus, an attempt to access the Value leads to runtime exception.
	Value interface{}
}

// WitnessConditionType represents the type of rule-based witness condition.
type WitnessConditionType byte

// Various witness condition types
const (
	// WitnessBoolean is a generic boolean condition.
	WitnessBoolean WitnessConditionType = 0x00
	// WitnessNot reverses another condition.
	WitnessNot WitnessConditionType = 0x01
	// WitnessAnd means that all conditions must be met.
	WitnessAnd WitnessConditionType = 0x02
	// WitnessOr means that any of conditions must be met.
	WitnessOr WitnessConditionType = 0x03
	// WitnessScriptHash matches executing contract's script hash.
	WitnessScriptHash WitnessConditionType = 0x18
	// WitnessGroup matches executing contract's group key.
	WitnessGroup WitnessConditionType = 0x19
	// WitnessCalledByEntry matches when current script is an entry script or is
	// called by an entry script.
	WitnessCalledByEntry WitnessConditionType = 0x20
	// WitnessCalledByContract matches when current script is called by the
	// specified contract.
	WitnessCalledByContract WitnessConditionType = 0x28
	// WitnessCalledByGroup matches when current script is called by contract
	// belonging to the specified group.
	WitnessCalledByGroup WitnessConditionType = 0x29
)
