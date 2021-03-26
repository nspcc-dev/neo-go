package core

import (
	"errors"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

var (
	// governingTokenTX represents transaction that is used to create
	// governing (NEO) token. It's a part of the genesis block.
	governingTokenTX transaction.Transaction

	// utilityTokenTX represents transaction that is used to create
	// utility (GAS) token. It's a part of the genesis block. It's mostly
	// useful for its hash that represents GAS asset ID.
	utilityTokenTX transaction.Transaction
)

// createGenesisBlock creates a genesis block based on the given configuration.
func createGenesisBlock(cfg config.ProtocolConfiguration) (*block.Block, error) {
	validators, err := validatorsFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	nextConsensus, err := getNextConsensusAddress(validators)
	if err != nil {
		return nil, err
	}

	base := block.Header{
		Version:       0,
		PrevHash:      util.Uint256{},
		Timestamp:     uint64(time.Date(2016, 7, 15, 15, 8, 21, 0, time.UTC).Unix()) * 1000, // Milliseconds.
		Index:         0,
		NextConsensus: nextConsensus,
		Script: transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: []byte{byte(opcode.PUSH1)},
		},
		StateRootEnabled: cfg.StateRootInHeader,
	}

	b := &block.Block{
		Header:       base,
		Transactions: []*transaction.Transaction{},
	}
	b.RebuildMerkleRoot()

	return b, nil
}

func validatorsFromConfig(cfg config.ProtocolConfiguration) ([]*keys.PublicKey, error) {
	vs, err := committeeFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return vs[:cfg.ValidatorsCount], nil
}

func committeeFromConfig(cfg config.ProtocolConfiguration) ([]*keys.PublicKey, error) {
	if len(cfg.StandbyCommittee) < cfg.ValidatorsCount {
		return nil, errors.New("validators count can be less than the size of StandbyCommittee")
	}
	validators := make([]*keys.PublicKey, len(cfg.StandbyCommittee))
	for i := range validators {
		pubKey, err := keys.NewPublicKeyFromString(cfg.StandbyCommittee[i])
		if err != nil {
			return nil, err
		}
		validators[i] = pubKey
	}
	return validators, nil
}

func getNextConsensusAddress(validators []*keys.PublicKey) (val util.Uint160, err error) {
	raw, err := smartcontract.CreateDefaultMultiSigRedeemScript(validators)
	if err != nil {
		return val, err
	}
	return hash.Hash160(raw), nil
}

// headerSliceReverse reverses the given slice of *Header.
func headerSliceReverse(dest []*block.Header) {
	for i, j := 0, len(dest)-1; i < j; i, j = i+1, j-1 {
		dest[i], dest[j] = dest[j], dest[i]
	}
}
