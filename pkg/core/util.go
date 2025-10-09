package core

import (
	"fmt"
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

// CreateGenesisBlock creates a genesis block based on the given configuration.
func CreateGenesisBlock(cfg config.ProtocolConfiguration) (*block.Block, error) {
	validators, committee, err := validatorsFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	nextConsensus, err := getNextConsensusAddress(validators)
	if err != nil {
		return nil, err
	}

	txs := []*transaction.Transaction{}
	if cfg.Genesis.Transaction != nil {
		committeeH, err := getCommitteeAddress(committee)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate committee address: %w", err)
		}
		tx := cfg.Genesis.Transaction
		signers := []transaction.Signer{
			{
				Account: nextConsensus,
				Scopes:  transaction.CalledByEntry,
			},
		}
		scripts := []transaction.Witness{
			{
				InvocationScript:   []byte{},
				VerificationScript: []byte{byte(opcode.PUSH1)},
			},
		}
		if !committeeH.Equals(nextConsensus) {
			signers = append(signers, []transaction.Signer{
				{
					Account: committeeH,
					Scopes:  transaction.CalledByEntry,
				},
			}...)
			scripts = append(scripts, []transaction.Witness{
				{
					InvocationScript:   []byte{},
					VerificationScript: []byte{byte(opcode.PUSH1)},
				},
			}...)
		}

		txs = append(txs, &transaction.Transaction{
			SystemFee:       tx.SystemFee,
			ValidUntilBlock: 1,
			Script:          tx.Script,
			Signers:         signers,
			Scripts:         scripts,
		})
	}

	genesisVersion := block.VersionInitial
	height, ok := cfg.Hardforks[config.HFFaun.String()]
	if ok && height == 0 {
		genesisVersion = block.VersionFaun
	}

	base := block.Header{
		Version:       genesisVersion,
		PrevHash:      util.Uint256{},
		PrevStateRoot: util.Uint256{},
		Timestamp:     uint64(time.Date(2016, 7, 15, 15, 8, 21, 0, time.UTC).Unix()) * 1000, // Milliseconds.
		Nonce:         2083236893,
		Index:         0,
		NextConsensus: nextConsensus,
		Script: transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: []byte{byte(opcode.PUSH1)},
		},
	}

	b := &block.Block{
		Header:       base,
		Transactions: txs,
	}
	b.RebuildMerkleRoot()

	return b, nil
}

func validatorsFromConfig(cfg config.ProtocolConfiguration) ([]*keys.PublicKey, []*keys.PublicKey, error) {
	vs, err := keys.NewPublicKeysFromStrings(cfg.StandbyCommittee)
	if err != nil {
		return nil, nil, err
	}
	return vs.Copy()[:cfg.GetNumOfCNs(0)], vs, nil
}

func getNextConsensusAddress(validators []*keys.PublicKey) (val util.Uint160, err error) {
	raw, err := smartcontract.CreateDefaultMultiSigRedeemScript(validators)
	if err != nil {
		return val, err
	}
	return hash.Hash160(raw), nil
}

func getCommitteeAddress(committee []*keys.PublicKey) (val util.Uint160, err error) {
	raw, err := smartcontract.CreateMajorityMultiSigRedeemScript(committee)
	if err != nil {
		return val, err
	}
	return hash.Hash160(raw), nil
}
