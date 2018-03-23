package core

import (
	"bytes"
	"encoding/binary"
	"sort"
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// Creates a genesis block based on the given configuration.
func createGenesisBlock(cfg config.ProtocolConfiguration) (*Block, error) {
	validators, err := getValidators(cfg)
	if err != nil {
		return nil, err
	}

	nextConsensus, err := getNextConsensusAddress(validators)
	if err != nil {
		return nil, err
	}

	merkleRoot, err := util.Uint256DecodeString("803ff4abe3ea6533bcc0be574efa02f83ae8fdc651c879056b0d9be336c01bf4")
	if err != nil {
		return nil, err
	}

	base := BlockBase{
		Version:       0,
		PrevHash:      util.Uint256{},
		Timestamp:     uint32(time.Date(2016, 7, 15, 15, 8, 21, 0, time.UTC).Unix()),
		Index:         0,
		ConsensusData: 2083236893,
		MerkleRoot:    merkleRoot,
		NextConsensus: nextConsensus,
		Script: &transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: []byte{byte(vm.Opusht)},
		},
	}

	governingTX := governingTokenTX()
	utilityTX := utilityTokenTX()
	rawScript, err := smartcontract.CreateMultiSigRedeemScript(
		len(cfg.StandbyValidators)/2+1,
		validators,
	)
	if err != nil {
		return nil, err
	}
	scriptOut, err := util.Uint160FromScript(rawScript)
	if err != nil {
		return nil, err
	}

	block := &Block{
		BlockBase: base,
		Transactions: []*transaction.Transaction{
			{
				Type: transaction.MinerType,
				Data: &transaction.MinerTX{
					Nonce: 2083236893,
				},
				Attributes: []*transaction.Attribute{},
				Inputs:     []*transaction.Input{},
				Outputs:    []*transaction.Output{},
				Scripts:    []*transaction.Witness{},
			},
			governingTX,
			utilityTX,
			{
				Type:   transaction.IssueType,
				Data:   &transaction.IssueTX{}, // no fields.
				Inputs: []*transaction.Input{},
				Outputs: []*transaction.Output{
					{
						AssetID:    governingTX.Hash(),
						Amount:     governingTX.Data.(*transaction.RegisterTX).Amount,
						ScriptHash: scriptOut,
					},
				},
				Scripts: []*transaction.Witness{
					{
						InvocationScript:   []byte{},
						VerificationScript: []byte{byte(vm.Opusht)},
					},
				},
			},
		},
	}

	block.createHash()

	return block, nil
}

func governingTokenTX() *transaction.Transaction {
	admin, _ := util.Uint160FromScript([]byte{byte(vm.Opusht)})
	registerTX := &transaction.RegisterTX{
		AssetType: transaction.GoverningToken,
		Name:      "[{\"lang\":\"zh-CN\",\"name\":\"小蚁股\"},{\"lang\":\"en\",\"name\":\"AntShare\"}]",
		Amount:    100000000,
		Precision: 0,
		Owner:     &crypto.PublicKey{},
		Admin:     admin,
	}
	return &transaction.Transaction{
		Data:       registerTX,
		Attributes: []*transaction.Attribute{},
		Inputs:     []*transaction.Input{},
		Outputs:    []*transaction.Output{},
		Scripts:    []*transaction.Witness{},
	}
}

func utilityTokenTX() *transaction.Transaction {
	admin, _ := util.Uint160FromScript([]byte{byte(vm.Opushf)})
	registerTX := &transaction.RegisterTX{
		AssetType: transaction.UtilityToken,
		Name:      "[{\"lang\":\"zh-CN\",\"name\":\"小蚁币\"},{\"lang\":\"en\",\"name\":\"AntCoin\"}]",
		Amount:    calculateUtilityAmount(),
		Precision: 8,
		Owner:     &crypto.PublicKey{},
		Admin:     admin,
	}
	return &transaction.Transaction{
		Data:       registerTX,
		Attributes: []*transaction.Attribute{},
		Inputs:     []*transaction.Input{},
		Outputs:    []*transaction.Output{},
		Scripts:    []*transaction.Witness{},
	}
}

