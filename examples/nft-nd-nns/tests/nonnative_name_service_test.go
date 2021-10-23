package tests

import (
	"strings"
	"testing"

	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func newExecutorWithNS(t *testing.T) (*neotest.Executor, util.Uint160) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc)
	c := neotest.CompileFile(t, e.CommitteeHash, "..", "../nns.yml")
	e.DeployContract(t, c, nil)

	h, err := e.Chain.GetContractScriptHash(1)
	require.NoError(t, err)
	require.Equal(t, c.Hash, h)
	return e, c.Hash
}

//
//func TestNameService_Price(t *testing.T) {
//	e, nsHash := newExecutorWithNS(t)
//
//	testGetSet(t, e.Chain, nsHash, "Price",
//		defaultNameServiceDomainPrice, 0, 10000_00000000)
//}

func TestNonfungible(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)

	acc := e.NewAccount(t)
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc, "symbol", "NNS")
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc, "decimals", 0)
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc, "totalSupply", 0)
}

func TestAddRoot(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)

	t.Run("invalid format", func(t *testing.T) {
		testNameServiceInvoke(t, e, nsHash, "addRoot", nil, "")
	})
	t.Run("not signed by committee", func(t *testing.T) {
		acc := e.NewAccount(t)
		tx := e.PrepareInvoke(t, acc, nsHash, "addRoot", "some")
		e.AddBlock(t, tx)
		e.CheckFault(t, tx.Hash(), "not witnessed by committee")
	})

	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "some")
	t.Run("already exists", func(t *testing.T) {
		testNameServiceInvoke(t, e, nsHash, "addRoot", nil, "some")
	})
}

func TestExpiration(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)
	bc := e.Chain

	acc := e.NewAccount(t)

	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, acc, "register",
		true, "first.com", acc.Contract.ScriptHash())

	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc,
		"setRecord", stackitem.Null{}, "first.com", int64(nns.TXT), "sometext")
	b1 := e.TopBlock(t)

	tx := e.PrepareInvoke(t, acc, nsHash, "register", "second.com", acc.Contract.ScriptHash())
	b2 := e.NewBlock(t, tx)
	b2.Index = b1.Index + 1
	b2.PrevHash = b1.Hash()
	b2.Timestamp = b1.Timestamp + 10000
	require.NoError(t, bc.AddBlock(e.SignBlock(b2)))
	e.CheckHalt(t, tx.Hash())

	tx = e.PrepareInvoke(t, acc, nsHash, "isAvailable", "first.com")
	b3 := e.NewBlock(t, tx)
	b3.Index = b2.Index + 1
	b3.PrevHash = b2.Hash()
	b3.Timestamp = b1.Timestamp + (millisecondsInYear + 1)
	require.NoError(t, bc.AddBlock(e.SignBlock(b3)))
	e.CheckHalt(t, tx.Hash(), stackitem.NewBool(true))

	tx = e.PrepareInvoke(t, acc, nsHash, "isAvailable", "second.com")
	b4 := e.NewBlock(t, tx)
	b4.Index = b3.Index + 1
	b4.PrevHash = b3.Hash()
	b4.Timestamp = b3.Timestamp + 1000
	require.NoError(t, bc.AddBlock(e.SignBlock(b4)))
	e.CheckHalt(t, tx.Hash(), stackitem.NewBool(false))

	tx = e.PrepareInvoke(t, acc, nsHash, "getRecord", "first.com", int64(nns.TXT))
	b5 := e.NewBlock(t, tx)
	b5.Index = b4.Index + 1
	b5.PrevHash = b4.Hash()
	b5.Timestamp = b4.Timestamp + 1000
	require.NoError(t, bc.AddBlock(e.SignBlock(b5)))
	e.CheckFault(t, tx.Hash(), "name has expired")
}

const millisecondsInYear = 365 * 24 * 3600 * 1000

