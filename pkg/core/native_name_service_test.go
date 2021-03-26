package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestNameService_Price(t *testing.T) {
	bc := newTestChain(t)

	testGetSet(t, bc, bc.contracts.NameService.Hash, "Price",
		native.DefaultDomainPrice, 1, 10000_00000000)
}

func TestNonfungible(t *testing.T) {
	bc := newTestChain(t)

	acc := newAccountWithGAS(t, bc)
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc, "symbol", "NNS")
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc, "decimals", 0)
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc, "totalSupply", 0)
}

func TestAddRoot(t *testing.T) {
	bc := newTestChain(t)

	transferFundsToCommittee(t, bc)
	nsHash := bc.contracts.NameService.Hash

	t.Run("invalid format", func(t *testing.T) {
		testNameServiceInvoke(t, bc, "addRoot", nil, "")
	})
	t.Run("not signed by committee", func(t *testing.T) {
		aer, err := invokeContractMethod(bc, 1000_0000, nsHash, "addRoot", "some")
		require.NoError(t, err)
		checkFAULTState(t, aer)
	})

	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "some")
	t.Run("already exists", func(t *testing.T) {
		testNameServiceInvoke(t, bc, "addRoot", nil, "some")
	})
}

func TestExpiration(t *testing.T) {
	bc := newTestChain(t)

	transferFundsToCommittee(t, bc)
	acc := newAccountWithGAS(t, bc)

	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, acc, "register",
		true, "first.com", acc.Contract.ScriptHash())

	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc,
		"setRecord", stackitem.Null{}, "first.com", int64(native.RecordTypeTXT), "sometext")
	b1 := bc.topBlock.Load().(*block.Block)

	tx, err := prepareContractMethodInvokeGeneric(bc, defaultRegisterSysfee, bc.contracts.NameService.Hash,
		"register", acc, "second.com", acc.Contract.ScriptHash())
	require.NoError(t, err)
	b2 := newBlockCustom(bc.GetConfig(), func(b *block.Block) {
		b.Index = b1.Index + 1
		b.PrevHash = b1.Hash()
		b.Timestamp = b1.Timestamp + 10000
	}, tx)
	require.NoError(t, bc.AddBlock(b2))
	checkTxHalt(t, bc, tx.Hash())

	tx, err = prepareContractMethodInvokeGeneric(bc, defaultNameServiceSysfee, bc.contracts.NameService.Hash,
		"isAvailable", acc, "first.com")
	require.NoError(t, err)
	b3 := newBlockCustom(bc.GetConfig(), func(b *block.Block) {
		b.Index = b2.Index + 1
		b.PrevHash = b2.Hash()
		b.Timestamp = b1.Timestamp + (secondsInYear+1)*1000
	}, tx)
	require.NoError(t, bc.AddBlock(b3))
	aer, err := bc.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkResult(t, &aer[0], stackitem.NewBool(true))

	tx, err = prepareContractMethodInvokeGeneric(bc, defaultNameServiceSysfee, bc.contracts.NameService.Hash,
		"isAvailable", acc, "second.com")
	require.NoError(t, err)
	b4 := newBlockCustom(bc.GetConfig(), func(b *block.Block) {
		b.Index = b3.Index + 1
		b.PrevHash = b3.Hash()
		b.Timestamp = b3.Timestamp + 1000
	}, tx)
	require.NoError(t, bc.AddBlock(b4))
	aer, err = bc.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkResult(t, &aer[0], stackitem.NewBool(false))

	tx, err = prepareContractMethodInvokeGeneric(bc, defaultNameServiceSysfee, bc.contracts.NameService.Hash,
		"getRecord", acc, "first.com", int64(native.RecordTypeTXT))
	require.NoError(t, err)
	b5 := newBlockCustom(bc.GetConfig(), func(b *block.Block) {
		b.Index = b4.Index + 1
		b.PrevHash = b4.Hash()
		b.Timestamp = b4.Timestamp + 1000
	}, tx)
	require.NoError(t, bc.AddBlock(b5))
	aer, err = bc.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkResult(t, &aer[0], stackitem.Null{})

}

const secondsInYear = 365 * 24 * 3600

