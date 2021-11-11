package neotest

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

// Signer is a generic interface which can be either simple- or multi-signature signer.
type Signer interface {
	// ScriptHash returns signer script hash.
	Script() []byte
	// Script returns signer verification script.
	ScriptHash() util.Uint160
	// SignHashable returns invocation script for signing an item.
	SignHashable(uint32, hash.Hashable) []byte
	// SignTx signs a transaction.
	SignTx(netmode.Magic, *transaction.Transaction) error
}

// signer represents simple-signature signer.
type signer wallet.Account

// multiSigner represents single multi-signature signer consisting of provided accounts.
type multiSigner []*wallet.Account

// NewSingleSigner returns multi-signature signer for the provided account.
// It must contain exactly as many accounts as needed to sign the script.
func NewSingleSigner(acc *wallet.Account) Signer {
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
	return append([]byte{byte(opcode.PUSHDATA1), 64},
		(*wallet.Account)(s).PrivateKey().SignHashable(magic, item)...)
}

// SignTx implements Signer interface.
func (s *signer) SignTx(magic netmode.Magic, tx *transaction.Transaction) error {
	return (*wallet.Account)(s).SignTx(magic, tx)
}

// NewMultiSigner returns multi-signature signer for the provided account.
// It must contain at least as many accounts as needed to sign the script.
func NewMultiSigner(accs ...*wallet.Account) Signer {
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
	for _, acc := range accs {
		if !bytes.Equal(script, acc.Contract.Script) {
			panic("all accounts must have equal verification script")
		}
	}

	return multiSigner(accs[:m])
}

// ScriptHash implements Signer interface.
func (m multiSigner) ScriptHash() util.Uint160 {
	return m[0].Contract.ScriptHash()
}

// Script implements Signer interface.
func (m multiSigner) Script() []byte {
	return m[0].Contract.Script
}

// SignHashable implements Signer interface.
func (m multiSigner) SignHashable(magic uint32, item hash.Hashable) []byte {
	var script []byte
	for _, acc := range m {
		sign := acc.PrivateKey().SignHashable(magic, item)
		script = append(script, byte(opcode.PUSHDATA1), 64)
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

func checkMultiSigner(t *testing.T, s Signer) {
	accs, ok := s.(multiSigner)
	require.True(t, ok, "expected to be a multi-signer")
	require.True(t, len(accs) > 0, "empty multi-signer")

	m := len(accs[0].Contract.Parameters)
	require.True(t, m <= len(accs), "honest not count is too big for a multi-signer")

	h := accs[0].Contract.ScriptHash()
	for i := 1; i < len(accs); i++ {
		require.Equal(t, m, len(accs[i].Contract.Parameters), "inconsistent multi-signer accounts")
		require.Equal(t, h, accs[i].Contract.ScriptHash(), "inconsistent multi-signer accounts")
	}
}
