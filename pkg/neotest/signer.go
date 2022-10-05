package neotest

import (
	"bytes"
	"fmt"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

// Signer is a generic interface which can be either a simple- or multi-signature signer.
type Signer interface {
	// ScriptHash returns a signer script hash.
	Script() []byte
	// Script returns a signer verification script.
	ScriptHash() util.Uint160
	// SignHashable returns an invocation script for signing an item.
	SignHashable(uint32, hash.Hashable) []byte
	// SignTx signs a transaction.
	SignTx(netmode.Magic, *transaction.Transaction) error
}

// SingleSigner is a generic interface for a simple one-signature signer.
type SingleSigner interface {
	Signer
	// Account returns the underlying account which can be used to
	// get a public key and/or sign arbitrary things.
	Account() *wallet.Account
}

// MultiSigner is an interface for multisignature signing account.
type MultiSigner interface {
	Signer
	// Single returns a simple-signature signer for the n-th account in a list.
	Single(n int) SingleSigner
}

// signer represents a simple-signature signer.
type signer wallet.Account

// multiSigner represents a single multi-signature signer consisting of the provided accounts.
type multiSigner struct {
	accounts []*wallet.Account
	m        int
}

// NewSingleSigner returns a multi-signature signer for the provided account.
// It must contain exactly as many accounts as needed to sign the script.
func NewSingleSigner(acc *wallet.Account) SingleSigner {
	if !vm.IsSignatureContract(acc.Contract.Script) {
		panic("account must have simple-signature verification script")
	}
	return (*signer)(acc)
}

// Script implements Signer interface.
func (s *signer) Script() []byte {
	return (*wallet.Account)(s).Contract.Script
}

// ScriptHash implements Signer interface.
func (s *signer) ScriptHash() util.Uint160 {
	return (*wallet.Account)(s).Contract.ScriptHash()
}

// SignHashable implements Signer interface.
func (s *signer) SignHashable(magic uint32, item hash.Hashable) []byte {
	return append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen},
		(*wallet.Account)(s).SignHashable(netmode.Magic(magic), item)...)
}

// SignTx implements Signer interface.
func (s *signer) SignTx(magic netmode.Magic, tx *transaction.Transaction) error {
	return (*wallet.Account)(s).SignTx(magic, tx)
}

// Account implements SingleSigner interface.
func (s *signer) Account() *wallet.Account {
	return (*wallet.Account)(s)
}

// NewMultiSigner returns a multi-signature signer for the provided account.
// It must contain at least as many accounts as needed to sign the script.
func NewMultiSigner(accs ...*wallet.Account) MultiSigner {
	if len(accs) == 0 {
		panic("empty account list")
	}
	script := accs[0].Contract.Script
	m, _, ok := vm.ParseMultiSigContract(script)
	if !ok {
		panic("all accounts must have multi-signature verification script")
	}
	if len(accs) < m {
		panic(fmt.Sprintf("verification script requires %d signatures, "+
			"but only %d accounts were provided", m, len(accs)))
	}
	sort.Slice(accs, func(i, j int) bool {
		p1 := accs[i].PublicKey()
		p2 := accs[j].PublicKey()
		return p1.Cmp(p2) == -1
	})
	for _, acc := range accs {
		if !bytes.Equal(script, acc.Contract.Script) {
			panic("all accounts must have equal verification script")
		}
	}

	return multiSigner{accounts: accs, m: m}
}

// ScriptHash implements Signer interface.
func (m multiSigner) ScriptHash() util.Uint160 {
	return m.accounts[0].Contract.ScriptHash()
}

// Script implements Signer interface.
func (m multiSigner) Script() []byte {
	return m.accounts[0].Contract.Script
}

// SignHashable implements Signer interface.
func (m multiSigner) SignHashable(magic uint32, item hash.Hashable) []byte {
	var script []byte
	for i := 0; i < m.m; i++ {
		sign := m.accounts[i].SignHashable(netmode.Magic(magic), item)
		script = append(script, byte(opcode.PUSHDATA1), keys.SignatureLen)
		script = append(script, sign...)
	}
	return script
}

// SignTx implements Signer interface.
func (m multiSigner) SignTx(magic netmode.Magic, tx *transaction.Transaction) error {
	invoc := m.SignHashable(uint32(magic), tx)
	verif := m.Script()
	for i := range tx.Scripts {
		if bytes.Equal(tx.Scripts[i].VerificationScript, verif) {
			tx.Scripts[i].InvocationScript = invoc
			return nil
		}
	}
	tx.Scripts = append(tx.Scripts, transaction.Witness{
		InvocationScript:   invoc,
		VerificationScript: verif,
	})
	return nil
}

// Single implements MultiSigner interface.
func (m multiSigner) Single(n int) SingleSigner {
	if len(m.accounts) <= n {
		panic("invalid index")
	}
	return NewSingleSigner(wallet.NewAccountFromPrivateKey(m.accounts[n].PrivateKey()))
}

func checkMultiSigner(t testing.TB, s Signer) {
	ms, ok := s.(multiSigner)
	require.True(t, ok, "expected to be a multi-signer")

	accs := ms.accounts
	require.True(t, len(accs) > 0, "empty multi-signer")

	m := len(accs[0].Contract.Parameters)
	require.True(t, m <= len(accs), "honest not count is too big for a multi-signer")

	h := accs[0].Contract.ScriptHash()
	for i := 1; i < len(accs); i++ {
		require.Equal(t, m, len(accs[i].Contract.Parameters), "inconsistent multi-signer accounts")
		require.Equal(t, h, accs[i].Contract.ScriptHash(), "inconsistent multi-signer accounts")
	}
}
