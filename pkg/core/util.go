package core

import (
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
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
	validators, err := getValidators(cfg)
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
		Timestamp:     uint32(time.Date(2016, 7, 15, 15, 8, 21, 0, time.UTC).Unix()),
		Index:         0,
		ConsensusData: 2083236893,
		NextConsensus: nextConsensus,
		Script: transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: []byte{byte(opcode.OLDPUSH1)},
		},
	}

	rawScript, err := smartcontract.CreateMultiSigRedeemScript(
		len(cfg.StandbyValidators)/2+1,
		validators,
	)
	if err != nil {
		return nil, err
	}
	scriptOut := hash.Hash160(rawScript)

	minerTx := transaction.NewMinerTXWithNonce(2083236893)
	minerTx.Sender = hash.Hash160([]byte{byte(opcode.PUSH1)})

	issueTx := transaction.NewIssueTX()
	// TODO NEO3.0: nonce should be constant to avoid variability of genesis block
	issueTx.Nonce = 0
	issueTx.Sender = hash.Hash160([]byte{byte(opcode.OLDPUSH1)})
	issueTx.Outputs = []transaction.Output{
		{
			AssetID:    governingTokenTX.Hash(),
			Amount:     governingTokenTX.Data.(*transaction.RegisterTX).Amount,
			ScriptHash: scriptOut,
		},
	}
	issueTx.Scripts = []transaction.Witness{
		{
			InvocationScript:   []byte{},
			VerificationScript: []byte{byte(opcode.OLDPUSH1)},
		},
	}

	b := &block.Block{
		Base: base,
		Transactions: []*transaction.Transaction{
			minerTx,
			&governingTokenTX,
			&utilityTokenTX,
			issueTx,
			deployNativeContracts(),
		},
	}

	if err = b.RebuildMerkleRoot(); err != nil {
		return nil, err
	}

	return b, nil
}

func deployNativeContracts() *transaction.Transaction {
	buf := io.NewBufBinWriter()
	emit.Syscall(buf.BinWriter, "Neo.Native.Deploy")
	script := buf.Bytes()
	tx := transaction.NewInvocationTX(script, 0)
	tx.Nonce = 0
	tx.Sender = hash.Hash160([]byte{byte(opcode.PUSH1)})
	tx.Scripts = []transaction.Witness{
		{
			InvocationScript:   []byte{},
			VerificationScript: []byte{byte(opcode.PUSH1)},
		},
	}
	return tx
}

func init() {
	admin := hash.Hash160([]byte{byte(opcode.OLDPUSH1)})
	registerTX := &transaction.RegisterTX{
		AssetType: transaction.GoverningToken,
		Name:      "[{\"lang\":\"zh-CN\",\"name\":\"小蚁股\"},{\"lang\":\"en\",\"name\":\"AntShare\"}]",
		Amount:    util.Fixed8FromInt64(100000000),
		Precision: 0,
		Admin:     admin,
	}

	governingTokenTX = *transaction.NewRegisterTX(registerTX)
	// TODO NEO3.0: nonce should be constant to avoid variability of token hash
	governingTokenTX.Nonce = 0
	governingTokenTX.Sender = hash.Hash160([]byte{byte(opcode.OLDPUSH1)})

	admin = hash.Hash160([]byte{0x00})
	registerTX = &transaction.RegisterTX{
		AssetType: transaction.UtilityToken,
		Name:      "[{\"lang\":\"zh-CN\",\"name\":\"小蚁币\"},{\"lang\":\"en\",\"name\":\"AntCoin\"}]",
		Amount:    calculateUtilityAmount(),
		Precision: 8,
		Admin:     admin,
	}
	utilityTokenTX = *transaction.NewRegisterTX(registerTX)
	// TODO NEO3.0: nonce should be constant to avoid variability of token hash
	utilityTokenTX.Nonce = 0
	utilityTokenTX.Sender = hash.Hash160([]byte{byte(opcode.OLDPUSH1)})
}

// GoverningTokenID returns the governing token (NEO) hash.
func GoverningTokenID() util.Uint256 {
	return governingTokenTX.Hash()
}

// UtilityTokenID returns the utility token (GAS) hash.
func UtilityTokenID() util.Uint256 {
	return utilityTokenTX.Hash()
}

func getValidators(cfg config.ProtocolConfiguration) ([]*keys.PublicKey, error) {
	validators := make([]*keys.PublicKey, len(cfg.StandbyValidators))
	for i, pubKeyStr := range cfg.StandbyValidators {
		pubKey, err := keys.NewPublicKeyFromString(pubKeyStr)
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
