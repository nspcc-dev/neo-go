package nns_test

import (
	"math/big"
	"strconv"
	"strings"
	"testing"

	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

const (
	millisecondsInYear          = 365 * 24 * 3600 * 1000
	maxDomainNameFragmentLength = 63
)

func newNSClient(t *testing.T, registerComTLD bool) *neotest.ContractInvoker {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	ctr := neotest.CompileFile(t, e.CommitteeHash, ".", "nns.yml")
	e.DeployContract(t, ctr, nil)

	c := e.CommitteeInvoker(ctr.Hash)
	if registerComTLD {
		// Set expiration big enough to pass all tests.
		mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)
		c.Invoke(t, true, "register", "com", c.CommitteeHash, mail, refresh, retry, expire, ttl)
	}
	return c
}

func TestNameService_Price(t *testing.T) {
	const (
		minPrice       = int64(-1)
		maxPrice       = int64(10000_00000000)
		defaultPrice   = 10_0000_0000
		committeePrice = -1
	)

	c := newNSClient(t, false)

	t.Run("set, not signed by committee", func(t *testing.T) {
		acc := c.NewAccount(t)
		cAcc := c.WithSigners(acc)
		cAcc.InvokeFail(t, "not witnessed by committee", "setPrice", minPrice+1)
	})
	t.Run("get, default value", func(t *testing.T) {
		c.Invoke(t, defaultPrice, "getPrice", 0)
		c.Invoke(t, committeePrice, "getPrice", 1)
		c.Invoke(t, committeePrice, "getPrice", 2)
		c.Invoke(t, committeePrice, "getPrice", 3)
		c.Invoke(t, committeePrice, "getPrice", 4)
		c.Invoke(t, defaultPrice, "getPrice", 5)
	})
	t.Run("set, too small value", func(t *testing.T) {
		c.InvokeFail(t, "price is out of range", "setPrice", []interface{}{minPrice - 1})
		c.InvokeFail(t, "price is out of range", "setPrice", []interface{}{defaultPrice, minPrice - 1})
	})
	t.Run("set, too large value", func(t *testing.T) {
		c.InvokeFail(t, "price is out of range", "setPrice", []interface{}{minPrice - 1})
		c.InvokeFail(t, "price is out of range", "setPrice", []interface{}{defaultPrice, minPrice - 1})
	})
	t.Run("set, negative default price", func(t *testing.T) {
		c.InvokeFail(t, "default price is out of range", "setPrice", []interface{}{committeePrice, minPrice + 1})
	})
	t.Run("set, success", func(t *testing.T) {
		txSet := c.PrepareInvoke(t, "setPrice", []interface{}{defaultPrice - 1, committeePrice, committeePrice, committeePrice, committeePrice, committeePrice})
		txGet1 := c.PrepareInvoke(t, "getPrice", 5)
		txGet2 := c.PrepareInvoke(t, "getPrice", 6)
		c.AddBlockCheckHalt(t, txSet, txGet1, txGet2)
		c.CheckHalt(t, txSet.Hash(), stackitem.Null{})
		c.CheckHalt(t, txGet1.Hash(), stackitem.Make(committeePrice))
		c.CheckHalt(t, txGet2.Hash(), stackitem.Make(defaultPrice-1))

		// Get in the next block.
		c.Invoke(t, stackitem.Make(committeePrice), "getPrice", 2)
		c.Invoke(t, stackitem.Make(committeePrice), "getPrice", 5)
		c.Invoke(t, stackitem.Make(defaultPrice-1), "getPrice", 6)
	})
}

func TestNonfungible(t *testing.T) {
	c := newNSClient(t, false)

	c.Signers = []neotest.Signer{c.NewAccount(t)}
	c.Invoke(t, "NNS", "symbol")
	c.Invoke(t, 0, "decimals")
	c.Invoke(t, 0, "totalSupply")
}

