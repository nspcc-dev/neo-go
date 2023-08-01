package nns_test

import (
	"strings"
	"testing"

	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newNSClient(t *testing.T) *neotest.ContractInvoker {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	c := neotest.CompileFile(t, e.CommitteeHash, ".", "nns.yml")
	e.DeployContract(t, c, nil)

	return e.CommitteeInvoker(c.Hash)
}

func TestNameService_Price(t *testing.T) {
	const (
		minPrice = int64(0)
		maxPrice = int64(10000_00000000)
	)

	c := newNSClient(t)

	t.Run("set, not signed by committee", func(t *testing.T) {
		acc := c.NewAccount(t)
		cAcc := c.WithSigners(acc)
		cAcc.InvokeFail(t, "not witnessed by committee", "setPrice", minPrice+1)
	})

	t.Run("get, default value", func(t *testing.T) {
		c.Invoke(t, defaultNameServiceDomainPrice, "getPrice")
	})

	t.Run("set, too small value", func(t *testing.T) {
		c.InvokeFail(t, "The price is out of range.", "setPrice", minPrice-1)
	})

	t.Run("set, too large value", func(t *testing.T) {
		c.InvokeFail(t, "The price is out of range.", "setPrice", maxPrice+1)
	})

	t.Run("set, success", func(t *testing.T) {
		txSet := c.PrepareInvoke(t, "setPrice", int64(defaultNameServiceDomainPrice+1))
		txGet := c.PrepareInvoke(t, "getPrice")
		c.AddBlockCheckHalt(t, txSet, txGet)
		c.CheckHalt(t, txSet.Hash(), stackitem.Null{})
		c.CheckHalt(t, txGet.Hash(), stackitem.Make(defaultNameServiceDomainPrice+1))

		// Get in the next block.
		c.Invoke(t, stackitem.Make(defaultNameServiceDomainPrice+1), "getPrice")
	})
}

func TestNonfungible(t *testing.T) {
	c := newNSClient(t)

	c.Signers = []neotest.Signer{c.NewAccount(t)}
	c.Invoke(t, "NNS", "symbol")
	c.Invoke(t, 0, "decimals")
	c.Invoke(t, 0, "totalSupply")
}

func TestAddRoot(t *testing.T) {
	c := newNSClient(t)

	t.Run("invalid format", func(t *testing.T) {
		c.InvokeFail(t, "invalid root format", "addRoot", "")
	})
	t.Run("not signed by committee", func(t *testing.T) {
		acc := c.NewAccount(t)
		c := c.WithSigners(acc)
		c.InvokeFail(t, "not witnessed by committee", "addRoot", "some")
	})

	c.Invoke(t, stackitem.Null{}, "addRoot", "some")
	t.Run("already exists", func(t *testing.T) {
		c.InvokeFail(t, "already exists", "addRoot", "some")
	})
}

func TestExpiration(t *testing.T) {
	c := newNSClient(t)
	e := c.Executor
	bc := e.Chain

	acc := e.NewAccount(t)
	cAcc := c.WithSigners(acc)

	c.Invoke(t, stackitem.Null{}, "addRoot", "com")
	cAcc.Invoke(t, true, "register", "first.com", acc.ScriptHash())
	cAcc.Invoke(t, stackitem.Null{}, "setRecord", "first.com", int64(nns.TXT), "sometext")
	b1 := e.TopBlock(t)

	tx := cAcc.PrepareInvoke(t, "register", "second.com", acc.ScriptHash())
	b2 := e.NewUnsignedBlock(t, tx)
	b2.Index = b1.Index + 1
	b2.PrevHash = b1.Hash()
	b2.Timestamp = b1.Timestamp + 10000
	require.NoError(t, bc.AddBlock(e.SignBlock(b2)))
	e.CheckHalt(t, tx.Hash())

	tx = cAcc.PrepareInvoke(t, "isAvailable", "first.com")
	b3 := e.NewUnsignedBlock(t, tx)
	b3.Index = b2.Index + 1
	b3.PrevHash = b2.Hash()
	b3.Timestamp = b1.Timestamp + (millisecondsInYear + 1)
	require.NoError(t, bc.AddBlock(e.SignBlock(b3)))
	e.CheckHalt(t, tx.Hash(), stackitem.NewBool(true))

	tx = cAcc.PrepareInvoke(t, "isAvailable", "second.com")
	b4 := e.NewUnsignedBlock(t, tx)
	b4.Index = b3.Index + 1
	b4.PrevHash = b3.Hash()
	b4.Timestamp = b3.Timestamp + 1000
	require.NoError(t, bc.AddBlock(e.SignBlock(b4)))
	e.CheckHalt(t, tx.Hash(), stackitem.NewBool(false))

	tx = cAcc.PrepareInvoke(t, "getRecord", "first.com", int64(nns.TXT))
	b5 := e.NewUnsignedBlock(t, tx)
	b5.Index = b4.Index + 1
	b5.PrevHash = b4.Hash()
	b5.Timestamp = b4.Timestamp + 1000
	require.NoError(t, bc.AddBlock(e.SignBlock(b5)))
	e.CheckFault(t, tx.Hash(), "name has expired")

	cAcc.Invoke(t, true, "register", "first.com", acc.ScriptHash()) // Re-register.
	cAcc.Invoke(t, stackitem.Null{}, "resolve", "first.com", int64(nns.TXT))
}

const (
	millisecondsInYear          = 365 * 24 * 3600 * 1000
	maxDomainNameFragmentLength = 63
)

func TestRegisterAndRenew(t *testing.T) {
	c := newNSClient(t)
	e := c.Executor

	c.InvokeFail(t, "root not found", "isAvailable", "neo.com")
	c.Invoke(t, stackitem.Null{}, "addRoot", "org")
	c.InvokeFail(t, "root not found", "isAvailable", "neo.com")
	c.Invoke(t, stackitem.Null{}, "addRoot", "com")
	c.Invoke(t, true, "isAvailable", "neo.com")
	c.InvokeWithFeeFail(t, "GAS limit exceeded", defaultNameServiceSysfee, "register", "neo.org", e.CommitteeHash)
	c.InvokeFail(t, "invalid domain name format", "register", "docs.neo.org", e.CommitteeHash)
	c.InvokeFail(t, "invalid domain name format", "register", "\nneo.com'", e.CommitteeHash)
	c.InvokeFail(t, "invalid domain name format", "register", "neo.com\n", e.CommitteeHash)
	c.InvokeWithFeeFail(t, "GAS limit exceeded", defaultNameServiceSysfee, "register", "neo.org", e.CommitteeHash)
	c.InvokeWithFeeFail(t, "GAS limit exceeded", defaultNameServiceDomainPrice, "register", "neo.com", e.CommitteeHash)
	var maxLenFragment string
	for i := 0; i < maxDomainNameFragmentLength; i++ {
		maxLenFragment += "q"
	}
	c.Invoke(t, true, "isAvailable", maxLenFragment+".com")
	c.Invoke(t, true, "register", maxLenFragment+".com", e.CommitteeHash)
	c.InvokeFail(t, "invalid domain name format", "register", maxLenFragment+"q.com", e.CommitteeHash)

	c.Invoke(t, true, "isAvailable", "neo.com")
	c.Invoke(t, 1, "balanceOf", e.CommitteeHash)
	c.Invoke(t, true, "register", "neo.com", e.CommitteeHash)
	topBlock := e.TopBlock(t)
	expectedExpiration := topBlock.Timestamp + millisecondsInYear
	c.Invoke(t, false, "register", "neo.com", e.CommitteeHash)
	c.Invoke(t, false, "isAvailable", "neo.com")

	t.Run("domain names with hyphen", func(t *testing.T) {
		c.InvokeFail(t, "invalid domain name format", "register", "-testdomain.com", e.CommitteeHash)
		c.InvokeFail(t, "invalid domain name format", "register", "testdomain-.com", e.CommitteeHash)
		c.Invoke(t, true, "register", "test-domain.com", e.CommitteeHash)
	})

	props := stackitem.NewMap()
	props.Add(stackitem.Make("name"), stackitem.Make("neo.com"))
	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	props.Add(stackitem.Make("admin"), stackitem.Null{}) // no admin was set
	c.Invoke(t, props, "properties", "neo.com")
	c.Invoke(t, 3, "balanceOf", e.CommitteeHash)
	c.Invoke(t, e.CommitteeHash.BytesBE(), "ownerOf", []byte("neo.com"))

	t.Run("invalid token ID", func(t *testing.T) {
		c.InvokeFail(t, "token not found", "properties", "not.exists")
		c.InvokeFail(t, "token not found", "ownerOf", "not.exists")
		c.InvokeFail(t, "invalid conversion", "properties", []any{})
		c.InvokeFail(t, "invalid conversion", "ownerOf", []any{})
	})

	// Renew
	expectedExpiration += millisecondsInYear
	c.Invoke(t, expectedExpiration, "renew", "neo.com")

	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	c.Invoke(t, props, "properties", "neo.com")
}

func TestSetGetRecord(t *testing.T) {
	c := newNSClient(t)
	e := c.Executor

	acc := e.NewAccount(t)
	cAcc := c.WithSigners(acc)
	c.Invoke(t, stackitem.Null{}, "addRoot", "com")

	t.Run("set before register", func(t *testing.T) {
		c.InvokeFail(t, "token not found", "setRecord", "neo.com", int64(nns.TXT), "sometext")
	})
	c.Invoke(t, true, "register", "neo.com", e.CommitteeHash)
	t.Run("invalid parameters", func(t *testing.T) {
		c.InvokeFail(t, "unsupported record type", "setRecord", "neo.com", int64(0xFF), "1.2.3.4")
		c.InvokeFail(t, "invalid record", "setRecord", "neo.com", int64(nns.A), "not.an.ip.address")
	})
	t.Run("invalid witness", func(t *testing.T) {
		cAcc.InvokeFail(t, "not witnessed by admin", "setRecord", "neo.com", int64(nns.A), "1.2.3.4")
	})
	c.Invoke(t, stackitem.Null{}, "getRecord", "neo.com", int64(nns.A))
	c.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.A), "1.2.3.4")
	c.Invoke(t, "1.2.3.4", "getRecord", "neo.com", int64(nns.A))
	c.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.A), "1.2.3.4")
	c.Invoke(t, "1.2.3.4", "getRecord", "neo.com", int64(nns.A))
	c.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.AAAA), "2001:0201:1f1f:0000:0000:0100:11a0:11df")
	c.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.CNAME), "nspcc.ru")
	c.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.TXT), "sometext")

	// Delete record.
	t.Run("invalid witness", func(t *testing.T) {
		cAcc.InvokeFail(t, "not witnessed by admin", "deleteRecord", "neo.com", int64(nns.CNAME))
	})
	c.Invoke(t, "nspcc.ru", "getRecord", "neo.com", int64(nns.CNAME))
	c.Invoke(t, stackitem.Null{}, "deleteRecord", "neo.com", int64(nns.CNAME))
	c.Invoke(t, stackitem.Null{}, "getRecord", "neo.com", int64(nns.CNAME))
	c.Invoke(t, "1.2.3.4", "getRecord", "neo.com", int64(nns.A))

	t.Run("SetRecord_compatibility", func(t *testing.T) {
		// tests are got from the NNS C# implementation and changed accordingly to non-native implementation behavior
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
			args := []any{"neo.com", int64(testCase.Type), testCase.Name}
			t.Run(testCase.Name, func(t *testing.T) {
				if testCase.ShouldFail {
					c.InvokeFail(t, "", "setRecord", args...)
				} else {
					c.Invoke(t, stackitem.Null{}, "setRecord", args...)
				}
			})
		}
	})
}

