package core

import (
	"testing"

	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
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
	bc, nsHash := newTestChainWithNS(t)

	testGetSet(t, bc, nsHash, "Price",
		defaultNameServiceDomainPrice, 0, 10000_00000000)
}

func TestNonfungible(t *testing.T) {
	bc, nsHash := newTestChainWithNS(t)

	acc := newAccountWithGAS(t, bc)
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc, "symbol", "NNS")
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc, "decimals", 0)
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc, "totalSupply", 0)
}

func TestAddRoot(t *testing.T) {
	bc, nsHash := newTestChainWithNS(t)

	transferFundsToCommittee(t, bc)

	t.Run("invalid format", func(t *testing.T) {
		testNameServiceInvoke(t, bc, nsHash, "addRoot", nil, "")
	})
	t.Run("not signed by committee", func(t *testing.T) {
		aer, err := invokeContractMethod(bc, 1000_0000, nsHash, "addRoot", "some")
		require.NoError(t, err)
		checkFAULTState(t, aer)
	})

	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "some")
	t.Run("already exists", func(t *testing.T) {
		testNameServiceInvoke(t, bc, nsHash, "addRoot", nil, "some")
	})
}

func TestExpiration(t *testing.T) {
	bc, nsHash := newTestChainWithNS(t)

	transferFundsToCommittee(t, bc)
	acc := newAccountWithGAS(t, bc)

	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, acc, "register",
		true, "first.com", acc.Contract.ScriptHash())

	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc,
		"setRecord", stackitem.Null{}, "first.com", int64(nns.TXT), "sometext")
	b1 := bc.topBlock.Load().(*block.Block)

	tx, err := prepareContractMethodInvokeGeneric(bc, defaultRegisterSysfee, nsHash,
		"register", acc, "second.com", acc.Contract.ScriptHash())
	require.NoError(t, err)
	b2 := newBlockCustom(bc.GetConfig(), func(b *block.Block) {
		b.Index = b1.Index + 1
		b.PrevHash = b1.Hash()
		b.Timestamp = b1.Timestamp + 10000
	}, tx)
	require.NoError(t, bc.AddBlock(b2))
	checkTxHalt(t, bc, tx.Hash())

	tx, err = prepareContractMethodInvokeGeneric(bc, defaultNameServiceSysfee, nsHash,
		"isAvailable", acc, "first.com")
	require.NoError(t, err)
	b3 := newBlockCustom(bc.GetConfig(), func(b *block.Block) {
		b.Index = b2.Index + 1
		b.PrevHash = b2.Hash()
		b.Timestamp = b1.Timestamp + (millisecondsInYear + 1)
	}, tx)
	require.NoError(t, bc.AddBlock(b3))
	aer, err := bc.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkResult(t, &aer[0], stackitem.NewBool(true))

	tx, err = prepareContractMethodInvokeGeneric(bc, defaultNameServiceSysfee, nsHash,
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

	tx, err = prepareContractMethodInvokeGeneric(bc, defaultNameServiceSysfee, nsHash,
		"getRecord", acc, "first.com", int64(nns.TXT))
	require.NoError(t, err)
	b5 := newBlockCustom(bc.GetConfig(), func(b *block.Block) {
		b.Index = b4.Index + 1
		b.PrevHash = b4.Hash()
		b.Timestamp = b4.Timestamp + 1000
	}, tx)
	require.NoError(t, bc.AddBlock(b5))
	aer, err = bc.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkFAULTState(t, &aer[0]) // name has expired (panic)
}

const millisecondsInYear = 365 * 24 * 3600 * 1000