func TestRegisterAndRenew(t *testing.T) {
	bc := newTestChain(t)

	transferFundsToCommittee(t, bc)

	testNameServiceInvoke(t, bc, "isAvailable", nil, "neo.com")
	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "org")
	testNameServiceInvoke(t, bc, "isAvailable", nil, "neo.com")
	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvoke(t, bc, "isAvailable", true, "neo.com")
	testNameServiceInvoke(t, bc, "register", nil, "neo.org", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, "register", nil, "docs.neo.org", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, "register", nil, "\nneo.com'", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, "register", nil, "neo.com\n", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, "register", nil, "neo.com", testchain.CommitteeScriptHash())
	testNameServiceInvokeAux(t, bc, native.DefaultDomainPrice, true, "register",
		nil, "neo.com", testchain.CommitteeScriptHash())

	testNameServiceInvoke(t, bc, "isAvailable", true, "neo.com")
	testNameServiceInvoke(t, bc, "balanceOf", 0, testchain.CommitteeScriptHash())
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, true, "register",
		true, "neo.com", testchain.CommitteeScriptHash())
	topBlock := bc.topBlock.Load().(*block.Block)
	expectedExpiration := topBlock.Timestamp/1000 + secondsInYear
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, true, "register",
		false, "neo.com", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, "isAvailable", false, "neo.com")

	props := stackitem.NewMap()
	props.Add(stackitem.Make("name"), stackitem.Make("neo.com"))
	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	testNameServiceInvoke(t, bc, "properties", props, "neo.com")
	testNameServiceInvoke(t, bc, "balanceOf", 1, testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, "ownerOf", testchain.CommitteeScriptHash().BytesBE(), []byte("neo.com"))

	t.Run("invalid token ID", func(t *testing.T) {
		testNameServiceInvoke(t, bc, "properties", nil, "not.exists")
		testNameServiceInvoke(t, bc, "ownerOf", nil, "not.exists")
		testNameServiceInvoke(t, bc, "properties", nil, []interface{}{})
		testNameServiceInvoke(t, bc, "ownerOf", nil, []interface{}{})
	})

	// Renew
	expectedExpiration += secondsInYear
	testNameServiceInvokeAux(t, bc, 100_0000_0000, true, "renew", expectedExpiration, "neo.com")

	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	testNameServiceInvoke(t, bc, "properties", props, "neo.com")
}

func TestSetGetRecord(t *testing.T) {
	bc := newTestChain(t)

	transferFundsToCommittee(t, bc)
	acc := newAccountWithGAS(t, bc)
	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "com")

	t.Run("set before register", func(t *testing.T) {
		testNameServiceInvoke(t, bc, "setRecord", nil, "neo.com", int64(native.RecordTypeTXT), "sometext")
	})
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, true, "register",
		true, "neo.com", testchain.CommitteeScriptHash())
	t.Run("invalid parameters", func(t *testing.T) {
		testNameServiceInvoke(t, bc, "setRecord", nil, "neo.com", int64(0xFF), "1.2.3.4")
		testNameServiceInvoke(t, bc, "setRecord", nil, "neo.com", int64(native.RecordTypeA), "not.an.ip.address")
	})
	t.Run("invalid witness", func(t *testing.T) {
		testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc, "setRecord", nil,
			"neo.com", int64(native.RecordTypeA), "1.2.3.4")
	})
	testNameServiceInvoke(t, bc, "getRecord", stackitem.Null{}, "neo.com", int64(native.RecordTypeA))
	testNameServiceInvoke(t, bc, "setRecord", stackitem.Null{}, "neo.com", int64(native.RecordTypeA), "1.2.3.4")
	testNameServiceInvoke(t, bc, "getRecord", "1.2.3.4", "neo.com", int64(native.RecordTypeA))
	testNameServiceInvoke(t, bc, "setRecord", stackitem.Null{}, "neo.com", int64(native.RecordTypeA), "1.2.3.4")
	testNameServiceInvoke(t, bc, "getRecord", "1.2.3.4", "neo.com", int64(native.RecordTypeA))
	testNameServiceInvoke(t, bc, "setRecord", stackitem.Null{}, "neo.com", int64(native.RecordTypeAAAA), "2001:0000:1f1f:0000:0000:0100:11a0:addf")
	testNameServiceInvoke(t, bc, "setRecord", stackitem.Null{}, "neo.com", int64(native.RecordTypeCNAME), "nspcc.ru")
	testNameServiceInvoke(t, bc, "setRecord", stackitem.Null{}, "neo.com", int64(native.RecordTypeTXT), "sometext")

	// Delete record.
	t.Run("invalid witness", func(t *testing.T) {
		testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc, "setRecord", nil,
			"neo.com", int64(native.RecordTypeCNAME))
	})
	testNameServiceInvoke(t, bc, "getRecord", "nspcc.ru", "neo.com", int64(native.RecordTypeCNAME))
	testNameServiceInvoke(t, bc, "deleteRecord", stackitem.Null{}, "neo.com", int64(native.RecordTypeCNAME))
	testNameServiceInvoke(t, bc, "getRecord", stackitem.Null{}, "neo.com", int64(native.RecordTypeCNAME))
	testNameServiceInvoke(t, bc, "getRecord", "1.2.3.4", "neo.com", int64(native.RecordTypeA))
}

