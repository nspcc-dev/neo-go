package transaction

// TransactionType is the type of a transaction.
type TransactionType uint8

// All processes in NEO system are recorded in transactions.
// There are several types of transactions.
const (
	MinerTX      TransactionType = 0x00
	IssueTX      TransactionType = 0x01
	ClaimTX      TransactionType = 0x02
	EnrollmentTX TransactionType = 0x20
	VotingTX     TransactionType = 0x24
	RegisterTX   TransactionType = 0x40
	ContractTX   TransactionType = 0x80
	AgencyTX     TransactionType = 0xb0
)

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