func TestRegisterTLD(t *testing.T) {
	c := newNSClient(t, false)
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	t.Run("invalid format", func(t *testing.T) {
		c.InvokeFail(t, "invalid domain name format", "register", "", c.CommitteeHash, mail, refresh, retry, expire, ttl)
	})
	t.Run("not signed by committee", func(t *testing.T) {
		acc := c.NewAccount(t)
		c := c.WithSigners(acc)
		c.InvokeFail(t, "not witnessed by committee", "register", "some", c.CommitteeHash, mail, refresh, retry, expire, ttl)
	})

	c.Invoke(t, true, "register", "some", c.CommitteeHash, mail, refresh, retry, expire, ttl)
	t.Run("already exists", func(t *testing.T) {
		c.InvokeFail(t, "TLD already exists", "register", "some", c.CommitteeHash, mail, refresh, retry, expire, ttl)
	})
}

func TestExpiration(t *testing.T) {
	c := newNSClient(t, true)
	e := c.Executor
	bc := e.Chain
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	acc := e.NewAccount(t)
	cAcc := c.WithSigners(acc)
	cAccCommittee := c.WithSigners(acc, c.Committee) // acc + committee signers for ".com"'s subdomains registration

	cAccCommittee.Invoke(t, true, "register", "first.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "first.com", int64(nns.TXT), "sometext")
	b1 := e.TopBlock(t)

	tx := cAccCommittee.PrepareInvoke(t, "register", "second.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	b2 := e.NewUnsignedBlock(t, tx)
	b2.Index = b1.Index + 1
	b2.PrevHash = b1.Hash()
	b2.Timestamp = b1.Timestamp + 10000
	require.NoError(t, bc.AddBlock(e.SignBlock(b2)))
	e.CheckHalt(t, tx.Hash(), stackitem.NewBool(true))

	b3 := e.NewUnsignedBlock(t)
	b3.Index = b2.Index + 1
	b3.PrevHash = b2.Hash()
	b3.Timestamp = b1.Timestamp + (uint64(expire) * 1000)
	require.NoError(t, bc.AddBlock(e.SignBlock(b3)))

	cAcc.Invoke(t, true, "isAvailable", "first.com")  // "first.com" has been expired
	cAcc.Invoke(t, true, "isAvailable", "second.com") // TLD "com" has been expired
	cAcc.InvokeFail(t, "name has expired", "getRecords", "first.com", int64(nns.TXT))

	// TODO: According to the new code, we can't re-register expired "com" TLD, because it's already registered; at the
	// same time we can't renew it because it's already expired. We likely need to change this logic in the contract and
	// after that uncomment the lines below.
	// c.Invoke(t, true, "renew", "com")
	// cAcc.Invoke(t, true, "register", "first.com", acc.ScriptHash()) // Re-register.
	// cAcc.Invoke(t, stackitem.Null{}, "resolve", "first.com", int64(nns.TXT))
}

func TestRegisterAndRenew(t *testing.T) {
	c := newNSClient(t, false)
	e := c.Executor
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*2), int64(104)

	c.InvokeFail(t, "TLD not found", "isAvailable", "neo-go.com")
	c.Invoke(t, true, "register", "org", c.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.InvokeFail(t, "TLD not found", "isAvailable", "neo-go.com")
	c.Invoke(t, true, "register", "com", c.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.Invoke(t, true, "isAvailable", "neo-go.com")
	c.InvokeWithFeeFail(t, "GAS limit exceeded", defaultNameServiceSysfee, "register", "neo-go.org", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.InvokeFail(t, "one of the parent domains is not registered", "register", "docs.neo-go.org", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.InvokeFail(t, "invalid domain name format", "register", "\nneo-go.com'", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.InvokeFail(t, "invalid domain name format", "register", "neo-go.com\n", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.InvokeWithFeeFail(t, "GAS limit exceeded", defaultNameServiceSysfee, "register", "neo-go.org", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.InvokeWithFeeFail(t, "GAS limit exceeded", defaultNameServiceDomainPrice, "register", "neo-go.com", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	var maxLenFragment string
	for i := 0; i < maxDomainNameFragmentLength; i++ {
		maxLenFragment += "q"
	}
	c.Invoke(t, true, "isAvailable", maxLenFragment+".com")
	c.Invoke(t, true, "register", maxLenFragment+".com", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.InvokeFail(t, "invalid domain name format", "register", maxLenFragment+"q.com", e.CommitteeHash, mail, refresh, retry, expire, ttl)

	c.Invoke(t, true, "isAvailable", "neo-go.com")
	c.Invoke(t, 3, "balanceOf", e.CommitteeHash) // org, com, qqq...qqq.com
	c.Invoke(t, true, "register", "neo-go.com", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	topBlock := e.TopBlock(t)
	expectedExpiration := topBlock.Timestamp + uint64(expire*1000)
	c.Invoke(t, false, "register", "neo-go.com", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	c.Invoke(t, false, "isAvailable", "neo-go.com")

	t.Run("domain names with hyphen", func(t *testing.T) {
		c.InvokeFail(t, "invalid domain name format", "register", "-testdomain.com", e.CommitteeHash, mail, refresh, retry, expire, ttl)
		c.InvokeFail(t, "invalid domain name format", "register", "testdomain-.com", e.CommitteeHash, mail, refresh, retry, expire, ttl)
		c.Invoke(t, true, "register", "test-domain.com", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	})

	props := stackitem.NewMap()
	props.Add(stackitem.Make("name"), stackitem.Make("neo-go.com"))
	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	props.Add(stackitem.Make("admin"), stackitem.Null{}) // no admin was set
	c.Invoke(t, props, "properties", "neo-go.com")
	c.Invoke(t, 5, "balanceOf", e.CommitteeHash) // org, com, qqq...qqq.com, neo.com, test-domain.com
	c.Invoke(t, e.CommitteeHash.BytesBE(), "ownerOf", []byte("neo-go.com"))

	t.Run("invalid token ID", func(t *testing.T) {
		c.InvokeFail(t, "token not found", "properties", "not.exists")
		c.InvokeFail(t, "token not found", "ownerOf", "not.exists")
		c.InvokeFail(t, "invalid conversion", "properties", []interface{}{})
		c.InvokeFail(t, "invalid conversion", "ownerOf", []interface{}{})
	})

	// Renew
	expectedExpiration += millisecondsInYear
	c.Invoke(t, expectedExpiration, "renew", "neo-go.com")

	props.Add(stackitem.Make("expiration"), stackitem.Make(expectedExpiration))
	c.Invoke(t, props, "properties", "neo-go.com")

	// Invalid renewal period.
	c.InvokeFail(t, "invalid renewal period value", "renew", "neo-go.com", 11)
	// Too large expiration period.
	c.InvokeFail(t, "10 years of expiration period at max is allowed", "renew", "neo-go.com", 10)

	// Non-default renewal period.
	mult := 2
	c.Invoke(t, expectedExpiration+uint64(mult*millisecondsInYear), "renew", "neo-go.com", mult)
}

func TestSetAddGetRecord(t *testing.T) {
	c := newNSClient(t, true)
	e := c.Executor
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	acc := e.NewAccount(t)
	cAcc := c.WithSigners(acc)

	t.Run("set before register", func(t *testing.T) {
		c.InvokeFail(t, "token not found", "addRecord", "neo.com", int64(nns.TXT), "sometext")
	})
	c.Invoke(t, true, "register", "neo.com", e.CommitteeHash, mail, refresh, retry, expire, ttl)
	t.Run("invalid parameters", func(t *testing.T) {
		c.InvokeFail(t, "unsupported record type", "addRecord", "neo.com", int64(0xFF), "1.2.3.4")
		c.InvokeFail(t, "invalid record", "addRecord", "neo.com", int64(nns.A), "not.an.ip.address")
	})
	t.Run("invalid witness", func(t *testing.T) {
		cAcc.InvokeFail(t, "not witnessed by admin", "addRecord", "neo.com", int64(nns.A), "1.2.3.4")
	})
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{}), "getRecords", "neo.com", int64(nns.A))
	c.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.A), "1.2.3.4")
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("1.2.3.4")}), "getRecords", "neo.com", int64(nns.A))
	c.InvokeFail(t, "record already exists", "addRecord", "neo.com", int64(nns.A), "1.2.3.4") // Duplicating record.
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("1.2.3.4")}), "getRecords", "neo.com", int64(nns.A))
	c.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.AAAA), "2001:0201:1f1f:0000:0000:0100:11a0:11df")
	c.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.CNAME), "nspcc.ru")
	c.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.TXT), "sometext")
	// Add multiple records and update some of them.
	c.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.TXT), "sometext1")
	c.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.TXT), "sometext2")
	c.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.TXT), "sometext3")
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{
		stackitem.Make("sometext"),
		stackitem.Make("sometext1"),
		stackitem.Make("sometext2"),
		stackitem.Make("sometext3"),
	}), "getRecords", "neo.com", int64(nns.TXT))
	c.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.TXT), 2, "sometext22")
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{
		stackitem.Make("sometext"),
		stackitem.Make("sometext1"),
		stackitem.Make("sometext22"),
		stackitem.Make("sometext3"),
	}), "getRecords", "neo.com", int64(nns.TXT))

	// Delete record.
	t.Run("invalid witness", func(t *testing.T) {
		cAcc.InvokeFail(t, "not witnessed by admin", "deleteRecords", "neo.com", int64(nns.CNAME))
	})
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("nspcc.ru")}), "getRecords", "neo.com", int64(nns.CNAME))
	c.Invoke(t, stackitem.Null{}, "deleteRecords", "neo.com", int64(nns.CNAME))
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{}), "getRecords", "neo.com", int64(nns.CNAME))
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("1.2.3.4")}), "getRecords", "neo.com", int64(nns.A))

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
			args := []interface{}{"neo.com", int64(testCase.Type), testCase.Name}
			t.Run(testCase.Name, func(t *testing.T) {
				if testCase.ShouldFail {
					c.InvokeFail(t, "", "addRecord", args...)
				} else {
					c.Invoke(t, stackitem.Null{}, "addRecord", args...)
					c.Invoke(t, stackitem.Null{}, "deleteRecords", "neo.com", int64(testCase.Type)) // clear records after test to avoid duplicating records.
				}
			})
		}
	})
}

