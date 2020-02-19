package core

import (
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/block"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
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
			VerificationScript: []byte{byte(opcode.PUSHT)},
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

	b := &block.Block{
		Base: base,
		Transactions: []*transaction.Transaction{
			{
				Type: transaction.MinerType,
				Data: &transaction.MinerTX{
					Nonce: 2083236893,
				},
				Attributes: []transaction.Attribute{},
				Inputs:     []transaction.Input{},
				Outputs:    []transaction.Output{},
				Scripts:    []transaction.Witness{},
			},
			&governingTokenTX,
			&utilityTokenTX,
			{
				Type:   transaction.IssueType,
				Data:   &transaction.IssueTX{}, // no fields.
				Inputs: []transaction.Input{},
				Outputs: []transaction.Output{
					{
						AssetID:    governingTokenTX.Hash(),
						Amount:     governingTokenTX.Data.(*transaction.RegisterTX).Amount,
						ScriptHash: scriptOut,
					},
				},
				Scripts: []transaction.Witness{
					{
						InvocationScript:   []byte{},
						VerificationScript: []byte{byte(opcode.PUSHT)},
					},
				},
			},
		},
	}

	if err = b.RebuildMerkleRoot(); err != nil {
		return nil, err
	}

	return b, nil
}

func init() {
	admin := hash.Hash160([]byte{byte(opcode.PUSHT)})
	registerTX := &transaction.RegisterTX{
		AssetType: transaction.GoverningToken,
		Name:      "[{\"lang\":\"zh-CN\",\"name\":\"小蚁股\"},{\"lang\":\"en\",\"name\":\"AntShare\"}]",
		Amount:    util.Fixed8FromInt64(100000000),
		Precision: 0,
		Admin:     admin,
	}

	governingTokenTX = transaction.Transaction{
		Type:       transaction.RegisterType,
		Data:       registerTX,
		Attributes: []transaction.Attribute{},
		Inputs:     []transaction.Input{},
		Outputs:    []transaction.Output{},
		Scripts:    []transaction.Witness{},
	}

	admin = hash.Hash160([]byte{byte(opcode.PUSHF)})
	registerTX = &transaction.RegisterTX{
		AssetType: transaction.UtilityToken,
		Name:      "[{\"lang\":\"zh-CN\",\"name\":\"小蚁币\"},{\"lang\":\"en\",\"name\":\"AntCoin\"}]",
		Amount:    calculateUtilityAmount(),
		Precision: 8,
		Admin:     admin,
	}
	utilityTokenTX = transaction.Transaction{
		Type:       transaction.RegisterType,
		Data:       registerTX,
		Attributes: []transaction.Attribute{},
		Inputs:     []transaction.Input{},
		Outputs:    []transaction.Output{},
		Scripts:    []transaction.Witness{},
	}
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
