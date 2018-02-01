package core

// TransactionType is the type of a transaction.
type TransactionType uint8

// String implements the stringer interface.
func (t TransactionType) String() string {
	switch t {
	case MinerTX:
		return "miner transaction"
	case IssueTX:
		return "issue transaction"
	case ClaimTX:
		return "claim transaction"
	case EnrollmentTX:
		return "enrollment transaction"
	case VotingTX:
		return "voting transaction"
	case RegisterTX:
		return "register transaction"
	case ContractTX:
		return "contract transaction"
	case AgencyTX:
		return "agency transaction"
	default:
		return ""
	}
}