func TestRegisterAndRenew(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)

	testNameServiceInvoke(t, e, nsHash, "isAvailable", nil, "neo.com")
	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "org")
	testNameServiceInvoke(t, e, nsHash, "isAvailable", nil, "neo.com")
	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvoke(t, e, nsHash, "isAvailable", true, "neo.com")
	testNameServiceInvoke(t, e, nsHash, "register", nil, "neo.org", e.CommitteeHash)
	testNameServiceInvoke(t, e, nsHash, "register", nil, "docs.neo.org", e.CommitteeHash)
	testNameServiceInvoke(t, e, nsHash, "register", nil, "\nneo.com'", e.CommitteeHash)
	testNameServiceInvoke(t, e, nsHash, "register", nil, "neo.com\n", e.CommitteeHash)
	testNameServiceInvoke(t, e, nsHash, "register", nil, "neo.com", e.CommitteeHash)
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceDomainPrice, e.Committee, "register",
		nil, "neo.com", e.CommitteeHash)

	testNameServiceInvoke(t, e, nsHash, "isAvailable", true, "neo.com")
	testNameServiceInvoke(t, e, nsHash, "balanceOf", 0, e.CommitteeHash)
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, e.Committee, "register",
		true, "neo.com", e.CommitteeHash)
	topBlock := e.TopBlock(t)
	expectedExpiration := topBlock.Timestamp + millisecondsInYear
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, e.Committee, "register",
		false, "neo.com", e.CommitteeHash)
	testNameServiceInvoke(t, e, nsHash, "isAvailable", false, "neo.com")

	props := stackitem.NewMap()
	props.Add(stackitem.Make("name"), stackitem.Make("neo.com"))
	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	testNameServiceInvoke(t, e, nsHash, "properties", props, "neo.com")
	testNameServiceInvoke(t, e, nsHash, "balanceOf", 1, e.CommitteeHash)
	testNameServiceInvoke(t, e, nsHash, "ownerOf", e.CommitteeHash.BytesBE(), []byte("neo.com"))

	t.Run("invalid token ID", func(t *testing.T) {
		testNameServiceInvoke(t, e, nsHash, "properties", nil, "not.exists")
		testNameServiceInvoke(t, e, nsHash, "ownerOf", nil, "not.exists")
		testNameServiceInvoke(t, e, nsHash, "properties", nil, []interface{}{})
		testNameServiceInvoke(t, e, nsHash, "ownerOf", nil, []interface{}{})
	})

	// Renew
	expectedExpiration += millisecondsInYear
	testNameServiceInvokeAux(t, e, nsHash, 100_0000_0000, e.Committee, "renew", expectedExpiration, "neo.com")

	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	testNameServiceInvoke(t, e, nsHash, "properties", props, "neo.com")
}

func TestSetGetRecord(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)

	acc := e.NewAccount(t)
	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "com")

	t.Run("set before register", func(t *testing.T) {
		testNameServiceInvoke(t, e, nsHash, "setRecord", nil, "neo.com", int64(nns.TXT), "sometext")
	})
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, e.Committee, "register",
		true, "neo.com", e.CommitteeHash)
	t.Run("invalid parameters", func(t *testing.T) {
		testNameServiceInvoke(t, e, nsHash, "setRecord", nil, "neo.com", int64(0xFF), "1.2.3.4")
		testNameServiceInvoke(t, e, nsHash, "setRecord", nil, "neo.com", int64(nns.A), "not.an.ip.address")
	})
	t.Run("invalid witness", func(t *testing.T) {
		testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc, "setRecord", nil,
			"neo.com", int64(nns.A), "1.2.3.4")
	})
	testNameServiceInvoke(t, e, nsHash, "getRecord", stackitem.Null{}, "neo.com", int64(nns.A))
	testNameServiceInvoke(t, e, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.A), "1.2.3.4")
	testNameServiceInvoke(t, e, nsHash, "getRecord", "1.2.3.4", "neo.com", int64(nns.A))
	testNameServiceInvoke(t, e, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.A), "1.2.3.4")
	testNameServiceInvoke(t, e, nsHash, "getRecord", "1.2.3.4", "neo.com", int64(nns.A))
	testNameServiceInvoke(t, e, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.AAAA), "2001:0201:1f1f:0000:0000:0100:11a0:11df")
	testNameServiceInvoke(t, e, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.CNAME), "nspcc.ru")
	testNameServiceInvoke(t, e, nsHash, "setRecord", stackitem.Null{}, "neo.com", int64(nns.TXT), "sometext")

	// Delete record.
	t.Run("invalid witness", func(t *testing.T) {
		testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc, "setRecord", nil,
			"neo.com", int64(nns.CNAME))
	})
	testNameServiceInvoke(t, e, nsHash, "getRecord", "nspcc.ru", "neo.com", int64(nns.CNAME))
	testNameServiceInvoke(t, e, nsHash, "deleteRecord", stackitem.Null{}, "neo.com", int64(nns.CNAME))
	testNameServiceInvoke(t, e, nsHash, "getRecord", stackitem.Null{}, "neo.com", int64(nns.CNAME))
	testNameServiceInvoke(t, e, nsHash, "getRecord", "1.2.3.4", "neo.com", int64(nns.A))

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
				testNameServiceInvoke(t, e, nsHash, "setRecord", expected, "neo.com", int64(testCase.Type), testCase.Name)
			})
		}
	})
}

