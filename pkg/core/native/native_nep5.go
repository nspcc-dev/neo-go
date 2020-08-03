package native

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// prefixAccount is the standard prefix used to store account data.
const prefixAccount = 20

// makeAccountKey creates a key from account script hash.
func makeAccountKey(h util.Uint160) []byte {
	k := make([]byte, util.Uint160Size+1)
	k[0] = prefixAccount
	copy(k[1:], h.BytesBE())
	return k
}

// nep5TokenNative represents NEP-5 token contract.
type nep5TokenNative struct {
	interop.ContractMD
	symbol     string
	decimals   int64
	factor     int64
	onPersist  func(*interop.Context) error
	incBalance func(*interop.Context, util.Uint160, *state.StorageItem, *big.Int) error
}

// totalSupplyKey is the key used to store totalSupply value.
var totalSupplyKey = []byte{11}

func (c *nep5TokenNative) Metadata() *interop.ContractMD {
	return &c.ContractMD
}

var _ interop.Contract = (*nep5TokenNative)(nil)

func newNEP5Native(name string) *nep5TokenNative {
	n := &nep5TokenNative{ContractMD: *interop.NewContractMD(name)}
	n.Manifest.SupportedStandards = []string{manifest.NEP5StandardName}

	desc := newDescriptor("name", smartcontract.StringType)
	md := newMethodAndPrice(n.Name, 0, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("symbol", smartcontract.StringType)
	md = newMethodAndPrice(n.Symbol, 0, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("decimals", smartcontract.IntegerType)
	md = newMethodAndPrice(n.Decimals, 0, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("totalSupply", smartcontract.IntegerType)
	md = newMethodAndPrice(n.TotalSupply, 1000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("balanceOf", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.balanceOf, 1000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("transfer", smartcontract.BoolType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("to", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
	)
	md = newMethodAndPrice(n.Transfer, 8000000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("onPersist", smartcontract.VoidType)
	md = newMethodAndPrice(getOnPersistWrapper(n.OnPersist), 0, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc, false)

	n.AddEvent("Transfer", desc.Parameters...)

	return n
}

func (c *nep5TokenNative) Initialize(_ *interop.Context) error {
	return nil
}

func (c *nep5TokenNative) Name(_ *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewByteArray([]byte(c.ContractMD.Name))
}

func (c *nep5TokenNative) Symbol(_ *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewByteArray([]byte(c.symbol))
}

func (c *nep5TokenNative) Decimals(_ *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(c.decimals))
}

func (c *nep5TokenNative) TotalSupply(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(c.getTotalSupply(ic.DAO))
}

func (c *nep5TokenNative) getTotalSupply(d dao.DAO) *big.Int {
	si := d.GetStorageItem(c.ContractID, totalSupplyKey)
	if si == nil {
		return big.NewInt(0)
	}
	return bigint.FromBytes(si.Value)
}

func (c *nep5TokenNative) saveTotalSupply(d dao.DAO, supply *big.Int) error {
	si := &state.StorageItem{Value: bigint.ToBytes(supply)}
	return d.PutStorageItem(c.ContractID, totalSupplyKey, si)
}

func (c *nep5TokenNative) Transfer(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	from := toUint160(args[0])
	to := toUint160(args[1])
	amount := toBigInt(args[2])
	err := c.transfer(ic, from, to, amount)
	return stackitem.NewBool(err == nil)
}

func addrToStackItem(u *util.Uint160) stackitem.Item {
	if u == nil {
		return stackitem.Null{}
	}
	return stackitem.NewByteArray(u.BytesBE())
}

func (c *nep5TokenNative) emitTransfer(ic *interop.Context, from, to *util.Uint160, amount *big.Int) {
	ne := state.NotificationEvent{
		ScriptHash: c.Hash,
		Name:       "Transfer",
		Item: stackitem.NewArray([]stackitem.Item{
			addrToStackItem(from),
			addrToStackItem(to),
			stackitem.NewBigInteger(amount),
		}),
	}
	ic.Notifications = append(ic.Notifications, ne)
}

func (c *nep5TokenNative) updateAccBalance(ic *interop.Context, acc util.Uint160, amount *big.Int) error {
	key := makeAccountKey(acc)
	si := ic.DAO.GetStorageItem(c.ContractID, key)
	if si == nil {
		if amount.Sign() <= 0 {
			return errors.New("insufficient funds")
		}
		si = new(state.StorageItem)
	}

	err := c.incBalance(ic, acc, si, amount)
	if err != nil {
		return err
	}
	if si.Value == nil {
		err = ic.DAO.DeleteStorageItem(c.ContractID, key)
	} else {
		err = ic.DAO.PutStorageItem(c.ContractID, key, si)
	}
	return err
}

func (c *nep5TokenNative) transfer(ic *interop.Context, from, to util.Uint160, amount *big.Int) error {
	if amount.Sign() == -1 {
		return errors.New("negative amount")
	}

	caller := ic.ScriptGetter.GetCallingScriptHash()
	if caller.Equals(util.Uint160{}) || !from.Equals(caller) {
		ok, err := runtime.CheckHashedWitness(ic, from)
		if err != nil {
			return err
		} else if !ok {
			return errors.New("invalid signature")
		}
	}
	isEmpty := from.Equals(to) || amount.Sign() == 0
	inc := amount
	if isEmpty {
		inc = big.NewInt(0)
	} else {
		inc = new(big.Int).Neg(inc)
	}
	if err := c.updateAccBalance(ic, from, inc); err != nil {
		return err
	}

	if !isEmpty {
		if err := c.updateAccBalance(ic, to, amount); err != nil {
			return err
		}
	}

	c.emitTransfer(ic, &from, &to, amount)
	return nil
}

func (c *nep5TokenNative) balanceOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	h := toUint160(args[0])
	bs, err := ic.DAO.GetNEP5Balances(h)
	if err != nil {
		panic(err)
	}
	balance := bs.Trackers[c.ContractID].Balance
	return stackitem.NewBigInteger(&balance)
}

func (c *nep5TokenNative) mint(ic *interop.Context, h util.Uint160, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	c.addTokens(ic, h, amount)
	c.emitTransfer(ic, nil, &h, amount)
}

func (c *nep5TokenNative) burn(ic *interop.Context, h util.Uint160, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	c.addTokens(ic, h, new(big.Int).Neg(amount))
	c.emitTransfer(ic, &h, nil, amount)
}

func (c *nep5TokenNative) addTokens(ic *interop.Context, h util.Uint160, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}

	key := makeAccountKey(h)
	si := ic.DAO.GetStorageItem(c.ContractID, key)
	if si == nil {
		si = new(state.StorageItem)
	}
	if err := c.incBalance(ic, h, si, amount); err != nil {
		panic(err)
	}
	if err := ic.DAO.PutStorageItem(c.ContractID, key, si); err != nil {
		panic(err)
	}

	supply := c.getTotalSupply(ic.DAO)
	supply.Add(supply, amount)
	err := c.saveTotalSupply(ic.DAO, supply)
	if err != nil {
		panic(err)
	}
}

func (c *nep5TokenNative) OnPersist(ic *interop.Context) error {
	if ic.Trigger != trigger.System {
		return errors.New("onPersist should be triggerred by system")
	}
	return nil
}

func newDescriptor(name string, ret smartcontract.ParamType, ps ...manifest.Parameter) *manifest.Method {
	return &manifest.Method{
		Name:       name,
		Parameters: ps,
		ReturnType: ret,
	}
}

func newMethodAndPrice(f interop.Method, price int64, flags smartcontract.CallFlag) *interop.MethodAndPrice {
	return &interop.MethodAndPrice{
		Func:          f,
		Price:         price,
		RequiredFlags: flags,
	}
}

func toBigInt(s stackitem.Item) *big.Int {
	bi, err := s.TryInteger()
	if err != nil {
		panic(err)
	}
	return bi
}

func toUint160(s stackitem.Item) util.Uint160 {
	buf, err := s.TryBytes()
	if err != nil {
		panic(err)
	}
	u, err := util.Uint160DecodeBytesBE(buf)
	if err != nil {
		panic(err)
	}
	return u
}

func getOnPersistWrapper(f func(ic *interop.Context) error) interop.Method {
	return func(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
		return stackitem.NewBool(f(ic) == nil)
	}
}
