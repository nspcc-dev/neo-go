package native

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// prefixAccount is the standard prefix used to store account data.
const prefixAccount = 20

// makeAccountKey creates a key from account script hash.
func makeAccountKey(h util.Uint160) []byte {
	return makeUint160Key(prefixAccount, h)
}

// nep17TokenNative represents NEP-17 token contract.
type nep17TokenNative struct {
	interop.ContractMD
	symbol       string
	decimals     int64
	factor       int64
	incBalance   func(*interop.Context, util.Uint160, *state.StorageItem, *big.Int, *big.Int) (func(), error)
	balFromBytes func(item *state.StorageItem) (*big.Int, error)
}

// totalSupplyKey is the key used to store totalSupply value.
var totalSupplyKey = []byte{11}

func (c *nep17TokenNative) Metadata() *interop.ContractMD {
	return &c.ContractMD
}

func newNEP17Native(name string, id int32) *nep17TokenNative {
	n := &nep17TokenNative{ContractMD: *interop.NewContractMD(name, id)}
	n.Manifest.SupportedStandards = []string{manifest.NEP17StandardName}

	desc := newDescriptor("symbol", smartcontract.StringType)
	md := newMethodAndPrice(n.Symbol, 0, callflag.NoneFlag)
	n.AddMethod(md, desc)

	desc = newDescriptor("decimals", smartcontract.IntegerType)
	md = newMethodAndPrice(n.Decimals, 0, callflag.NoneFlag)
	n.AddMethod(md, desc)

	desc = newDescriptor("totalSupply", smartcontract.IntegerType)
	md = newMethodAndPrice(n.TotalSupply, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("balanceOf", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.balanceOf, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	transferParams := []manifest.Parameter{
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("to", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
	}
	desc = newDescriptor("transfer", smartcontract.BoolType,
		append(transferParams, manifest.NewParameter("data", smartcontract.AnyType))...,
	)
	md = newMethodAndPrice(n.Transfer, 1<<17, callflag.States|callflag.AllowCall|callflag.AllowNotify)
	md.StorageFee = 50
	n.AddMethod(md, desc)

	n.AddEvent("Transfer", transferParams...)

	return n
}

func (c *nep17TokenNative) Initialize(_ *interop.Context) error {
	return nil
}

func (c *nep17TokenNative) Symbol(_ *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewByteArray([]byte(c.symbol))
}

func (c *nep17TokenNative) Decimals(_ *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(c.decimals))
}

func (c *nep17TokenNative) TotalSupply(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	_, supply := c.getTotalSupply(ic.DAO)
	return stackitem.NewBigInteger(supply)
}

func (c *nep17TokenNative) getTotalSupply(d *dao.Simple) (state.StorageItem, *big.Int) {
	si := d.GetStorageItem(c.ID, totalSupplyKey)
	if si == nil {
		si = []byte{}
	}
	return si, bigint.FromBytes(si)
}

func (c *nep17TokenNative) saveTotalSupply(d *dao.Simple, si state.StorageItem, supply *big.Int) {
	si = state.StorageItem(bigint.ToPreallocatedBytes(supply, si))
	d.PutStorageItem(c.ID, totalSupplyKey, si)
}

func (c *nep17TokenNative) Transfer(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	from := toUint160(args[0])
	to := toUint160(args[1])
	amount := toBigInt(args[2])
	err := c.TransferInternal(ic, from, to, amount, args[3])
	return stackitem.NewBool(err == nil)
}

func addrToStackItem(u *util.Uint160) stackitem.Item {
	if u == nil {
		return stackitem.Null{}
	}
	return stackitem.NewByteArray(u.BytesBE())
}

func (c *nep17TokenNative) postTransfer(ic *interop.Context, from, to *util.Uint160, amount *big.Int,
	data stackitem.Item, callOnPayment bool, postCalls ...func()) {
	var skipPostCalls bool
	defer func() {
		if skipPostCalls {
			return
		}
		for _, f := range postCalls {
			if f != nil {
				f()
			}
		}
	}()
	c.emitTransfer(ic, from, to, amount)
	if to == nil || !callOnPayment {
		return
	}
	cs, err := ic.GetContract(*to)
	if err != nil {
		return
	}

	fromArg := stackitem.Item(stackitem.Null{})
	if from != nil {
		fromArg = stackitem.NewByteArray((*from).BytesBE())
	}
	args := []stackitem.Item{
		fromArg,
		stackitem.NewBigInteger(amount),
		data,
	}
	if err := contract.CallFromNative(ic, c.Hash, cs, manifest.MethodOnNEP17Payment, args, false); err != nil {
		skipPostCalls = true
		panic(err)
	}
}

func (c *nep17TokenNative) emitTransfer(ic *interop.Context, from, to *util.Uint160, amount *big.Int) {
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

// updateAccBalance adds specified amount to the acc's balance. If requiredBalance
// is set and amount is 0, then acc's balance is checked against requiredBalance.
func (c *nep17TokenNative) updateAccBalance(ic *interop.Context, acc util.Uint160, amount *big.Int, requiredBalance *big.Int) (func(), error) {
	key := makeAccountKey(acc)
	si := ic.DAO.GetStorageItem(c.ID, key)
	if si == nil {
		if amount.Sign() < 0 {
			return nil, errors.New("insufficient funds")
		}
		if amount.Sign() == 0 {
			// it's OK to transfer 0 if the balance 0, no need to put si to the storage
			return nil, nil
		}
		si = state.StorageItem{}
	}

	postF, err := c.incBalance(ic, acc, &si, amount, requiredBalance)
	if err != nil {
		if si != nil && amount.Sign() <= 0 {
			ic.DAO.PutStorageItem(c.ID, key, si)
		}
		return nil, err
	}
	if si == nil {
		ic.DAO.DeleteStorageItem(c.ID, key)
	} else {
		ic.DAO.PutStorageItem(c.ID, key, si)
	}
	return postF, nil
}

// TransferInternal transfers NEO between accounts.
func (c *nep17TokenNative) TransferInternal(ic *interop.Context, from, to util.Uint160, amount *big.Int, data stackitem.Item) error {
	var postF1, postF2 func()

	if amount.Sign() == -1 {
		return errors.New("negative amount")
	}

	caller := ic.VM.GetCallingScriptHash()
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

	postF1, err := c.updateAccBalance(ic, from, inc, amount)
	if err != nil {
		return err
	}

	if !isEmpty {
		postF2, err = c.updateAccBalance(ic, to, amount, nil)
		if err != nil {
			return err
		}
	}

	c.postTransfer(ic, &from, &to, amount, data, true, postF1, postF2)
	return nil
}

func (c *nep17TokenNative) balanceOf(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	h := toUint160(args[0])
	return stackitem.NewBigInteger(c.balanceOfInternal(ic.DAO, h))
}

func (c *nep17TokenNative) balanceOfInternal(d *dao.Simple, h util.Uint160) *big.Int {
	key := makeAccountKey(h)
	si := d.GetStorageItem(c.ID, key)
	if si == nil {
		return big.NewInt(0)
	}
	balance, err := c.balFromBytes(&si)
	if err != nil {
		panic(fmt.Errorf("can not deserialize balance state: %w", err))
	}
	return balance
}

func (c *nep17TokenNative) mint(ic *interop.Context, h util.Uint160, amount *big.Int, callOnPayment bool) {
	if amount.Sign() == 0 {
		return
	}
	postF := c.addTokens(ic, h, amount)
	c.postTransfer(ic, nil, &h, amount, stackitem.Null{}, callOnPayment, postF)
}

func (c *nep17TokenNative) burn(ic *interop.Context, h util.Uint160, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	amount.Neg(amount)
	postF := c.addTokens(ic, h, amount)
	amount.Neg(amount)
	c.postTransfer(ic, &h, nil, amount, stackitem.Null{}, false, postF)
}

func (c *nep17TokenNative) addTokens(ic *interop.Context, h util.Uint160, amount *big.Int) func() {
	if amount.Sign() == 0 {
		return nil
	}

	key := makeAccountKey(h)
	si := ic.DAO.GetStorageItem(c.ID, key)
	if si == nil {
		si = state.StorageItem{}
	}
	postF, err := c.incBalance(ic, h, &si, amount, nil)
	if err != nil {
		panic(err)
	}
	if si == nil {
		ic.DAO.DeleteStorageItem(c.ID, key)
	} else {
		ic.DAO.PutStorageItem(c.ID, key, si)
	}

	buf, supply := c.getTotalSupply(ic.DAO)
	supply.Add(supply, amount)
	c.saveTotalSupply(ic.DAO, buf, supply)
	return postF
}

func newDescriptor(name string, ret smartcontract.ParamType, ps ...manifest.Parameter) *manifest.Method {
	if len(ps) == 0 {
		ps = []manifest.Parameter{}
	}
	return &manifest.Method{
		Name:       name,
		Parameters: ps,
		ReturnType: ret,
	}
}

func newMethodAndPrice(f interop.Method, cpuFee int64, flags callflag.CallFlag) *interop.MethodAndPrice {
	return &interop.MethodAndPrice{
		Func:          f,
		CPUFee:        cpuFee,
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

func toUint32(s stackitem.Item) uint32 {
	bigInt := toBigInt(s)
	if !bigInt.IsUint64() {
		panic("bigint is not an uint64")
	}
	uint64Value := bigInt.Uint64()
	if uint64Value > math.MaxUint32 {
		panic("bigint does not fit into uint32")
	}
	return uint32(uint64Value)
}

func toInt64(s stackitem.Item) int64 {
	bigInt := toBigInt(s)
	if !bigInt.IsInt64() {
		panic("bigint is not an uint64")
	}
	return bigInt.Int64()
}