func TestSetAdmin(t *testing.T) {
	bc := newTestChain(t)

	transferFundsToCommittee(t, bc)
	owner := newAccountWithGAS(t, bc)
	admin := newAccountWithGAS(t, bc)
	guest := newAccountWithGAS(t, bc)
	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "com")

	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, owner, "register", true,
		"neo.com", owner.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, guest, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())

	// Must be witnessed by both owner and admin.
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, owner, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, admin, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, []*wallet.Account{owner, admin},
		"setAdmin", stackitem.Null{},
		"neo.com", admin.PrivateKey().GetScriptHash())

	t.Run("set and delete by admin", func(t *testing.T) {
		testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, admin, "setRecord", stackitem.Null{},
			"neo.com", int64(native.RecordTypeTXT), "sometext")
		testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, guest, "deleteRecord", nil,
			"neo.com", int64(native.RecordTypeTXT))
		testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, admin, "deleteRecord", stackitem.Null{},
			"neo.com", int64(native.RecordTypeTXT))
	})

	t.Run("set admin to null", func(t *testing.T) {
		testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, admin, "setRecord", stackitem.Null{},
			"neo.com", int64(native.RecordTypeTXT), "sometext")
		testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, owner, "setAdmin", stackitem.Null{},
			"neo.com", nil)
		testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, admin, "deleteRecord", nil,
			"neo.com", int64(native.RecordTypeTXT))
	})
}

func TestTransfer(t *testing.T) {
	bc := newTestChain(t)

	transferFundsToCommittee(t, bc)
	from := newAccountWithGAS(t, bc)
	to := newAccountWithGAS(t, bc)

	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, from, "register",
		true, "neo.com", from.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, from, "setRecord", stackitem.Null{},
		"neo.com", int64(native.RecordTypeA), "1.2.3.4")
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, from, "transfer",
		nil, to.Contract.ScriptHash().BytesBE(), []byte("not.exists"))
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, true, "transfer",
		false, to.Contract.ScriptHash().BytesBE(), []byte("neo.com"))
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, from, "transfer",
		true, to.Contract.ScriptHash().BytesBE(), []byte("neo.com"))
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, from, "totalSupply", 1)
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, from, "ownerOf",
		to.Contract.ScriptHash().BytesBE(), []byte("neo.com"))
	cs, cs2 := getTestContractState(bc) // cs2 doesn't have OnNEP11Transfer
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs2))
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, to, "transfer",
		nil, cs2.Hash.BytesBE(), []byte("neo.com"))
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, to, "transfer",
		true, cs.Hash.BytesBE(), []byte("neo.com"))
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, from, "totalSupply", 1)
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, from, "ownerOf",
		cs.Hash.BytesBE(), []byte("neo.com"))
}

func TestTokensOf(t *testing.T) {
	bc := newTestChain(t)

	transferFundsToCommittee(t, bc)
	acc1 := newAccountWithGAS(t, bc)
	acc2 := newAccountWithGAS(t, bc)

	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, acc1, "register",
		true, "neo.com", acc1.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, acc2, "register",
		true, "nspcc.com", acc2.PrivateKey().GetScriptHash())

	testTokensOf(t, bc, acc1, [][]byte{[]byte("neo.com")}, acc1.Contract.ScriptHash().BytesBE())
	testTokensOf(t, bc, acc1, [][]byte{[]byte("nspcc.com")}, acc2.Contract.ScriptHash().BytesBE())
	testTokensOf(t, bc, acc1, [][]byte{[]byte("neo.com"), []byte("nspcc.com")})
	testTokensOf(t, bc, acc1, nil, util.Uint160{}.BytesBE())
}