func TestRegisterAndRenew(t *testing.T) {
	bc, nsHash := newTestChainWithNS(t)

	transferFundsToCommittee(t, bc)

	testNameServiceInvoke(t, bc, nsHash, "isAvailable", nil, "neo.com")
	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "org")
	testNameServiceInvoke(t, bc, nsHash, "isAvailable", nil, "neo.com")
	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvoke(t, bc, nsHash, "isAvailable", true, "neo.com")
	testNameServiceInvoke(t, bc, nsHash, "register", nil, "neo.org", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, nsHash, "register", nil, "docs.neo.org", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, nsHash, "register", nil, "\nneo.com'", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, nsHash, "register", nil, "neo.com\n", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, nsHash, "register", nil, "neo.com", testchain.CommitteeScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceDomainPrice, true, "register",
		nil, "neo.com", testchain.CommitteeScriptHash())

	testNameServiceInvoke(t, bc, nsHash, "isAvailable", true, "neo.com")
	testNameServiceInvoke(t, bc, nsHash, "balanceOf", 0, testchain.CommitteeScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, true, "register",
		true, "neo.com", testchain.CommitteeScriptHash())
	topBlock := bc.topBlock.Load().(*block.Block)
	expectedExpiration := topBlock.Timestamp + millisecondsInYear
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, true, "register",
		false, "neo.com", testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, nsHash, "isAvailable", false, "neo.com")

	props := stackitem.NewMap()
	props.Add(stackitem.Make("name"), stackitem.Make("neo.com"))
	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	testNameServiceInvoke(t, bc, nsHash, "properties", props, "neo.com")
	testNameServiceInvoke(t, bc, nsHash, "balanceOf", 1, testchain.CommitteeScriptHash())
	testNameServiceInvoke(t, bc, nsHash, "ownerOf", testchain.CommitteeScriptHash().BytesBE(), []byte("neo.com"))

	t.Run("invalid token ID", func(t *testing.T) {
		testNameServiceInvoke(t, bc, nsHash, "properties", nil, "not.exists")
		testNameServiceInvoke(t, bc, nsHash, "ownerOf", nil, "not.exists")
		testNameServiceInvoke(t, bc, nsHash, "properties", nil, []interface{}{})
		testNameServiceInvoke(t, bc, nsHash, "ownerOf", nil, []interface{}{})
	})

	// Renew
	expectedExpiration += millisecondsInYear
	testNameServiceInvokeAux(t, bc, nsHash, 100_0000_0000, true, "renew", expectedExpiration, "neo.com")

	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	testNameServiceInvoke(t, bc, nsHash, "properties", props, "neo.com")
}

