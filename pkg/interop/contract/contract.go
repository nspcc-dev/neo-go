/*
Package contract provides functions to work with contracts.
*/
package contract

// Contract represents a Neo contract and is used in interop functions. It's
// an opaque data structure that you can manipulate with using functions from
// this package. It's similar in function to the Contract class in the Neo .net
// framework.
type Contract struct{}

// Create creates a new contract using a set of input parameters:
//     script      contract's bytecode (limited in length by 1M)
//     manifest    contract's manifest (limited in length by 2 KiB)
// It returns this new created Contract when successful (and fails transaction
// if not). It uses `System.Contract.Create` syscall.
func Create(script []byte, manifest []byte) Contract {
	return Contract{}
}

// Update updates script and manifest of the calling contract (that is the one that calls Update)
// to the new ones. Its parameters have exactly the same semantics as for
// Create. The old contract will be deleted by this call, if it has any storage
// associated it will be migrated to the new contract. New contract is returned.
// This function uses `System.Contract.Update` syscall.
func Update(script []byte, manifest []byte) Contract {
	return Contract{}
}

// Destroy deletes calling contract (the one that calls Destroy) from the
// blockchain, so it's only possible to do that from the contract itself and
// not by any outside code. When contract is deleted all associated storage
// items are deleted too. This function uses `System.Contract.Destroy` syscall.
func Destroy() {}

// IsStandard checks if contract with provided hash is a standard signature/multisig contract.
// This function uses `System.Contract.IsStandard` syscall.
func IsStandard(h []byte) bool {
	return false
}

// CreateStandardAccount calculates script hash of a given public key.
// This function uses `System.Contract.CreateStandardAccount` syscall.
func CreateStandardAccount(pub []byte) []byte {
	return nil
}
