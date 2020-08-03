package core

import (
	"errors"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
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

	base := block.Base{
		Version:       0,
		PrevHash:      util.Uint256{},
		Timestamp:     uint64(time.Date(2016, 7, 15, 15, 8, 21, 0, time.UTC).Unix()) * 1000, // Milliseconds.
		Index:         0,
		NextConsensus: nextConsensus,
		Script: transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: []byte{byte(opcode.PUSH1)},
		},
		Network: cfg.Magic,
	}

	b := &block.Block{
		Base: base,
		Transactions: []*transaction.Transaction{
			deployNativeContracts(cfg.Magic),
		},
		ConsensusData: block.ConsensusData{
			PrimaryIndex: 0,
			Nonce:        2083236893,
		},
	}

	if err = b.RebuildMerkleRoot(); err != nil {
		return nil, err
	}

	return b, nil
}

func deployNativeContracts(magic netmode.Magic) *transaction.Transaction {
	buf := io.NewBufBinWriter()
	emit.Syscall(buf.BinWriter, "Neo.Native.Deploy")
	script := buf.Bytes()
	tx := transaction.New(magic, script, 0)
	tx.Nonce = 0
	tx.Signers = []transaction.Signer{
		{
			Account: hash.Hash160([]byte{byte(opcode.PUSH1)}),
			Scopes:  transaction.FeeOnly,
		},
	}
	tx.Scripts = []transaction.Witness{
		{
			InvocationScript:   []byte{},
			VerificationScript: []byte{byte(opcode.PUSH1)},
		},
	}
	return tx
}

func validatorsFromConfig(cfg config.ProtocolConfiguration) ([]*keys.PublicKey, error) {
	if len(cfg.StandbyCommittee) < cfg.ValidatorsCount {
		return nil, errors.New("validators count can be less than the size of StandbyCommittee")
	}
	validators := make([]*keys.PublicKey, cfg.ValidatorsCount)
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
	vlen := len(validators)
	raw, err := smartcontract.CreateMultiSigRedeemScript(
		vlen-(vlen-1)/3,
		validators,
	)
	if err != nil {
		return val, err
	}
	return hash.Hash160(raw), nil
}

func calculateUtilityAmount() util.Fixed8 {
	sum := 0
	for i := 0; i < len(genAmount); i++ {
		sum += genAmount[i]
	}
	return util.Fixed8FromInt64(int64(sum * decrementInterval))
}

// headerSliceReverse reverses the given slice of *Header.
func headerSliceReverse(dest []*block.Header) {
	for i, j := 0, len(dest)-1; i < j; i, j = i+1, j-1 {
		dest[i], dest[j] = dest[j], dest[i]
	}
}

// CalculateNetworkFee returns network fee for transaction
func CalculateNetworkFee(script []byte) (int64, int) {
	var (
		netFee int64
		size   int
	)
	if vm.IsSignatureContract(script) {
		size += 67 + io.GetVarSize(script)
		netFee += opcodePrice(opcode.PUSHDATA1, opcode.PUSHNULL) + crypto.ECDSAVerifyPrice
	} else if m, pubs, ok := vm.ParseMultiSigContract(script); ok {
		n := len(pubs)
		sizeInv := 66 * m
		size += io.GetVarSize(sizeInv) + sizeInv + io.GetVarSize(script)
		netFee += calculateMultisigFee(m) + calculateMultisigFee(n)
		netFee += opcodePrice(opcode.PUSHNULL) + crypto.ECDSAVerifyPrice*int64(n)
	} else {
		// We can support more contract types in the future.
	}
	return netFee, size
}

func calculateMultisigFee(n int) int64 {
	result := opcodePrice(opcode.PUSHDATA1) * int64(n)
	bw := io.NewBufBinWriter()
	emit.Int(bw.BinWriter, int64(n))
	// it's a hack because prices of small PUSH* opcodes are equal
	result += opcodePrice(opcode.Opcode(bw.Bytes()[0]))
	return result
}