func TestSetGetRecord(t *testing.T) {
	bc, nsHash := newTestChainWithNS(t)

	transferFundsToCommittee(t, bc)
	acc := newAccountWithGAS(t, bc)
	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "com")

	t.Run("set before register", func(t *testing.T) {
		testNameServiceInvoke(t, bc, nsHash, "setRecord", nil, "neo.com", int64(nns.TXT), "sometext")
	})
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, true, "register",
		true, "neo.com", testchain.CommitteeScriptHash())
	t.Run("invalid parameters", func(t *testing.T) {
		testNameServiceInvoke(t, bc, nsHash, "setRecord", nil, "neo.com", int64(0xFF), "1.2.3.4")
		testNameServiceInvoke(t, bc, nsHash, "setRecord", nil, "neo.com", int64(nns.A), "not.an.ip.address")
	})
	t.Run("invalid witness", func(t *testing.T) {
		testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc, "setRecord", nil,
			"neo.com", int64(nns.A), "1.2.3.4")
	})
	testNameServiceInvoke(t, bc, nsHash, "getRecord", stackitem.Null{}, "neo.com", int64(nns.A))
	testNameServiceInvoke(t, bc, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.A), "1.2.3.4")
	testNameServiceInvoke(t, bc, nsHash, "getRecord", "1.2.3.4", "neo.com", int64(nns.A))
	testNameServiceInvoke(t, bc, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.A), "1.2.3.4")
	testNameServiceInvoke(t, bc, nsHash, "getRecord", "1.2.3.4", "neo.com", int64(nns.A))
	testNameServiceInvoke(t, bc, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.AAAA), "2001:0201:1f1f:0000:0000:0100:11a0:11df")
	testNameServiceInvoke(t, bc, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.CNAME), "nspcc.ru")
	testNameServiceInvoke(t, bc, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.TXT), "sometext")

	// Delete record.
	t.Run("invalid witness", func(t *testing.T) {
		testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc, "setRecord", nil,
			"neo.com", int64(nns.CNAME))
	})
	testNameServiceInvoke(t, bc, nsHash, "getRecord", "nspcc.ru", "neo.com", int64(nns.CNAME))
	testNameServiceInvoke(t, bc, nsHash, "deleteRecord", stackitem.Null{}, "neo.com", int64(nns.CNAME))
	testNameServiceInvoke(t, bc, nsHash, "getRecord", stackitem.Null{}, "neo.com", int64(nns.CNAME))
	testNameServiceInvoke(t, bc, nsHash, "getRecord", "1.2.3.4", "neo.com", int64(nns.A))

	t.Run("SetRecord_compatibility", func(t *testing.T) {
		// tests are got from the NNS C# implementation and changed accordingly to non-native implementation behaviour
		testCases := []struct {
			Type       nns.RecordType
			Name       string
			ShouldFail bool
		}{
			{Type: nns.A, Name: "0.0.0.0", ShouldFail: true},
			{Type: nns.A, Name: "1.1.0.1"},
			{Type: nns.A, Name: "10.10.10.10", ShouldFail: true},
			{Type: nns.A, Name: "255.255.255.255", ShouldFail: true},
			{Type: nns.A, Name: "192.168.1.1", ShouldFail: true},
			{Type: nns.A, Name: "1a", ShouldFail: true},
			{Type: nns.A, Name: "256.0.0.0", ShouldFail: true},
			{Type: nns.A, Name: "01.01.01.01", ShouldFail: true},
			{Type: nns.A, Name: "00.0.0.0", ShouldFail: true},
			{Type: nns.A, Name: "0.0.0.-1", ShouldFail: true},
			{Type: nns.A, Name: "0.0.0.0.1", ShouldFail: true},
			{Type: nns.A, Name: "11111111.11111111.11111111.11111111", ShouldFail: true},
			{Type: nns.A, Name: "11111111.11111111.11111111.11111111", ShouldFail: true},
			{Type: nns.A, Name: "ff.ff.ff.ff", ShouldFail: true},
			{Type: nns.A, Name: "0.0.256", ShouldFail: true},
			{Type: nns.A, Name: "0.0.0", ShouldFail: true},
			{Type: nns.A, Name: "0.257", ShouldFail: true},
			{Type: nns.A, Name: "1.1", ShouldFail: true},
			{Type: nns.A, Name: "257", ShouldFail: true},
			{Type: nns.A, Name: "1", ShouldFail: true},
			// {2000} & {2001} & ]2002, 3ffe[ & {3fff} are valid values for IPv6 fragment0
			{Type: nns.AAAA, Name: "2002:db8::8:800:200c:417a", ShouldFail: true},
			{Type: nns.AAAA, Name: "3ffd:1b8::8:800:200c:417a"},
			{Type: nns.AAAA, Name: "3ffd::101"},
			{Type: nns.AAAA, Name: "2003::1"},
			{Type: nns.AAAA, Name: "2003::"},
			{Type: nns.AAAA, Name: "2002:db8:0:0:8:800:200c:417a", ShouldFail: true},
			{Type: nns.AAAA, Name: "3ffd:db8:0:0:8:800:200c:417a"},
			{Type: nns.AAAA, Name: "3ffd:0:0:0:0:0:0:101"},
			{Type: nns.AAAA, Name: "2002:0:0:0:0:0:0:101", ShouldFail: true},
			{Type: nns.AAAA, Name: "3ffd:0:0:0:0:0:0:101"},
			{Type: nns.AAAA, Name: "2001:200:0:0:0:0:0:1"},
			{Type: nns.AAAA, Name: "0:0:0:0:0:0:0:1", ShouldFail: true},
			{Type: nns.AAAA, Name: "2002:0:0:0:0:0:0:1", ShouldFail: true},
			{Type: nns.AAAA, Name: "2001:200:0:0:0:0:0:0"},
			{Type: nns.AAAA, Name: "2002:0:0:0:0:0:0:0", ShouldFail: true},
			{Type: nns.AAAA, Name: "2002:DB8::8:800:200C:417A", ShouldFail: true},
			{Type: nns.AAAA, Name: "3FFD:1B8::8:800:200C:417A"},
			{Type: nns.AAAA, Name: "3FFD::101"},
			{Type: nns.AAAA, Name: "3fFD::101"},
			{Type: nns.AAAA, Name: "2002:DB8:0:0:8:800:200C:417A", ShouldFail: true},
			{Type: nns.AAAA, Name: "3FFD:DB8:0:0:8:800:200C:417A"},
			{Type: nns.AAAA, Name: "3FFD:0:0:0:0:0:0:101"},
			{Type: nns.AAAA, Name: "3FFD::ffff:1.01.1.01", ShouldFail: true},
			{Type: nns.AAAA, Name: "2001:DB8:0:0:8:800:200C:4Z", ShouldFail: true},
			{Type: nns.AAAA, Name: "2001::13.1.68.3", ShouldFail: true},
		}
		for _, testCase := range testCases {
			var expected interface{}
			if testCase.ShouldFail {
				expected = nil
			} else {
				expected = stackitem.Null{}
			}
			t.Run(testCase.Name, func(t *testing.T) {
				testNameServiceInvoke(t, bc, nsHash, "setRecord", expected, "neo.com", int64(testCase.Type), testCase.Name)
			})
		}
	})
}