func TestSetAdmin(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)

	owner := e.NewAccount(t)
	admin := e.NewAccount(t)
	guest := e.NewAccount(t)
	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "com")

	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, owner, "register", true,
		"neo.com", owner.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, guest, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())

	// Must be witnessed by both owner and admin.
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, owner, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, admin, "setAdmin", nil,
		"neo.com", admin.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, []*wallet.Account{owner, admin},
		"setAdmin", stackitem.Null{},
		"neo.com", admin.PrivateKey().GetScriptHash())

	t.Run("set and delete by admin", func(t *testing.T) {
		testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, admin, "setRecord", stackitem.Null{},
			"neo.com", int64(nns.TXT), "sometext")
		testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, guest, "deleteRecord", nil,
			"neo.com", int64(nns.TXT))
		testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, admin, "deleteRecord", stackitem.Null{},
			"neo.com", int64(nns.TXT))
	})

	t.Run("set admin to null", func(t *testing.T) {
		testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, admin, "setRecord", stackitem.Null{},
			"neo.com", int64(nns.TXT), "sometext")
		testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, owner, "setAdmin", stackitem.Null{},
			"neo.com", nil)
		testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, admin, "deleteRecord", nil,
			"neo.com", int64(nns.TXT))
	})
}

func TestTransfer(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)

	from := e.NewAccount(t)
	to := e.NewAccount(t)

	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, from, "register",
		true, "neo.com", from.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, from, "setRecord", stackitem.Null{},
		"neo.com", int64(nns.A), "1.2.3.4")
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, from, "transfer",
		nil, to.Contract.ScriptHash().BytesBE(), []byte("not.exists"), nil)
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, e.Committee, "transfer",
		false, to.Contract.ScriptHash().BytesBE(), []byte("neo.com"), nil)
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, from, "transfer",
		true, to.Contract.ScriptHash().BytesBE(), []byte("neo.com"), nil)
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, from, "totalSupply", 1)
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, from, "ownerOf",
		to.Contract.ScriptHash().BytesBE(), []byte("neo.com"))

	// without onNEP11Transfer
	c := neotest.CompileSource(t, e.CommitteeHash,
		strings.NewReader(`package foo
			func Main() int { return 0 }`),
		&compiler.Options{Name: "foo"})
	e.DeployContract(t, c, nil)
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, to, "transfer",
		nil, c.Hash.BytesBE(), []byte("neo.com"), nil)

	// with onNEP11Transfer
	c = neotest.CompileSource(t, e.CommitteeHash,
		strings.NewReader(`package foo
			import "github.com/nspcc-dev/neo-go/pkg/interop"
			func OnNEP11Payment(from interop.Hash160, amount int, token []byte, data interface{}) {}`),
		&compiler.Options{Name: "foo"})
	e.DeployContract(t, c, nil)
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, to, "transfer",
		true, c.Hash.BytesBE(), []byte("neo.com"), nil)
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, from, "totalSupply", 1)
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, from, "ownerOf",
		c.Hash.BytesBE(), []byte("neo.com"))
}

