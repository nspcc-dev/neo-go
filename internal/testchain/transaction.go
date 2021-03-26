package testchain

import (
	"encoding/json"
	"fmt"
	gio "io"

	"github.com/nspcc-dev/neo-go/cli/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

var (
	ownerHash   = MultisigScriptHash()
	ownerScript = MultisigVerificationScript()
)

// NewTransferFromOwner returns transaction transferring funds from NEO and GAS owner.
func NewTransferFromOwner(bc blockchainer.Blockchainer, contractHash, to util.Uint160, amount int64,
	nonce, validUntil uint32) (*transaction.Transaction, error) {
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, contractHash, "transfer", callflag.All, ownerHash, to, amount, nil)
	emit.Opcodes(w.BinWriter, opcode.ASSERT)
	if w.Err != nil {
		return nil, w.Err
	}

	script := w.Bytes()
	tx := transaction.New(script, 11000000)
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
func NewDeployTx(bc blockchainer.Blockchainer, name string, sender util.Uint160, r gio.Reader, confFile *string) (*transaction.Transaction, util.Uint160, []byte, error) {
	// nef.NewFile() cares about version a lot.
	config.Version = "0.90.0-test"

	avm, di, err := compiler.CompileWithDebugInfo(name, r)
	if err != nil {
		return nil, util.Uint160{}, nil, err
	}

	ne, err := nef.NewFile(avm)
	if err != nil {
		return nil, util.Uint160{}, nil, err
	}

	o := &compiler.Options{
		Name:            name,
		NoStandardCheck: true,
		NoEventsCheck:   true,
	}
	if confFile != nil {
		conf, err := smartcontract.ParseContractConfig(*confFile)
		if err != nil {
			return nil, util.Uint160{}, nil, fmt.Errorf("failed to parse configuration: %w", err)
		}
		o.Name = conf.Name
		o.ContractEvents = conf.Events
		o.ContractSupportedStandards = conf.SupportedStandards
		o.SafeMethods = conf.SafeMethods

	}
	m, err := compiler.CreateManifest(di, o)
	if err != nil {
		return nil, util.Uint160{}, nil, fmt.Errorf("failed to create manifest: %w", err)
	}

	rawManifest, err := json.Marshal(m)
	if err != nil {
		return nil, util.Uint160{}, nil, err
	}
	neb, err := ne.Bytes()
	if err != nil {
		return nil, util.Uint160{}, nil, err
	}
	buf := io.NewBufBinWriter()
	emit.AppCall(buf.BinWriter, bc.ManagementContractHash(), "deploy", callflag.All, neb, rawManifest)
	if buf.Err != nil {
		return nil, util.Uint160{}, nil, buf.Err
	}

	tx := transaction.New(buf.Bytes(), 100*native.GASFactor)
	tx.Signers = []transaction.Signer{{Account: sender}}
	h := state.CreateContractHash(tx.Sender(), ne.Checksum, name)

	return tx, h, avm, nil
}

// SignTx signs provided transactions with validator keys.
func SignTx(bc blockchainer.Blockchainer, txs ...*transaction.Transaction) error {
	signTxGeneric(bc, Sign, ownerScript, txs...)
	return nil
}

// SignTxCommittee signs transactions by committee.
func SignTxCommittee(bc blockchainer.Blockchainer, txs ...*transaction.Transaction) error {
	signTxGeneric(bc, SignCommittee, CommitteeVerificationScript(), txs...)
	return nil
}

func signTxGeneric(bc blockchainer.Blockchainer, sign func(hash.Hashable) []byte, verif []byte, txs ...*transaction.Transaction) {
	for _, tx := range txs {
		size := io.GetVarSize(tx)
		netFee, sizeDelta := fee.Calculate(bc.GetPolicer().GetBaseExecFee(), verif)
		tx.NetworkFee += netFee
		size += sizeDelta
		tx.NetworkFee += int64(size) * bc.FeePerByte()
		tx.Scripts = []transaction.Witness{{
			InvocationScript:   sign(tx),
			VerificationScript: verif,
		}}
	}
}