func TestSetAdmin(t *testing.T) {
	bc, nsHash := newTestChainWithNS(t)

	transferFundsToCommittee(t, bc)
	owner := newAccountWithGAS(t, bc)
	admin := newAccountWithGAS(t, bc)
	guest := newAccountWithGAS(t, bc)
	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "com")

	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, owner, "register", true,
		"neo.com", owner.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, guest, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())

	// Must be witnessed by both owner and admin.
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, owner, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, admin, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, []*wallet.Account{owner, admin},
		"setAdmin", stackitem.Null{},
		"neo.com", admin.PrivateKey().GetScriptHash())

	t.Run("set and delete by admin", func(t *testing.T) {
		testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, admin, "setRecord", stackitem.Null{},
			"neo.com", int64(nns.TXT), "sometext")
		testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, guest, "deleteRecord", nil,
			"neo.com", int64(nns.TXT))
		testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, admin, "deleteRecord", stackitem.Null{},
			"neo.com", int64(nns.TXT))
	})

	t.Run("set admin to null", func(t *testing.T) {
		testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, admin, "setRecord", stackitem.Null{},
			"neo.com", int64(nns.TXT), "sometext")
		testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, owner, "setAdmin", stackitem.Null{},
			"neo.com", nil)
		testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, admin, "deleteRecord", nil,
			"neo.com", int64(nns.TXT))
	})
}

func TestTransfer(t *testing.T) {
	bc, nsHash := newTestChainWithNS(t)

	transferFundsToCommittee(t, bc)
	from := newAccountWithGAS(t, bc)
	to := newAccountWithGAS(t, bc)

	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, from, "register",
		true, "neo.com", from.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, from, "setRecord", stackitem.Null{},
		"neo.com", int64(nns.A), "1.2.3.4")
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, from, "transfer",
		nil, to.Contract.ScriptHash().BytesBE(), []byte("not.exists"), nil)
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, true, "transfer",
		false, to.Contract.ScriptHash().BytesBE(), []byte("neo.com"), nil)
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, from, "transfer",
		true, to.Contract.ScriptHash().BytesBE(), []byte("neo.com"), nil)
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, from, "totalSupply", 1)
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, from, "ownerOf",
		to.Contract.ScriptHash().BytesBE(), []byte("neo.com"))
	cs, cs2 := getTestContractState(bc) // cs2 doesn't have OnNEP11Transfer
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs2))
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, to, "transfer",
		nil, cs2.Hash.BytesBE(), []byte("neo.com"), nil)
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, to, "transfer",
		true, cs.Hash.BytesBE(), []byte("neo.com"), nil)
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, from, "totalSupply", 1)
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, from, "ownerOf",
		cs.Hash.BytesBE(), []byte("neo.com"))
}

