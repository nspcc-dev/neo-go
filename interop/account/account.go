package account

// Package account provides function signatures that can be used inside
// smart contracts that are written in the neo-go-sc framework.

// Account stubs a NEO account type.
type Account struct{}

// GetScripHash returns the script hash of the given account.
func GetScriptHash(a Account) []byte {
	return nil
}

// GetVotes returns the votes of the given account which should be a slice of
// public key raw bytes.
func GetVotes(a Account) [][]byte {
	return nil
}

// GetBalance returns the balance of for the given account and asset id.
func GetBalance(a Account, assetID []byte) int {
	return 0
}