func getValidators(cfg config.ProtocolConfiguration) ([]*crypto.PublicKey, error) {
	validators := make([]*crypto.PublicKey, len(cfg.StandbyValidators))
	for i, pubKeyStr := range cfg.StandbyValidators {
		pubKey, err := crypto.NewPublicKeyFromString(pubKeyStr)
		if err != nil {
			return nil, err
		}
		validators[i] = pubKey
	}
	return validators, nil
}

func getNextConsensusAddress(validators []*crypto.PublicKey) (val util.Uint160, err error) {
	vlen := len(validators)
	raw, err := smartcontract.CreateMultiSigRedeemScript(
		vlen-(vlen-1)/3,
		validators,
	)
	if err != nil {
		return val, err
	}
	return util.Uint160FromScript(raw)
}

func calculateUtilityAmount() util.Fixed8 {
	sum := 0
	for i := 0; i < len(genAmount); i++ {
		sum += genAmount[i]
	}
	return util.Fixed8(sum * decrementInterval)
}

// Utilities for quick bootstrapping blockchains. Normally we should
// create the genisis block. For now (to speed up development) we will add
// The hashes manually.

func GenesisHashPrivNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")
	return hash
}

func GenesisHashTestNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("b3181718ef6167105b70920e4a8fbbd0a0a56aacf460d70e10ba6fa1668f1fef")
	return hash
}

func GenesisHashMainNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")
	return hash
}

// headerSliceReverse reverses the given slice of *Header.
func headerSliceReverse(dest []*Header) {
	for i, j := 0, len(dest)-1; i < j; i, j = i+1, j-1 {
		dest[i], dest[j] = dest[j], dest[i]
	}
}

// storeAsCurrentBlock stores the given block witch prefix
// SYSCurrentBlock.
func storeAsCurrentBlock(batch storage.Batch, block *Block) {
	buf := new(bytes.Buffer)
	buf.Write(block.Hash().BytesReverse())
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, block.Index)
	buf.Write(b)
	batch.Put(storage.SYSCurrentBlock.Bytes(), buf.Bytes())
}

// storeAsBlock stores the given block as DataBlock.
func storeAsBlock(batch storage.Batch, block *Block, sysFee uint32) error {
	var (
		key = storage.AppendPrefix(storage.DataBlock, block.Hash().BytesReverse())
		buf = new(bytes.Buffer)
	)

	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, sysFee)

	b, err := block.Trim()
	if err != nil {
		return err
	}
	buf.Write(b)
	batch.Put(key, buf.Bytes())
	return nil
}

// storeAsTransaction stores the given TX as DataTransaction.
func storeAsTransaction(batch storage.Batch, tx *transaction.Transaction, index uint32) error {
	key := storage.AppendPrefix(storage.DataTransaction, tx.Hash().BytesReverse())
	buf := new(bytes.Buffer)
	if err := tx.EncodeBinary(buf); err != nil {
		return err
	}

	dest := make([]byte, buf.Len()+4)
	binary.LittleEndian.PutUint32(dest[:4], index)
	copy(dest[4:], buf.Bytes())
	batch.Put(key, dest)

	return nil
}

// readStoredHeaderHashes returns a sorted list of header hashes
// retrieved from the given Store.
func readStoredHeaderHashes(store storage.Store) ([]util.Uint256, error) {
	hashMap := make(map[uint32][]util.Uint256)
	store.Seek(storage.IXHeaderHashList.Bytes(), func(k, v []byte) {
		storedCount := binary.LittleEndian.Uint32(k[1:])
		hashes, err := util.Read2000Uint256Hashes(v)
		if err != nil {
			panic(err)
		}
		hashMap[storedCount] = hashes
	})

	var (
		i          = 0
		sortedKeys = make([]int, len(hashMap))
	)

	for k, _ := range hashMap {
		sortedKeys[i] = int(k)
		i++
	}
	sort.Ints(sortedKeys)

	hashes := []util.Uint256{}
	for _, key := range sortedKeys {
		values := hashMap[uint32(key)]
		for _, hash := range values {
			hashes = append(hashes, hash)
		}
	}

	return hashes, nil
}
