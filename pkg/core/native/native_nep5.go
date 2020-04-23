package native

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// nep5TokenNative represents NEP-5 token contract.
type nep5TokenNative struct {
	interop.ContractMD
	name       string
	symbol     string
	decimals   int64
	factor     int64
	onPersist  func(*interop.Context) error
	incBalance func(*interop.Context, *state.Account, *big.Int) error
}

// totalSupplyKey is the key used to store totalSupply value.
var totalSupplyKey = []byte{11}

func (c *nep5TokenNative) Metadata() *interop.ContractMD {
	return &c.ContractMD
}

var _ interop.Contract = (*nep5TokenNative)(nil)

func newNEP5Native(name string) *nep5TokenNative {
	n := &nep5TokenNative{ContractMD: *interop.NewContractMD(name)}

	desc := newDescriptor("name", smartcontract.StringType)
	md := newMethodAndPrice(n.Name, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("symbol", smartcontract.StringType)
	md = newMethodAndPrice(n.Symbol, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("decimals", smartcontract.IntegerType)
	md = newMethodAndPrice(n.Decimals, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("totalSupply", smartcontract.IntegerType)
	md = newMethodAndPrice(n.TotalSupply, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("balanceOf", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.balanceOf, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("transfer", smartcontract.BoolType,
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("to", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
	)
	md = newMethodAndPrice(n.Transfer, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, false)

	n.AddEvent("Transfer", desc.Parameters...)

	return n
}

func (c *nep5TokenNative) Initialize(_ *interop.Context) error {
	return nil
}

func (c *nep5TokenNative) Name(_ *interop.Context, _ []vm.StackItem) vm.StackItem {
	return vm.NewByteArrayItem([]byte(c.name))
}

func (c *nep5TokenNative) Symbol(_ *interop.Context, _ []vm.StackItem) vm.StackItem {
	return vm.NewByteArrayItem([]byte(c.symbol))
}

func (c *nep5TokenNative) Decimals(_ *interop.Context, _ []vm.StackItem) vm.StackItem {
	return vm.NewBigIntegerItem(big.NewInt(c.decimals))
}

func (c *nep5TokenNative) TotalSupply(ic *interop.Context, _ []vm.StackItem) vm.StackItem {
	return vm.NewBigIntegerItem(c.getTotalSupply(ic))
}

func (c *nep5TokenNative) getTotalSupply(ic *interop.Context) *big.Int {
	si := ic.DAO.GetStorageItem(c.Hash, totalSupplyKey)
	if si == nil {
		return big.NewInt(0)
	}
	return emit.BytesToInt(si.Value)
}

func (c *nep5TokenNative) saveTotalSupply(ic *interop.Context, supply *big.Int) error {
	si := &state.StorageItem{Value: emit.IntToBytes(supply)}
	return ic.DAO.PutStorageItem(c.Hash, totalSupplyKey, si)
}

func (c *nep5TokenNative) Transfer(ic *interop.Context, args []vm.StackItem) vm.StackItem {
	from := toUint160(args[0])
	to := toUint160(args[1])
	amount := toBigInt(args[2])
	err := c.transfer(ic, from, to, amount)
	return vm.NewBoolItem(err == nil)
}

func addrToStackItem(u *util.Uint160) vm.StackItem {
	if u == nil {
		return vm.NullItem{}
	}
	return vm.NewByteArrayItem(u.BytesBE())
}

func (c *nep5TokenNative) emitTransfer(ic *interop.Context, from, to *util.Uint160, amount *big.Int) {
	ne := state.NotificationEvent{
		ScriptHash: c.Hash,
		Item: vm.NewArrayItem([]vm.StackItem{
			vm.NewByteArrayItem([]byte("Transfer")),
			addrToStackItem(from),
			addrToStackItem(to),
			vm.NewBigIntegerItem(amount),
		}),
	}
	ic.Notifications = append(ic.Notifications, ne)
}

func (c *nep5TokenNative) transfer(ic *interop.Context, from, to util.Uint160, amount *big.Int) error {
	if amount.Sign() == -1 {
		return errors.New("negative amount")
	}

	accFrom, err := ic.DAO.GetAccountStateOrNew(from)
	if err != nil {
		return err
	}

	isEmpty := from.Equals(to) || amount.Sign() == 0
	inc := amount
	if isEmpty {
		inc = big.NewInt(0)
	}
	if err := c.incBalance(ic, accFrom, inc); err != nil {
		return err
	}
	if err := ic.DAO.PutAccountState(accFrom); err != nil {
		return err
	}

	if !isEmpty {
		accTo, err := ic.DAO.GetAccountStateOrNew(to)
		if err != nil {
			return err
		}
		if err := c.incBalance(ic, accTo, amount); err != nil {
			return err
		}
		if err := ic.DAO.PutAccountState(accTo); err != nil {
			return err
		}
	}

	c.emitTransfer(ic, &from, &to, amount)
	return nil
}

func (c *nep5TokenNative) balanceOf(ic *interop.Context, args []vm.StackItem) vm.StackItem {
	h := toUint160(args[0])
	bs, err := ic.DAO.GetNEP5Balances(h)
	if err != nil {
		panic(err)
	}
	balance := bs.Trackers[c.Hash].Balance
	return vm.NewBigIntegerItem(big.NewInt(balance))
}

func (c *nep5TokenNative) mint(ic *interop.Context, h util.Uint160, amount *big.Int) {
	c.addTokens(ic, h, amount)
	c.emitTransfer(ic, nil, &h, amount)
}

func (c *nep5TokenNative) burn(ic *interop.Context, h util.Uint160, amount *big.Int) {
	amount = new(big.Int).Neg(amount)
	c.addTokens(ic, h, amount)
	c.emitTransfer(ic, &h, nil, amount)
}

func (c *nep5TokenNative) addTokens(ic *interop.Context, h util.Uint160, amount *big.Int) {
	if sign := amount.Sign(); sign == -1 {
		panic("negative amount")
	} else if sign == 0 {
		return
	}

	acc, err := ic.DAO.GetAccountStateOrNew(h)
	if err != nil {
		panic(err)
	}
	if err := c.incBalance(ic, acc, amount); err != nil {
		panic(err)
	}
	if err := ic.DAO.PutAccountState(acc); err != nil {
		panic(err)
	}

	supply := c.getTotalSupply(ic)
	supply.Add(supply, amount)
	err = c.saveTotalSupply(ic, supply)
	if err != nil {
		panic(err)
	}
}

func (c *nep5TokenNative) OnPersist(ic *interop.Context) error {
	return c.onPersist(ic)
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

func toBigInt(s vm.StackItem) *big.Int {
	bi, err := s.TryInteger()
	if err != nil {
		panic(err)
	}
	return bi
}

func toUint160(s vm.StackItem) util.Uint160 {
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
