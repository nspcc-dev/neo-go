/*
Package account provides getter functions for Account interop structure.
To use these functions you need to get an Account first via blockchain.GetAccount
call.
*/
package account

// Account represents NEO account type that is used in interop functions, it's
// an opaque data structure that you can get data from only using functions from
// this package. It's similar in function to the Account class in the Neo .net
// framework.
type Account struct{}

// GetScriptHash returns the script hash of the given Account (20 bytes in BE
// representation). It uses `Neo.Account.GetBalance` syscall internally.
func GetScriptHash(a Account) []byte {
	return nil
}

// GetVotes returns current votes of the given account represented as a slice of
// public keys. Keys are serialized into byte slices in their compressed form (33
// bytes long each). This function uses `Neo.Account.GetVotes` syscall
// internally.
func GetVotes(a Account) [][]byte {
	return nil
}

// GetBalance returns current balance of the given asset (by its ID, 256 bit
// hash in BE form) for the given account. Only native UTXO assets can be
// queiried via this function, for NEP-5 ones use respective contract calls.
// The value returned is represented as an integer with original value multiplied
// by 10⁸ so you can work with fractional parts of the balance too. This function
// uses `Neo.Account.GetBalance` syscall internally.
func GetBalance(a Account, assetID []byte) int {
	return 0
}

// IsStandard checks whether given account uses standard (CHECKSIG or
// CHECKMULTISIG) contract. It only works for deployed contracts and uses
// `Neo.Account.IsStandard` syscall internally.
func IsStandard(a Account) bool {
	return false
}
