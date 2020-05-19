/*
Package contract provides functions to work with contracts.
*/
package contract

import "github.com/nspcc-dev/neo-go/pkg/interop/storage"

// Contract represents a Neo contract and is used in interop functions. It's
// an opaque data structure that you can manipulate with using functions from
// this package. It's similar in function to the Contract class in the Neo .net
// framework.
type Contract struct{}

// GetScript returns the script of the given contract. It uses
// `Neo.Contract.GetScript` syscall.
func GetScript(c Contract) []byte {
	return nil
}

// IsPayable returns whether the given contract is payable (able to accept
// asset transfers to its address). It uses `Neo.Contract.IsPayable` syscall.
func IsPayable(c Contract) bool {
	return false
}

// GetStorageContext returns storage context for the given contract. It only
// works for contracts created in this transaction (so you can't take a storage
// context for arbitrary contract). Refer to the `storage` package on how to
// use this context. This function uses `Neo.Contract.GetStorageContext` syscall.
func GetStorageContext(c Contract) storage.Context {
	return storage.Context{}
}

// Create creates a new contract using a set of input parameters:
//     script      contract's bytecode (limited in length by 1M)
//     params      contract's input parameter types, one byte per parameter, see
//                 ParamType in the `smartcontract` package for value
//                 definitions. Maximum number of parameters: 252.
//     returnType  return value type, also a ParamType constant
//     properties  bit field with contract's permissions (storage, dynamic
//                 invoke, payable), see PropertyState in the `smartcontract`
//                 package
//     name        human-readable contract name (no longer than 252 bytes)
//     version     human-readable contract version (no longer than 252 bytes)
//     author      contract's author (no longer than 252 bytes)
//     email       contract's author/support e-mail (no longer than 252 bytes)
//     description human-readable contract description (no longer than 64K bytes)
// It returns this new created Contract when successful (and fails transaction
// if not). It uses `Neo.Contract.Create` syscall.
func Create(
	script []byte,
	params []byte,
	returnType byte,
	properties byte,
	name,
	version,
	author,
	email,
	description string) Contract {
	return Contract{}
}

// Migrate migrates calling contract (that is the one that calls Migrate) to
// the new contract. Its parameters have exactly the same semantics as for
// Create. The old contract will be deleted by this call, if it has any storage
// associated it will be migrated to the new contract. New contract is returned.
// This function uses `Neo.Contract.Migrate` syscall.
func Migrate(
	script []byte,
	params []byte,
	returnType byte,
	properties byte,
	name,
	version,
	author,
	email,
	description string) Contract {
	return Contract{}
}

// Destroy deletes calling contract (the one that calls Destroy) from the
// blockchain, so it's only possible to do that from the contract itself and
// not by any outside code. When contract is deleted all associated storage
// items are deleted too. This function uses `Neo.Contract.Destroy` syscall.
func Destroy() {}
