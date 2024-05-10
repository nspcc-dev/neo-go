package nativenames

// Names of all native contracts.
const (
	Management  = "ContractManagement"
	Ledger      = "LedgerContract"
	Neo         = "NeoToken"
	Gas         = "GasToken"
	Policy      = "PolicyContract"
	Oracle      = "OracleContract"
	Designation = "RoleManagement"
	Notary      = "Notary"
	CryptoLib   = "CryptoLib"
	StdLib      = "StdLib"
)

// All contains the list of all native contract names ordered by the contract ID.
var All = []string{
	Management,
	StdLib,
	CryptoLib,
	Ledger,
	Neo,
	Gas,
	Policy,
	Designation,
	Oracle,
	Notary,
}

// IsValid checks if the name is a valid native contract's name.
func IsValid(name string) bool {
	return name == Management ||
		name == Ledger ||
		name == Neo ||
		name == Gas ||
		name == Policy ||
		name == Oracle ||
		name == Designation ||
		name == Notary ||
		name == CryptoLib ||
		name == StdLib
}
