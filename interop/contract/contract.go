package contract

// Package contract provides function signatures that can be used inside
// smart contracts that are written in the neo-go-sc framework.

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