func TestSetAdmin(t *testing.T) {
	c := newNSClient(t)
	e := c.Executor

	owner := e.NewAccount(t)
	cOwner := c.WithSigners(owner)
	admin := e.NewAccount(t)
	cAdmin := c.WithSigners(admin)
	guest := e.NewAccount(t)
	cGuest := c.WithSigners(guest)

	c.Invoke(t, stackitem.Null{}, "addRoot", "com")

	cOwner.Invoke(t, true, "register", "neo.com", owner.ScriptHash())
	expectedExpiration := e.TopBlock(t).Timestamp + millisecondsInYear
	cGuest.InvokeFail(t, "not witnessed", "setAdmin", "neo.com", admin.ScriptHash())

	// Must be witnessed by both owner and admin.
	cOwner.InvokeFail(t, "not witnessed by admin", "setAdmin", "neo.com", admin.ScriptHash())
	cAdmin.InvokeFail(t, "not witnessed by owner", "setAdmin", "neo.com", admin.ScriptHash())
	cc := c.WithSigners(owner, admin)
	cc.Invoke(t, stackitem.Null{}, "setAdmin", "neo.com", admin.ScriptHash())
	props := stackitem.NewMap()
	props.Add(stackitem.Make("name"), stackitem.Make("neo.com"))
	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	props.Add(stackitem.Make("admin"), stackitem.Make(admin.ScriptHash().BytesBE()))
	c.Invoke(t, props, "properties", "neo.com")

	t.Run("set and delete by admin", func(t *testing.T) {
		cAdmin.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.TXT), "sometext")
		cGuest.InvokeFail(t, "not witnessed by admin", "deleteRecord", "neo.com", int64(nns.TXT))
		cAdmin.Invoke(t, stackitem.Null{}, "deleteRecord", "neo.com", int64(nns.TXT))
	})

	t.Run("set admin to null", func(t *testing.T) {
		cAdmin.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.TXT), "sometext")
		cOwner.Invoke(t, stackitem.Null{}, "setAdmin", "neo.com", nil)
		cAdmin.InvokeFail(t, "not witnessed by admin", "deleteRecord", "neo.com", int64(nns.TXT))
	})
}