func TestTokensOf(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)

	acc1 := e.NewAccount(t)
	acc2 := e.NewAccount(t)

	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, acc1, "register",
		true, "neo.com", acc1.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, acc2, "register",
		true, "nspcc.com", acc2.PrivateKey().GetScriptHash())

	testTokensOf(t, e, nsHash, [][]byte{[]byte("neo.com")}, acc1.Contract.ScriptHash().BytesBE())
	testTokensOf(t, e, nsHash, [][]byte{[]byte("nspcc.com")}, acc2.Contract.ScriptHash().BytesBE())
	testTokensOf(t, e, nsHash, [][]byte{[]byte("neo.com"), []byte("nspcc.com")})
	testTokensOf(t, e, nsHash, [][]byte{}, util.Uint160{}.BytesBE()) // empty hash is a valid hash still
}

func testTokensOf(t *testing.T, e *neotest.Executor, nsHash util.Uint160, result [][]byte, args ...interface{}) {
	method := "tokensOf"
	if len(args) == 0 {
		method = "tokens"
	}
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, nsHash, method, callflag.All, args...)
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
	tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
	v, err := neotest.TestInvoke(e.Chain, tx)
	if result == nil {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)
	arr := make([]stackitem.Item, 0, len(result))
	for i := len(result) - 1; i >= 0; i-- {
		arr = append(arr, stackitem.Make(result[i]))
	}
	require.Equal(t, stackitem.NewArray(arr), v.Estack().Pop().Item())
}

func TestResolve(t *testing.T) {
	e, nsHash := newExecutorWithNS(t)

	acc := e.NewAccount(t)

	testNameServiceInvoke(t, e, nsHash, "addRoot", stackitem.Null{}, "com")
	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, acc, "register",
		true, "neo.com", acc.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"neo.com", int64(nns.A), "1.2.3.4")
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"neo.com", int64(nns.CNAME), "alias.com")

	testNameServiceInvokeAux(t, e, nsHash, defaultRegisterSysfee, acc, "register",
		true, "alias.com", acc.PrivateKey().GetScriptHash())
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, acc, "setRecord", stackitem.Null{},
		"alias.com", int64(nns.TXT), "sometxt")

	testNameServiceInvoke(t, e, nsHash, "resolve", "1.2.3.4",
		"neo.com", int64(nns.A))
	testNameServiceInvoke(t, e, nsHash, "resolve", "alias.com",
		"neo.com", int64(nns.CNAME))
	testNameServiceInvoke(t, e, nsHash, "resolve", "sometxt",
		"neo.com", int64(nns.TXT))
	testNameServiceInvoke(t, e, nsHash, "resolve", stackitem.Null{},
		"neo.com", int64(nns.AAAA))
}

const (
	defaultNameServiceDomainPrice = 10_0000_0000
	defaultNameServiceSysfee      = 6000_0000
	defaultRegisterSysfee         = 10_0000_0000 + defaultNameServiceDomainPrice
)

func testNameServiceInvoke(t *testing.T, e *neotest.Executor, nsHash util.Uint160, method string, result interface{}, args ...interface{}) {
	testNameServiceInvokeAux(t, e, nsHash, defaultNameServiceSysfee, e.Committee, method, result, args...)
}

func testNameServiceInvokeAux(t *testing.T, e *neotest.Executor, nsHash util.Uint160, sysfee int64, signer interface{}, method string, result interface{}, args ...interface{}) {
	if sysfee < 0 {
		sysfee = defaultNameServiceSysfee
	}
	tx := e.PrepareInvokeNoSign(t, nsHash, method, args...)
	e.SignTx(t, tx, sysfee, signer)
	e.AddBlock(t, tx)
	if result == nil {
		e.CheckFault(t, tx.Hash(), "")
	} else {
		e.CheckHalt(t, tx.Hash(), stackitem.Make(result))
	}
}
