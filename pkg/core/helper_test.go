package core

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/block"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var newBlockPrevHash util.Uint256
var unitTestNetCfg config.Config

var privNetKeys = []string{
	"KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY",
	"KzfPUYDC9n2yf4fK5ro4C8KMcdeXtFuEnStycbZgX3GomiUsvX6W",
	"KzgWE3u3EDp13XPXXuTKZxeJ3Gi8Bsm8f9ijY3ZsCKKRvZUo1Cdn",
	"L2oEXKRAAMiPEZukwR5ho2S6SMeQLhcK9mF71ZnF7GvT8dU4Kkgz",
}

// newTestChain should be called before newBlock invocation to properly setup
// global state.
func newTestChain(t *testing.T) *Blockchain {
	var err error
	unitTestNetCfg, err = config.Load("../../config", config.ModeUnitTestNet)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := NewBlockchain(storage.NewMemoryStore(), unitTestNetCfg.ProtocolConfiguration, zaptest.NewLogger(t))
	if err != nil {
		t.Fatal(err)
	}
	go chain.Run()
	zeroHash, err := chain.GetHeader(chain.GetHeaderHash(0))
	require.Nil(t, err)
	newBlockPrevHash = zeroHash.Hash()
	return chain
}

func newBlock(index uint32, txs ...*transaction.Transaction) *block.Block {
	validators, _ := getValidators(unitTestNetCfg.ProtocolConfiguration)
	vlen := len(validators)
	valScript, _ := smartcontract.CreateMultiSigRedeemScript(
		vlen-(vlen-1)/3,
		validators,
	)
	witness := transaction.Witness{
		VerificationScript: valScript,
	}
	b := &block.Block{
		Base: block.Base{
			Version:       0,
			PrevHash:      newBlockPrevHash,
			Timestamp:     uint32(time.Now().UTC().Unix()) + index,
			Index:         index,
			ConsensusData: 1111,
			NextConsensus: witness.ScriptHash(),
			Script:        witness,
		},
		Transactions: txs,
	}
	_ = b.RebuildMerkleRoot()
	newBlockPrevHash = b.Hash()

	invScript := make([]byte, 0)
	for _, wif := range privNetKeys {
		pKey, err := keys.NewPrivateKeyFromWIF(wif)
		if err != nil {
			panic(err)
		}
		b := b.GetHashableData()
		sig, err := pKey.Sign(b)
		if err != nil || len(sig) != 64 {
			panic(err)
		}
		invScript = append(invScript, byte(opcode.PUSHBYTES64))
		invScript = append(invScript, sig...)
	}
	b.Script.InvocationScript = invScript
	return b
}

func makeBlocks(n int) []*block.Block {
	blocks := make([]*block.Block, n)
	for i := 0; i < n; i++ {
		blocks[i] = newBlock(uint32(i+1), newMinerTX())
	}
	return blocks
}

func newMinerTX() *transaction.Transaction {
	return &transaction.Transaction{
		Type: transaction.MinerType,
		Data: &transaction.MinerTX{},
	}
}

func getDecodedBlock(t *testing.T, i int) *block.Block {
	data, err := getBlockData(i)
	if err != nil {
		t.Fatal(err)
	}

	b, err := hex.DecodeString(data["raw"].(string))
	if err != nil {
		t.Fatal(err)
	}

	block := &block.Block{}
	r := io.NewBinReaderFromBuf(b)
	block.DecodeBinary(r)
	if r.Err != nil {
		t.Fatal(r.Err)
	}

	return block
}

func getBlockData(i int) (map[string]interface{}, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("test_data/block_%d.json", i))
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, err
}

func newDumbBlock() *block.Block {
	return &block.Block{
		Base: block.Base{
			Version:       0,
			PrevHash:      hash.Sha256([]byte("a")),
			MerkleRoot:    hash.Sha256([]byte("b")),
			Timestamp:     uint32(100500),
			Index:         1,
			ConsensusData: 1111,
			NextConsensus: hash.Hash160([]byte("a")),
			Script: transaction.Witness{
				VerificationScript: []byte{0x51}, // PUSH1
				InvocationScript:   []byte{0x61}, // NOP
			},
		},
		Transactions: []*transaction.Transaction{
			{Type: transaction.MinerType},
			{Type: transaction.IssueType},
		},
	}
}