func testTokensOf(t *testing.T, bc *Blockchain, signer *wallet.Account, result [][]byte, args ...interface{}) {
	method := "tokensOf"
	if len(args) == 0 {
		method = "tokens"
	}
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, bc.contracts.NameService.Hash, method, callflag.All, args...)
	for range result {
		emit.Opcodes(w.BinWriter, opcode.DUP)
		emit.Syscall(w.BinWriter, interopnames.SystemIteratorNext)
		emit.Opcodes(w.BinWriter, opcode.ASSERT)

		emit.Opcodes(w.BinWriter, opcode.DUP)
		emit.Syscall(w.BinWriter, interopnames.SystemIteratorValue)
		emit.Opcodes(w.BinWriter, opcode.SWAP)
	}
	emit.Opcodes(w.BinWriter, opcode.DROP)
	emit.Int(w.BinWriter, int64(len(result)))
	emit.Opcodes(w.BinWriter, opcode.PACK)
	require.NoError(t, w.Err)
	script := w.Bytes()
	tx := transaction.New(script, defaultNameServiceSysfee)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	signTxWithAccounts(bc, tx, signer)
	aers, err := persistBlock(bc, tx)
	require.NoError(t, err)
	if result == nil {
		checkFAULTState(t, aers[0])
		return
	}
	arr := make([]stackitem.Item, 0, len(result))
	for i := len(result) - 1; i >= 0; i-- {
		arr = append(arr, stackitem.Make(result[i]))
	}
	checkResult(t, aers[0], stackitem.NewArray(arr))
}

func TestResolve(t *testing.T) {
	bc := newTestChain(t)

	transferFundsToCommittee(t, bc)
	acc := newAccountWithGAS(t, bc)

	testNameServiceInvoke(t, bc, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, acc, "register",
		true, "neo.com", acc.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"neo.com", int64(native.RecordTypeA), "1.2.3.4")
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"neo.com", int64(native.RecordTypeCNAME), "alias.com")

	testNameServiceInvokeAux(t, bc, defaultRegisterSysfee, acc, "register",
		true, "alias.com", acc.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"alias.com", int64(native.RecordTypeTXT), "sometxt")

	testNameServiceInvoke(t, bc, "resolve", "1.2.3.4",
		"neo.com", int64(native.RecordTypeA))
	testNameServiceInvoke(t, bc, "resolve", "alias.com",
		"neo.com", int64(native.RecordTypeCNAME))
	testNameServiceInvoke(t, bc, "resolve", "sometxt",
		"neo.com", int64(native.RecordTypeTXT))
	testNameServiceInvoke(t, bc, "resolve", stackitem.Null{},
		"neo.com", int64(native.RecordTypeAAAA))
}

const (
	defaultNameServiceSysfee = 4000_0000
	defaultRegisterSysfee    = 10_0000_0000 + native.DefaultDomainPrice
)

func testNameServiceInvoke(t *testing.T, bc *Blockchain, method string, result interface{}, args ...interface{}) {
	testNameServiceInvokeAux(t, bc, defaultNameServiceSysfee, true, method, result, args...)
}

func testNameServiceInvokeAux(t *testing.T, bc *Blockchain, sysfee int64, signer interface{}, method string, result interface{}, args ...interface{}) {
	if sysfee < 0 {
		sysfee = defaultNameServiceSysfee
	}
	aer, err := invokeContractMethodGeneric(bc, sysfee, bc.contracts.NameService.Hash, method, signer, args...)
	require.NoError(t, err)
	if result == nil {
		checkFAULTState(t, aer)
		return
	}
	checkResult(t, aer, stackitem.Make(result))
}

func newAccountWithGAS(t *testing.T, bc *Blockchain) *wallet.Account {
	acc, err := wallet.NewAccount()
	require.NoError(t, err)
	transferTokenFromMultisigAccount(t, bc, acc.PrivateKey().GetScriptHash(), bc.contracts.GAS.Hash, 1000_00000000)
	return acc
}
