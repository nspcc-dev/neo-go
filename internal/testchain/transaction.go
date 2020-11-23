package testchain

import (
	"encoding/json"
	gio "io"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

var (
	ownerHash   = MultisigScriptHash()
	ownerScript = MultisigVerificationScript()
)

// NewTransferFromOwner returns transaction transfering funds from NEO and GAS owner.
func NewTransferFromOwner(bc blockchainer.Blockchainer, contractHash, to util.Uint160, amount int64,
	nonce, validUntil uint32) (*transaction.Transaction, error) {
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, contractHash, "transfer", ownerHash, to, amount, nil)
	emit.Opcodes(w.BinWriter, opcode.ASSERT)
	if w.Err != nil {
		return nil, w.Err
	}

	script := w.Bytes()
	tx := transaction.New(netmode.UnitTestNet, script, 10000000)
	tx.ValidUntilBlock = validUntil
	tx.Nonce = nonce
	tx.Signers = []transaction.Signer{{
		Account:          ownerHash,
		Scopes:           transaction.CalledByEntry,
		AllowedContracts: nil,
		AllowedGroups:    nil,
	}}
	return tx, SignTx(bc, tx)
}

// NewDeployTx returns new deployment tx for contract with name with Go code read from r.
func NewDeployTx(name string, r gio.Reader) (*transaction.Transaction, []byte, error) {
	avm, di, err := compiler.CompileWithDebugInfo(name, r)
	if err != nil {
		return nil, nil, err
	}

	w := io.NewBufBinWriter()
	m, err := di.ConvertToManifest(name, nil)
	if err != nil {
		return nil, nil, err
	}
	bs, err := json.Marshal(m)
	if err != nil {
		return nil, nil, err
	}
	emit.Bytes(w.BinWriter, bs)
	emit.Bytes(w.BinWriter, avm)
	emit.Syscall(w.BinWriter, interopnames.SystemContractCreate)
	if w.Err != nil {
		return nil, nil, err
	}
	return transaction.New(Network(), w.Bytes(), 100*native.GASFactor), avm, nil
}

// SignTx signs provided transactions with validator keys.
func SignTx(bc blockchainer.Blockchainer, txs ...*transaction.Transaction) error {
	for _, tx := range txs {
		size := io.GetVarSize(tx)
		netFee, sizeDelta := fee.Calculate(ownerScript)
		tx.NetworkFee += netFee
		size += sizeDelta
		tx.NetworkFee += int64(size) * bc.FeePerByte()
		data := tx.GetSignedPart()
		tx.Scripts = []transaction.Witness{{
			InvocationScript:   Sign(data),
			VerificationScript: ownerScript,
		}}
	}
	return nil
}