func TestTransfer(t *testing.T) {
	c := newNSClient(t)
	e := c.Executor

	from := e.NewAccount(t)
	cFrom := c.WithSigners(from)
	to := e.NewAccount(t)
	cTo := c.WithSigners(to)

	c.Invoke(t, stackitem.Null{}, "addRoot", "com")
	cFrom.Invoke(t, true, "register", "neo.com", from.ScriptHash())
	cFrom.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.A), "1.2.3.4")
	cFrom.InvokeFail(t, "token not found", "transfer", to.ScriptHash(), "not.exists", nil)
	c.Invoke(t, false, "transfer", to.ScriptHash(), "neo.com", nil)
	cFrom.Invoke(t, true, "transfer", to.ScriptHash(), "neo.com", nil)
	cFrom.Invoke(t, 1, "totalSupply")
	cFrom.Invoke(t, to.ScriptHash().BytesBE(), "ownerOf", "neo.com")

	// without onNEP11Transfer
	ctr := neotest.CompileSource(t, e.CommitteeHash,
		strings.NewReader(`package foo
			func Main() int { return 0 }`),
		&compiler.Options{Name: "foo"})
	e.DeployContract(t, ctr, nil)
	cTo.InvokeFail(t, "method not found", "transfer", ctr.Hash, []byte("neo.com"), nil)

	// with onNEP11Transfer
	ctr = neotest.CompileSource(t, e.CommitteeHash,
		strings.NewReader(`package foo
			import "github.com/nspcc-dev/neo-go/pkg/interop"
			func OnNEP11Payment(from interop.Hash160, amount int, token []byte, data any) {}`),
		&compiler.Options{Name: "foo"})
	e.DeployContract(t, ctr, nil)
	cTo.Invoke(t, true, "transfer", ctr.Hash, []byte("neo.com"), nil)
	cFrom.Invoke(t, 1, "totalSupply")
	cFrom.Invoke(t, ctr.Hash.BytesBE(), "ownerOf", []byte("neo.com"))
}

