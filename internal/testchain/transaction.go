package testchain

import (
	"encoding/json"
	"fmt"
	gio "io"
	"strings"

	clisc "github.com/nspcc-dev/neo-go/cli/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Ledger is an interface that abstracts the implementation of the blockchain.
type Ledger interface {
	BlockHeight() uint32
	FeePerByte() int64
	GetBaseExecFee() int64
	GetHeader(hash util.Uint256) (*block.Header, error)
	GetHeaderHash(int) util.Uint256
	HeaderHeight() uint32
	ManagementContractHash() util.Uint160
}

var (
	ownerHash   = MultisigScriptHash()
	ownerScript = MultisigVerificationScript()
)

// NewTransferFromOwner returns a transaction transferring funds from NEO and GAS owner.
func NewTransferFromOwner(bc Ledger, contractHash, to util.Uint160, amount int64,
	nonce, validUntil uint32) (*transaction.Transaction, error) {
	script, err := smartcontract.CreateCallWithAssertScript(contractHash, "transfer", ownerHash, to, amount, nil)
	if err != nil {
		return nil, err
	}

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

// NewDeployTx returns a new deployment transaction for a contract with the source from r and a name equal to
// the filename without '.go' suffix.
func NewDeployTx(bc Ledger, name string, sender util.Uint160, r gio.Reader, confFile *string) (*transaction.Transaction, util.Uint160, []byte, error) {
	// nef.NewFile() cares about version a lot.
	config.Version = "0.90.0-test"

	o := &compiler.Options{
		Name:            strings.TrimSuffix(name, ".go"),
		NoStandardCheck: true,
		NoEventsCheck:   true,
	}
	if confFile != nil {
		conf, err := clisc.ParseContractConfig(*confFile)
		if err != nil {
			return nil, util.Uint160{}, nil, fmt.Errorf("failed to parse configuration: %w", err)
		}
		o.Name = conf.Name
		o.SourceURL = conf.SourceURL
		o.ContractEvents = conf.Events
		o.ContractSupportedStandards = conf.SupportedStandards
		o.Permissions = make([]manifest.Permission, len(conf.Permissions))
		for i := range conf.Permissions {
			o.Permissions[i] = manifest.Permission(conf.Permissions[i])
		}
		o.SafeMethods = conf.SafeMethods
		o.Overloads = conf.Overloads
	}

	ne, di, err := compiler.CompileWithOptions(name, r, o)
	if err != nil {
		return nil, util.Uint160{}, nil, err
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
	script, err := smartcontract.CreateCallScript(bc.ManagementContractHash(), "deploy", neb, rawManifest)
	if err != nil {
		return nil, util.Uint160{}, nil, err
	}

	tx := transaction.New(script, 100*native.GASFactor)
	tx.Signers = []transaction.Signer{{Account: sender}}
	h := state.CreateContractHash(tx.Sender(), ne.Checksum, m.Name)

	return tx, h, ne.Script, nil
}

// SignTx signs the provided transactions with validator keys.
func SignTx(bc Ledger, txs ...*transaction.Transaction) error {
	signTxGeneric(bc, Sign, ownerScript, txs...)
	return nil
}

// SignTxCommittee signs transactions by committee.
func SignTxCommittee(bc Ledger, txs ...*transaction.Transaction) error {
	signTxGeneric(bc, SignCommittee, CommitteeVerificationScript(), txs...)
	return nil
}

func signTxGeneric(bc Ledger, sign func(hash.Hashable) []byte, verif []byte, txs ...*transaction.Transaction) {
	for _, tx := range txs {
		size := io.GetVarSize(tx)
		netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), verif)
		tx.NetworkFee += netFee
		size += sizeDelta
		tx.NetworkFee += int64(size) * bc.FeePerByte()
		tx.Scripts = []transaction.Witness{{
			InvocationScript:   sign(tx),
			VerificationScript: verif,
		}}
	}
}