func TestSetAdmin(t *testing.T) {
	c := newNSClient(t, true)
	e := c.Executor
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	owner := e.NewAccount(t)
	cOwner := c.WithSigners(owner)
	cOwnerCommittee := c.WithSigners(owner, c.Committee)
	admin := e.NewAccount(t)
	cAdmin := c.WithSigners(admin)
	guest := e.NewAccount(t)
	cGuest := c.WithSigners(guest)

	cOwner.InvokeFail(t, "not witnessed by admin", "register", "neo.com", owner.ScriptHash(), mail, refresh, retry, expire, ttl) // admin is committee
	cOwnerCommittee.Invoke(t, true, "register", "neo.com", owner.ScriptHash(), mail, refresh, retry, expire, ttl)
	expectedExpiration := e.TopBlock(t).Timestamp + uint64(expire)*1000
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
		cAdmin.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.TXT), "sometext")
		cGuest.InvokeFail(t, "not witnessed by admin", "deleteRecords", "neo.com", int64(nns.TXT))
		cAdmin.Invoke(t, stackitem.Null{}, "deleteRecords", "neo.com", int64(nns.TXT))
	})

	t.Run("set admin to null", func(t *testing.T) {
		cAdmin.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.TXT), "sometext")
		cOwner.Invoke(t, stackitem.Null{}, "setAdmin", "neo.com", nil)
		cAdmin.InvokeFail(t, "not witnessed by admin", "deleteRecords", "neo.com", int64(nns.TXT))
	})
}