func TestTokensOf(t *testing.T) {
	c := newNSClient(t)
	e := c.Executor

	acc1 := e.NewAccount(t)
	cAcc1 := c.WithSigners(acc1)
	acc2 := e.NewAccount(t)
	cAcc2 := c.WithSigners(acc2)

	c.Invoke(t, stackitem.Null{}, "addRoot", "com")
	cAcc1.Invoke(t, true, "register", "neo.com", acc1.ScriptHash())
	cAcc2.Invoke(t, true, "register", "nspcc.com", acc2.ScriptHash())

	testTokensOf(t, c, [][]byte{[]byte("neo.com")}, acc1.ScriptHash().BytesBE())
	testTokensOf(t, c, [][]byte{[]byte("nspcc.com")}, acc2.ScriptHash().BytesBE())
	testTokensOf(t, c, [][]byte{[]byte("neo.com"), []byte("nspcc.com")})
	testTokensOf(t, c, [][]byte{}, util.Uint160{}.BytesBE()) // empty hash is a valid hash still
}

func testTokensOf(t *testing.T, c *neotest.ContractInvoker, result [][]byte, args ...any) {
	method := "tokensOf"
	if len(args) == 0 {
		method = "tokens"
	}
	s, err := c.TestInvoke(t, method, args...)
	if result == nil {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)
	iter := s.Pop().Interop().Value().(*storage.Iterator)
	arr := make([]stackitem.Item, 0, len(result))
	for i := range result {
		require.True(t, iter.Next())
		require.Equal(t, result[i], iter.Value().Value())
		arr = append(arr, stackitem.Make(result[i]))
	}
	require.False(t, iter.Next())
}

func TestResolve(t *testing.T) {
	c := newNSClient(t)
	e := c.Executor

	acc := e.NewAccount(t)
	cAcc := c.WithSigners(acc)

	c.Invoke(t, stackitem.Null{}, "addRoot", "com")
	cAcc.Invoke(t, true, "register", "neo.com", acc.ScriptHash())
	cAcc.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.A), "1.2.3.4")
	cAcc.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.CNAME), "alias.com")

	cAcc.Invoke(t, true, "register", "alias.com", acc.ScriptHash())
	cAcc.Invoke(t, stackitem.Null{}, "setRecord", "alias.com", int64(nns.TXT), "sometxt")

	c.Invoke(t, "1.2.3.4", "resolve", "neo.com", int64(nns.A))
	c.Invoke(t, "alias.com", "resolve", "neo.com", int64(nns.CNAME))
	c.Invoke(t, "sometxt", "resolve", "neo.com", int64(nns.TXT))
	c.Invoke(t, "sometxt", "resolve", "neo.com.", int64(nns.TXT))
	c.InvokeFail(t, "invalid domain name format", "resolve", "neo.com..", int64(nns.TXT))
	c.Invoke(t, stackitem.Null{}, "resolve", "neo.com", int64(nns.AAAA))
}

const (
	defaultNameServiceDomainPrice = 10_0000_0000
	defaultNameServiceSysfee      = 6000_0000
)