func TestTokensOf(t *testing.T) {
	bc, nsHash := newTestChainWithNS(t)

	transferFundsToCommittee(t, bc)
	acc1 := newAccountWithGAS(t, bc)
	acc2 := newAccountWithGAS(t, bc)

	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, acc1, "register",
		true, "neo.com", acc1.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, acc2, "register",
		true, "nspcc.com", acc2.PrivateKey().GetScriptHash())

	testTokensOf(t, bc, nsHash, acc1, [][]byte{[]byte("neo.com")}, acc1.Contract.ScriptHash().BytesBE())
	testTokensOf(t, bc, nsHash, acc1, [][]byte{[]byte("nspcc.com")}, acc2.Contract.ScriptHash().BytesBE())
	testTokensOf(t, bc, nsHash, acc1, [][]byte{[]byte("neo.com"), []byte("nspcc.com")})
	testTokensOf(t, bc, nsHash, acc1, [][]byte{}, util.Uint160{}.BytesBE()) // empty hash is a valid hash still
}

func testTokensOf(t *testing.T, bc *Blockchain, nsHash util.Uint160, signer *wallet.Account, result [][]byte, args ...interface{}) {
	method := "tokensOf"
	if len(args) == 0 {
		method = "tokens"
	}
	w := io.NewBufBinWriter()
	emit.AppCall(w, nsHash, method, callflag.All, args...)
	for range result {
		emit.Opcodes(w, opcode.DUP)
		emit.Syscall(w, interopnames.SystemIteratorNext)
		emit.Opcodes(w, opcode.ASSERT)

		emit.Opcodes(w, opcode.DUP)
		emit.Syscall(w, interopnames.SystemIteratorValue)
		emit.Opcodes(w, opcode.SWAP)
	}
	emit.Opcodes(w, opcode.DROP)
	emit.Int(w, int64(len(result)))
	emit.Opcodes(w, opcode.PACK)
	require.NoError(t, w.Error())
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
	bc, nsHash := newTestChainWithNS(t)

	transferFundsToCommittee(t, bc)
	acc := newAccountWithGAS(t, bc)

	testNameServiceInvoke(t, bc, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, acc, "register",
		true, "neo.com", acc.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"neo.com", int64(nns.A), "1.2.3.4")
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"neo.com", int64(nns.CNAME), "alias.com")

	testNameServiceInvokeAux(t, bc, nsHash, defaultRegisterSysfee, acc, "register",
		true, "alias.com", acc.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"alias.com", int64(nns.TXT), "sometxt")

	testNameServiceInvoke(t, bc, nsHash, "resolve", "1.2.3.4",
		"neo.com", int64(nns.A))
	testNameServiceInvoke(t, bc, nsHash, "resolve", "alias.com",
		"neo.com", int64(nns.CNAME))
	testNameServiceInvoke(t, bc, nsHash, "resolve", "sometxt",
		"neo.com", int64(nns.TXT))
	testNameServiceInvoke(t, bc, nsHash, "resolve", stackitem.Null{},
		"neo.com", int64(nns.AAAA))
}

const (
	defaultNameServiceDomainPrice = 10_0000_0000
	defaultNameServiceSysfee      = 4000_0000
	defaultRegisterSysfee         = 10_0000_0000 + defaultNameServiceDomainPrice
)

func testNameServiceInvoke(t *testing.T, bc *Blockchain, nsHash util.Uint160, method string, result interface{}, args ...interface{}) {
	testNameServiceInvokeAux(t, bc, nsHash, defaultNameServiceSysfee, true, method, result, args...)
}

func testNameServiceInvokeAux(t *testing.T, bc *Blockchain, nsHash util.Uint160, sysfee int64, signer interface{}, method string, result interface{}, args ...interface{}) {
	if sysfee < 0 {
		sysfee = defaultNameServiceSysfee
	}
	aer, err := invokeContractMethodGeneric(bc, sysfee, nsHash, method, signer, args...)
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
