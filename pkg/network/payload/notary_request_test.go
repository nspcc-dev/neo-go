package payload

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestNotaryRequestIsValid(t *testing.T) {
	mainTx := &transaction.Transaction{
		Attributes:      []transaction.Attribute{{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 1}}},
		Script:          []byte{0, 1, 2},
		ValidUntilBlock: 123,
	}
	errorCases := map[string]*P2PNotaryRequest{
		"main tx: missing NotaryAssisted attribute": {MainTransaction: &transaction.Transaction{}},
		"main tx: zero NKeys":                       {MainTransaction: &transaction.Transaction{Attributes: []transaction.Attribute{{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 0}}}}},
		"fallback transaction: invalid signers count": {
			MainTransaction:     mainTx,
			FallbackTransaction: &transaction.Transaction{Signers: []transaction.Signer{{Account: random.Uint160()}}},
		},
		"fallback transaction: invalid witnesses count": {
			MainTransaction:     mainTx,
			FallbackTransaction: &transaction.Transaction{Signers: []transaction.Signer{{Account: random.Uint160()}}},
		},
		"fallback tx: invalid dummy Notary witness (bad witnesses length)": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{}},
			},
		},
		"fallback tx: invalid dummy Notary witness (bad invocation script length)": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{}, {}},
			},
		},
		"fallback tx: invalid dummy Notary witness (bad invocation script prefix)": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 65}, make([]byte, 64, 64)...)}, {}},
			},
		},
		"fallback tx: invalid dummy Notary witness (non-empty verification script))": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 1, 1)}, {}},
			},
		},
		"fallback tx: missing NotValidBefore attribute": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 0)}, {}},
			},
		},
		"fallback tx: invalid number of Conflicts attributes": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Attributes: []transaction.Attribute{{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}}},
				Signers:    []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts:    []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 0)}, {}},
			},
		},
		"fallback tx: does not conflicts with main tx": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Attributes: []transaction.Attribute{
					{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}},
					{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: util.Uint256{}}},
				},
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 0)}, {}},
			},
		},
		"fallback tx: missing NotaryAssisted attribute": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Attributes: []transaction.Attribute{
					{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}},
					{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: mainTx.Hash()}},
				},
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 0)}, {}},
			},
		},
		"fallback tx: non-zero NKeys": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				Attributes: []transaction.Attribute{
					{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}},
					{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: mainTx.Hash()}},
					{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 1}},
				},
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 0)}, {}},
			},
		},
		"fallback tx: ValidUntilBlock mismatch": {
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				ValidUntilBlock: 321,
				Attributes: []transaction.Attribute{
					{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}},
					{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: mainTx.Hash()}},
					{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 0}},
				},
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 0)}, {}},
			},
		},
	}
	for name, errCase := range errorCases {
		t.Run(name, func(t *testing.T) {
			require.Error(t, errCase.isValid())
		})
	}
	t.Run("good", func(t *testing.T) {
		p := &P2PNotaryRequest{
			MainTransaction: mainTx,
			FallbackTransaction: &transaction.Transaction{
				ValidUntilBlock: 123,
				Attributes: []transaction.Attribute{
					{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}},
					{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: mainTx.Hash()}},
					{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 0}},
				},
				Signers: []transaction.Signer{{Account: random.Uint160()}, {Account: random.Uint160()}},
				Scripts: []transaction.Witness{{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 0)}, {}},
			},
		}
		require.NoError(t, p.isValid())
	})
}

func TestNotaryRequestBytesFromBytes(t *testing.T) {
	mainTx := &transaction.Transaction{
		Attributes:      []transaction.Attribute{{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 1}}},
		Script:          []byte{0, 1, 2},
		ValidUntilBlock: 123,
		Signers:         []transaction.Signer{{Account: util.Uint160{1, 5, 9}}},
		Scripts: []transaction.Witness{{
			InvocationScript:   []byte{1, 4, 7},
			VerificationScript: []byte{3, 6, 9},
		}},
	}
	_ = mainTx.Hash()
	_ = mainTx.Size()
	fallbackTx := &transaction.Transaction{
		Script:          []byte{3, 2, 1},
		ValidUntilBlock: 123,
		Attributes: []transaction.Attribute{
			{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}},
			{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: mainTx.Hash()}},
			{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 0}},
		},
		Signers: []transaction.Signer{{Account: util.Uint160{1, 4, 7}}, {Account: util.Uint160{9, 8, 7}}},
		Scripts: []transaction.Witness{
			{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...), VerificationScript: make([]byte, 0)},
			{InvocationScript: []byte{1, 2, 3}, VerificationScript: []byte{1, 2, 3}}},
	}
	_ = fallbackTx.Hash()
	_ = fallbackTx.Size()
	p := &P2PNotaryRequest{
		Network:             netmode.UnitTestNet,
		MainTransaction:     mainTx,
		FallbackTransaction: fallbackTx,
		Witness: transaction.Witness{
			InvocationScript:   []byte{1, 2, 3},
			VerificationScript: []byte{7, 8, 9},
		},
	}

	_ = p.Hash() // initialize hash caches
	bytes, err := p.Bytes()
	require.NoError(t, err)
	actual, err := NewP2PNotaryRequestFromBytes(netmode.UnitTestNet, bytes)
	require.NoError(t, err)
	require.Equal(t, p, actual)
}
