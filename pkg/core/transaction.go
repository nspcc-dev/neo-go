package core

import (
	"encoding/binary"
	"io"
)

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

// All processes in NEO system are recorded in transactions.
// There are several types of transactions.
const (
	MinerTX      TransactionType = 0x00
	IssueTX                      = 0x01
	ClaimTX                      = 0x02
	EnrollmentTX                 = 0x20
	VotingTX                     = 0x24
	RegisterTX                   = 0x40
	ContractTX                   = 0x80
	AgencyTX                     = 0xb0
)

// Transaction is a process recorded in the NEO system.
type Transaction struct {
	Type TransactionType
}

// DecodeBinary implements the payload interface.
func (t *Transaction) DecodeBinary(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &t.Type)
	return err
}

// EncodeBinary implements the payload interface.
func (t *Transaction) EncodeBinary(w io.Writer) error {
	return nil
}
