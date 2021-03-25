package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Takes 1 minute for 100 tx per block and 5000 blocks.
const (
	defaultBlockCount = 5000
	defaultTxPerBlock = 100
)

var (
	outFile    = flag.String("out", "", "filename to write dump to")
	blockCount = flag.Uint("blocks", defaultBlockCount, "number of blocks to generate")
	txPerBlock = flag.Uint("txs", defaultTxPerBlock, "number of blocks to generate")
)

func main() {
	flag.Parse()

	if *outFile == "" {
		handleError("", errors.New("output file is not provided"))
	}
	outStream, err := os.Create(*outFile) // fail early
	handleError("can't open output file", err)
	defer outStream.Close()

	const contract = `
	package contract
	import "github.com/nspcc-dev/neo-go/pkg/interop/storage"
	var ctx = storage.GetContext()
	func Put(key, value []byte) {
		storage.Put(ctx, key, value)
	}`

	acc, err := wallet.NewAccount()
	handleError("can't create new account", err)
	h := acc.Contract.ScriptHash()

	bc, err := newChain()
	handleError("can't initialize blockchain", err)

	valScript, err := smartcontract.CreateDefaultMultiSigRedeemScript(bc.GetStandByValidators())
	handleError("can't create verification script", err)
	lastBlock, err := bc.GetBlock(bc.GetHeaderHash(int(bc.BlockHeight())))
	handleError("can't fetch last block", err)

	txMoveNeo, err := testchain.NewTransferFromOwner(bc, bc.GoverningTokenHash(), h, native.NEOTotalSupply, 0, 2)
	handleError("can't transfer NEO", err)
	txMoveGas, err := testchain.NewTransferFromOwner(bc, bc.UtilityTokenHash(), h, 2_000_000_000_000_000, 0, 2)
	handleError("can't tranfser GAS", err)
	lastBlock = addBlock(bc, lastBlock, valScript, txMoveNeo, txMoveGas)

	tx, contractHash, _, err := testchain.NewDeployTx(bc, "DumpContract", h, strings.NewReader(contract), nil)
	handleError("can't create deploy tx", err)
	tx.NetworkFee = 10_000_000
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	handleError("can't sign deploy tx", acc.SignTx(netmode.UnitTestNet, tx))
	lastBlock = addBlock(bc, lastBlock, valScript, tx)

	key := make([]byte, 10)
	value := make([]byte, 10)
	nonce := uint32(0)

	blocksNum := uint32(*blockCount)
	txNum := int(*txPerBlock)
	for i := bc.BlockHeight(); i < blocksNum; i++ {
		txs := make([]*transaction.Transaction, txNum)
		for j := 0; j < txNum; j++ {
			nonce++
			rand.Read(key)
			rand.Read(value)

			w := io.NewBufBinWriter()
			emit.AppCall(w.BinWriter, contractHash, "put", callflag.All, key, value)
			handleError("can't create transaction", w.Err)

			tx := transaction.New(w.Bytes(), 4_000_000)
			tx.ValidUntilBlock = i + 1
			tx.NetworkFee = 4_000_000
			tx.Nonce = nonce
			tx.Signers = []transaction.Signer{{
				Account: h,
				Scopes:  transaction.CalledByEntry,
			}}
			handleError("can't sign tx", acc.SignTx(netmode.UnitTestNet, tx))

			txs[j] = tx
		}
		lastBlock = addBlock(bc, lastBlock, valScript, txs...)
	}

	w := io.NewBinWriterFromIO(outStream)
	w.WriteU32LE(bc.BlockHeight() + 1)
	handleError("error during dump", chaindump.Dump(bc, w, 0, bc.BlockHeight()+1))
}

func handleError(msg string, err error) {
	if err != nil {
		fmt.Printf("%s: %v\n", msg, err)
		os.Exit(1)
	}
}

func newChain() (*core.Blockchain, error) {
	unitTestNetCfg, err := config.Load("./config", netmode.UnitTestNet)
	if err != nil {
		return nil, err
	}
	unitTestNetCfg.ProtocolConfiguration.VerifyBlocks = false
	zapCfg := zap.NewDevelopmentConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	log, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	chain, err := core.NewBlockchain(storage.NewMemoryStore(), unitTestNetCfg.ProtocolConfiguration, log)
	if err != nil {
		return nil, err
	}
	go chain.Run()
	return chain, nil
}

func addBlock(bc *core.Blockchain, lastBlock *block.Block, script []byte, txs ...*transaction.Transaction) *block.Block {
	b, err := newBlock(bc, lastBlock, script, txs...)
	if err != nil {
		handleError("can't create block", err)
	}
	if err := bc.AddBlock(b); err != nil {
		handleError("can't add block", err)
	}
	return b
}

func newBlock(bc *core.Blockchain, lastBlock *block.Block, script []byte, txs ...*transaction.Transaction) (*block.Block, error) {
	witness := transaction.Witness{VerificationScript: script}
	b := &block.Block{
		Header: block.Header{
			Network:       netmode.UnitTestNet,
			PrevHash:      lastBlock.Hash(),
			Timestamp:     uint64(time.Now().UTC().Unix())*1000 + uint64(lastBlock.Index),
			Index:         lastBlock.Index + 1,
			NextConsensus: witness.ScriptHash(),
			Script:        witness,
		},
		Transactions: txs,
	}
	if bc.GetConfig().StateRootInHeader {
		sr, err := bc.GetStateModule().GetStateRoot(bc.BlockHeight())
		if err != nil {
			return nil, err
		}
		b.StateRootEnabled = true
		b.PrevStateRoot = sr.Root
	}
	b.RebuildMerkleRoot()
	b.Script.InvocationScript = testchain.Sign(b)
	return b, nil
}