func TestTransfer(t *testing.T) {
	c := newNSClient(t, true)
	e := c.Executor
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	from := e.NewAccount(t)
	cFrom := c.WithSigners(from)
	cFromCommittee := c.WithSigners(from, c.Committee)
	to := e.NewAccount(t)
	cTo := c.WithSigners(to)

	cFromCommittee.Invoke(t, true, "register", "neo.com", from.ScriptHash(), mail, refresh, retry, expire, ttl)
	cFrom.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.A), "1.2.3.4")
	cFrom.InvokeFail(t, "token not found", "transfer", to.ScriptHash(), "not.exists", nil)
	c.Invoke(t, false, "transfer", to.ScriptHash(), "neo.com", nil)
	cFrom.Invoke(t, true, "transfer", to.ScriptHash(), "neo.com", nil)
	cFrom.Invoke(t, 2, "totalSupply") // com, neo.com
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
			func OnNEP11Payment(from interop.Hash160, amount int, token []byte, data interface{}) {}`),
		&compiler.Options{Name: "foo"})
	e.DeployContract(t, ctr, nil)
	cTo.Invoke(t, true, "transfer", ctr.Hash, []byte("neo.com"), nil)
	cFrom.Invoke(t, 2, "totalSupply") // com, neo.com
	cFrom.Invoke(t, ctr.Hash.BytesBE(), "ownerOf", []byte("neo.com"))
}

func TestTokensOf(t *testing.T) {
	c := newNSClient(t, false)
	e := c.Executor
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	acc1 := e.NewAccount(t)
	cAcc1Committee := c.WithSigners(acc1, c.Committee)
	acc2 := e.NewAccount(t)
	cAcc2Committee := c.WithSigners(acc2, c.Committee)

	tld := []byte("com")
	c.Invoke(t, true, "register", tld, c.CommitteeHash, mail, refresh, retry, expire, ttl)
	cAcc1Committee.Invoke(t, true, "register", "neo.com", acc1.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc2Committee.Invoke(t, true, "register", "nspcc.com", acc2.ScriptHash(), mail, refresh, retry, expire, ttl)

	testTokensOf(t, c, tld, [][]byte{[]byte("neo.com")}, acc1.ScriptHash().BytesBE())
	testTokensOf(t, c, tld, [][]byte{[]byte("nspcc.com")}, acc2.ScriptHash().BytesBE())
	testTokensOf(t, c, tld, [][]byte{[]byte("neo.com"), []byte("nspcc.com")})
	testTokensOf(t, c, tld, [][]byte{}, util.Uint160{}.BytesBE()) // empty hash is a valid hash still
}

func testTokensOf(t *testing.T, c *neotest.ContractInvoker, tld []byte, result [][]byte, args ...interface{}) {
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
	if method == "tokens" {
		require.True(t, iter.Next())
		require.Equal(t, tld, iter.Value().Value())
	} else {
		require.False(t, iter.Next())
	}
}

func TestResolve(t *testing.T) {
	c := newNSClient(t, true)
	e := c.Executor
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	acc := e.NewAccount(t)
	cAcc := c.WithSigners(acc)
	cAccCommittee := c.WithSigners(acc, c.Committee)

	cAccCommittee.Invoke(t, true, "register", "neo.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.A), "1.2.3.4")
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.CNAME), "alias.com")

	cAccCommittee.Invoke(t, true, "register", "alias.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "alias.com", int64(nns.TXT), "sometxt from alias1")
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "alias.com", int64(nns.CNAME), "alias2.com")

	cAccCommittee.Invoke(t, true, "register", "alias2.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "alias2.com", int64(nns.TXT), "sometxt from alias2")

	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("1.2.3.4")}), "resolve", "neo.com", int64(nns.A))
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("1.2.3.4")}), "resolve", "neo.com.", int64(nns.A))
	c.InvokeFail(t, "invalid domain name format", "resolve", "neo.com..", int64(nns.A))

	// Check CNAME is properly resolved and is not included into the result.
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("sometxt from alias1"), stackitem.Make("sometxt from alias2")}), "resolve", "neo.com", int64(nns.TXT))
	// Check CNAME is included into the result and is not resolved.
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("alias.com")}), "resolve", "neo.com", int64(nns.CNAME))

	// Empty result.
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{}), "resolve", "neo.com", int64(nns.AAAA))
}

func TestGetAllRecords(t *testing.T) {
	c := newNSClient(t, true)
	e := c.Executor
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	acc := e.NewAccount(t)
	cAcc := c.WithSigners(acc)
	cAccCommittee := c.WithSigners(acc, c.Committee)

	cAccCommittee.Invoke(t, true, "register", "neo.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.A), "1.2.3.4")
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.CNAME), "alias.com")
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.TXT), "bla0")
	cAcc.Invoke(t, stackitem.Null{}, "setRecord", "neo.com", int64(nns.TXT), 0, "bla1") // overwrite
	time := e.TopBlock(t).Timestamp

	// Add some arbitrary data.
	cAccCommittee.Invoke(t, true, "register", "alias.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "alias.com", int64(nns.TXT), "sometxt")

	script, err := smartcontract.CreateCallAndUnwrapIteratorScript(c.Hash, "getAllRecords", 10, "neo.com")
	require.NoError(t, err)
	h := e.InvokeScript(t, script, []neotest.Signer{acc})
	e.CheckHalt(t, h, stackitem.NewArray([]stackitem.Item{
		stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray([]byte("neo.com")),
			stackitem.Make(nns.A),
			stackitem.NewByteArray([]byte("1.2.3.4")),
			stackitem.NewBigInteger(big.NewInt(0)),
		}),
		stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray([]byte("neo.com")),
			stackitem.Make(nns.CNAME),
			stackitem.NewByteArray([]byte("alias.com")),
			stackitem.NewBigInteger(big.NewInt(0)),
		}),
		stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray([]byte("neo.com")),
			stackitem.Make(nns.SOA),
			stackitem.NewBuffer([]byte("neo.com" + " " + mail + " " +
				strconv.Itoa(int(time)) + " " + strconv.Itoa(int(refresh)) + " " +
				strconv.Itoa(int(retry)) + " " + strconv.Itoa(int(expire)) + " " +
				strconv.Itoa(int(ttl)))),
			stackitem.NewBigInteger(big.NewInt(0)),
		}),
		stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray([]byte("neo.com")),
			stackitem.Make(nns.TXT),
			stackitem.NewByteArray([]byte("bla1")),
			stackitem.NewBigInteger(big.NewInt(0)),
		}),
	}))
}

func TestGetRecords(t *testing.T) {
	c := newNSClient(t, true)
	e := c.Executor
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	acc := e.NewAccount(t)
	cAcc := c.WithSigners(acc)
	cAccCommittee := c.WithSigners(acc, c.Committee)

	cAccCommittee.Invoke(t, true, "register", "neo.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.A), "1.2.3.4")
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.CNAME), "alias.com")

	// Add some arbitrary data.
	cAccCommittee.Invoke(t, true, "register", "alias.com", acc.ScriptHash(), mail, refresh, retry, expire, ttl)
	cAcc.Invoke(t, stackitem.Null{}, "addRecord", "alias.com", int64(nns.TXT), "sometxt")

	c.Invoke(t, stackitem.NewArray([]stackitem.Item{stackitem.Make("1.2.3.4")}), "getRecords", "neo.com", int64(nns.A))
	// Check empty result of `getRecords`.
	c.Invoke(t, stackitem.NewArray([]stackitem.Item{}), "getRecords", "neo.com", int64(nns.AAAA))
}

func TestNNSAddRecord(t *testing.T) {
	c := newNSClient(t, true)
	cAccCommittee := c.WithSigners(c.Committee)
	mail, refresh, retry, expire, ttl := "sami@nspcc.ru", int64(101), int64(102), int64(millisecondsInYear/1000*100), int64(104)

	cAccCommittee.Invoke(t, true, "register", "neo.com", c.CommitteeHash, mail, refresh, retry, expire, ttl)

	for i := 0; i <= maxRecordID+1; i++ {
		if i == maxRecordID+1 {
			c.InvokeFail(t, "maximum number of records reached", "addRecord", "neo.com", int64(nns.TXT), strconv.Itoa(i))
		} else {
			c.Invoke(t, stackitem.Null{}, "addRecord", "neo.com", int64(nns.TXT), strconv.Itoa(i))
		}
	}
}

func TestNNSRegisterArbitraryLevelDomain(t *testing.T) {
	c := newNSClient(t, true)

	newArgs := func(domain string, account neotest.Signer) []interface{} {
		return []interface{}{
			domain, account.ScriptHash(), "doesnt@matter.com",
			int64(101), int64(102), int64(103), int64(104),
		}
	}
	acc := c.NewAccount(t)
	cBoth := c.WithSigners(c.Committee, acc)
	args := newArgs("neo.com", acc)
	cBoth.Invoke(t, true, "register", args...)

	c1 := c.WithSigners(acc)
	// Use long (>4 chars) domain name to avoid committee signature check.
	// parent domain is missing
	args[0] = "testnet.filestorage.neo.com"
	c1.InvokeFail(t, "one of the parent domains is not registered", "register", args...)

	args[0] = "filestorage.neo.com"
	c1.Invoke(t, true, "register", args...)

	args[0] = "testnet.filestorage.neo.com"
	c1.Invoke(t, true, "register", args...)

	acc2 := c.NewAccount(t)
	c2 := c.WithSigners(c.Committee, acc2)
	args = newArgs("mainnet.filestorage.neo.com", acc2)
	c2.InvokeFail(t, "not witnessed by admin", "register", args...)

	c1.Invoke(t, stackitem.Null{}, "addRecord",
		"something.mainnet.filestorage.neo.com", int64(nns.A), "1.2.3.4")
	c1.Invoke(t, stackitem.Null{}, "addRecord",
		"another.filestorage.neo.com", int64(nns.A), "4.3.2.1")

	c2 = c.WithSigners(acc, acc2)
	c2.Invoke(t, stackitem.NewBool(false), "isAvailable", "mainnet.filestorage.neo.com")
	c2.InvokeFail(t, "parent domain has conflicting records: something.mainnet.filestorage.neo.com",
		"register", args...)

	c1.Invoke(t, stackitem.Null{}, "deleteRecords",
		"something.mainnet.filestorage.neo.com", int64(nns.A))
	c2.Invoke(t, stackitem.NewBool(true), "isAvailable", "mainnet.filestorage.neo.com")
	c2.Invoke(t, true, "register", args...)

	c2 = c.WithSigners(acc2)
	c2.Invoke(t, stackitem.Null{}, "addRecord",
		"cdn.mainnet.filestorage.neo.com", int64(nns.A), "166.15.14.13")
	result := stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray([]byte("166.15.14.13")),
	})
	c2.Invoke(t, result, "resolve", "cdn.mainnet.filestorage.neo.com", int64(nns.A))
}

const (
	defaultNameServiceDomainPrice = 10_0000_0000
	defaultNameServiceSysfee      = 6000_0000
	maxRecordID                   = 255
)
