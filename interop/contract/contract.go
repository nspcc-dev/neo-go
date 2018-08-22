package contract

import "github.com/CityOfZion/neo-storm/interop/storage"

// Package contract provides function signatures that can be used inside
// smart contracts that are written in the neo-storm framework.

// Contract stubs a NEO contract type.
type Contract struct{}

// GetScript returns the script of the given contract.
func GetScript(c Contract) []byte {
	return nil
}

// IsPayable returns whether the given contract is payable.
func IsPayable(c Contract) bool {
	return false
}

// GetStorageContext returns the storage context for the given contract.
func GetStorageContext(c Contract) storage.Context {
	return storage.Context{}
}

// Create creates a new contract.
// @FIXME What is the type of the returnType here?
func Create(
	script []byte,
	params []interface{},
	returnType byte,
	properties interface{},
	name,
	version,
	author,
	email,
	description string) {
}

// Migrate migrates a new contract.
// @FIXME What is the type of the returnType here?
func Migrate(
	script []byte,
	params []interface{},
	returnType byte,
	properties interface{},
	name,
	version,
	author,
	email,
	description string) {
}

// Destroy deletes a contract that is registered on the blockchain.
func Destroy(c Contract) {}
