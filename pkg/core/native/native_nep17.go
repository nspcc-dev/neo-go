package native

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/config"
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
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// prefixAccount is the standard prefix used to store account data.
const prefixAccount = 20

// makeAccountKey creates a key from the account script hash.
func makeAccountKey(h util.Uint160) []byte {
	return makeUint160Key(prefixAccount, h)
}

// nep17TokenNative represents a NEP-17 token contract.
type nep17TokenNative struct {
	interop.ContractMD
	// gasMinter is something capable of GAS minting (GAS contract in fact).
	// It's nil for the GAS contract itself and guaranteed not to be invoked for
	// the GAS contract.
	gasMinter    IGASMinter
	symbol       string
	decimals     int64
	factor       int64
	incBalance   func(*interop.Context, util.Uint160, *state.StorageItem, *big.Int, *big.Int) (*gasDistribution, error)
	balFromBytes func(item *state.StorageItem) (*big.Int, error)
}

// IGASMinter is an abstraction leak that describes GAS minting functionality.
type IGASMinter interface {
	// MintDeferrable mints specified amount of GAS to the receiver address and
	// optionally calls onNEP17Payment method. It always either runs or schedules
	// continuation, so the caller must never call the continuation afterwards
	// by himself.
	MintDeferrable(ic *interop.Context, to util.Uint160, amount *big.Int, callOnPayment bool, continuation func())
}

// totalSupplyKey is the key used to store totalSupply value.
var totalSupplyKey = []byte{11}

func (c *nep17TokenNative) Metadata() *interop.ContractMD {
	return &c.ContractMD
}

func newNEP17Native(name string, id int32, onManifestConstruction func(m *manifest.Manifest, hf config.Hardfork)) *nep17TokenNative {
	n := &nep17TokenNative{ContractMD: *interop.NewContractMD(name, id, func(m *manifest.Manifest, hf config.Hardfork) {
		m.SupportedStandards = []string{manifest.NEP17StandardName}
		if onManifestConstruction != nil {
			onManifestConstruction(m, hf)
		}
	})}

	desc := NewDescriptor("symbol", smartcontract.StringType)
	md := NewMethodAndPrice(n.Symbol, 0, callflag.NoneFlag)
	n.AddMethod(md, desc)

	desc = NewDescriptor("decimals", smartcontract.IntegerType)
	md = NewMethodAndPrice(n.Decimals, 0, callflag.NoneFlag)
	n.AddMethod(md, desc)

	desc = NewDescriptor("totalSupply", smartcontract.IntegerType)
	md = NewMethodAndPrice(n.TotalSupply, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = NewDescriptor("balanceOf", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = NewMethodAndPrice(n.balanceOf, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	transferParams := []manifest.Parameter{
		manifest.NewParameter("from", smartcontract.Hash160Type),
		manifest.NewParameter("to", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType),
	}
	desc = NewDescriptor("transfer", smartcontract.BoolType,
		append(transferParams, manifest.NewParameter("data", smartcontract.AnyType))...,
	)
	md = NewMethodAndPriceDeferrable(n.transferDeferrable, 1<<17, callflag.States|callflag.AllowCall|callflag.AllowNotify)
	md.StorageFee = 50
	n.AddMethod(md, desc)

	eDesc := NewEventDescriptor("Transfer", transferParams...)
	eMD := NewEvent(eDesc)
	n.AddEvent(eMD)

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
	d.PutBigInt(c.ID, totalSupplyKey, supply)
}

func (c *nep17TokenNative) transferDeferrable(ic *interop.Context, args []stackitem.Item, popArgsPushRes func(res stackitem.Item)) {
	from := toUint160(args[0])
	to := toUint160(args[1])
	amount := toBigInt(args[2])
	data := args[3]
	var dist1, dist2 *gasDistribution

	if amount.Sign() == -1 {
		panic(errors.New("negative amount"))
	}

	caller := ic.VM.GetCallingScriptHash()
	if caller.Equals(util.Uint160{}) || !from.Equals(caller) {
		ok, err := runtime.CheckHashedWitness(ic, from)
		if err != nil || !ok {
			popArgsPushRes(stackitem.NewBool(false))
			return
		}
	}
	isEmpty := from.Equals(to) || amount.Sign() == 0
	inc := amount
	if isEmpty {
		inc = big.NewInt(0)
	} else {
		inc = new(big.Int).Neg(inc)
	}

	dist1, err := c.updateAccBalance(ic, from, inc, amount)
	if err != nil {
		popArgsPushRes(stackitem.NewBool(false))
		return
	}

	if !isEmpty {
		dist2, err = c.updateAccBalance(ic, to, amount, nil)
		if err != nil {
			popArgsPushRes(stackitem.NewBool(false))
			return
		}
	}

	c.postTransfer(ic, &from, &to, amount, data, true, popArgsPushRes, dist1, dist2)
}

func addrToStackItem(u *util.Uint160) stackitem.Item {
	if u == nil {
		return stackitem.Null{}
	}
	return stackitem.NewByteArray(u.BytesBE())
}

// postTransfer emits Transfer notification, calls onNEP17Payment (if so) and schedules
// GAS distributions (if so). It always either runs or schedules popArgsPushRes callback
// execution so the caller must not call it afterward.
func (c *nep17TokenNative) postTransfer(ic *interop.Context, from, to *util.Uint160, amount *big.Int,
	data stackitem.Item, callOnPayment bool, popArgsPushRes func(stackitem.Item), dists ...*gasDistribution) {
	err := c.emitTransfer(ic, from, to, amount)
	if err != nil {
		panic(err)
	}
	continuation := func() {
		if len(dists) > 2 {
			// Write more nested continuations and nil distributions filtering if you need to support more than 2 stacked distributions.
			panic(fmt.Errorf("program bug: too many GAS distributions in postTransfer: max 2, got %d", len(dists)))
		}
		if len(dists) > 0 && dists[0] == nil { // filter out possible first nil distribution.
			dists = dists[1:]
		}
		// Execute a part of Mint, enqueue context with onNEP17Payment and return.
		if len(dists) > 0 && dists[0] != nil {
			c.gasMinter.MintDeferrable(ic, dists[0].To, dists[0].Amount, callOnPayment, func() {
				if len(dists) > 1 && dists[1] != nil { // at max two distributions are supported for now.
					c.gasMinter.MintDeferrable(ic, dists[1].To, dists[1].Amount, callOnPayment, func() {
						popArgsPushRes(stackitem.NewBool(true)) // the result of `transfer` in fact.
					})
				} else {
					popArgsPushRes(stackitem.NewBool(true)) // the result of `transfer` in fact.
				}
			})
		} else {
			popArgsPushRes(stackitem.NewBool(true)) // the result of `transfer` in fact.
		}
	}
	if to == nil || !callOnPayment {
		continuation()
		return
	}
	cs, err := ic.GetContract(*to)
	if err != nil {
		continuation()
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
	err = contract.CallFromNative(ic, c.Hash, cs, manifest.MethodOnNEP17Payment, args, false, func(v *vm.VM) {
		// onNEP17Payment returns void => no need to pop the result from the parent's stack.
		continuation()
	})
	if err != nil {
		panic(err)
	}
}

func (c *nep17TokenNative) emitTransfer(ic *interop.Context, from, to *util.Uint160, amount *big.Int) error {
	return ic.AddNotification(c.Hash, "Transfer", stackitem.NewArray([]stackitem.Item{
		addrToStackItem(from),
		addrToStackItem(to),
		stackitem.NewBigInteger(amount),
	}))
}

// updateAccBalance adds the specified amount to the acc's balance. If requiredBalance
// is set and amount is 0, the acc's balance is checked against requiredBalance.
func (c *nep17TokenNative) updateAccBalance(ic *interop.Context, acc util.Uint160, amount *big.Int, requiredBalance *big.Int) (*gasDistribution, error) {
	key := makeAccountKey(acc)
	si := ic.DAO.GetStorageItem(c.ID, key)
	if si == nil {
		if amount.Sign() < 0 {
			return nil, errors.New("insufficient funds")
		}
		if requiredBalance != nil && requiredBalance.Sign() > 0 {
			return nil, errors.New("insufficient funds")
		}
		if amount.Sign() == 0 {
			// it's OK to transfer 0 if the balance is 0, no need to put si to the storage
			return nil, nil
		}
		si = state.StorageItem{}
	}

	dist, err := c.incBalance(ic, acc, &si, amount, requiredBalance)
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
	return dist, nil
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

func (c *nep17TokenNative) MintDeferrable(ic *interop.Context, h util.Uint160, amount *big.Int, callOnPayment bool, continuation func()) {
	if amount.Sign() == 0 {
		continuation()
		return
	}
	c.addTokens(ic, h, amount)
	c.postTransfer(ic, nil, &h, amount, stackitem.Null{}, callOnPayment,
		// postTransfer is primarily designated to work as a `transfer` callback, that's why is always either returns
		// `true` or panics, but MintDeferrable doesn't return anything, so skip `res` handling and just continue the
		// caller's execution flow.
		func(res stackitem.Item) { continuation() })
}

func (c *nep17TokenNative) Burn(ic *interop.Context, h util.Uint160, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	amount.Neg(amount)
	dist := c.addTokens(ic, h, amount)
	amount.Neg(amount)
	c.postTransfer(ic, &h, nil, amount, stackitem.Null{}, false, func(res stackitem.Item) {}, dist) // no onNEP17Payment call => no source of asynchronous execution.
}

func (c *nep17TokenNative) addTokens(ic *interop.Context, h util.Uint160, amount *big.Int) *gasDistribution {
	if amount.Sign() == 0 {
		return nil
	}

	key := makeAccountKey(h)
	si := ic.DAO.GetStorageItem(c.ID, key)
	if si == nil {
		si = state.StorageItem{}
	}
	dist, err := c.incBalance(ic, h, &si, amount, nil)
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
	return dist
}

func NewDescriptor(name string, ret smartcontract.ParamType, ps ...manifest.Parameter) *manifest.Method {
	if len(ps) == 0 {
		ps = []manifest.Parameter{}
	}
	return &manifest.Method{
		Name:       name,
		Parameters: ps,
		ReturnType: ret,
	}
}

// NewMethodAndPrice builds method with the provided descriptor and ActiveFrom/ActiveTill hardfork
// values consequently specified via activations. [config.HFDefault] specified as ActiveFrom is treated
// as active starting from the genesis block.
func NewMethodAndPrice(f interop.Method, cpuFee int64, flags callflag.CallFlag, activations ...config.Hardfork) *interop.MethodAndPrice {
	return newMethodAndPriceInternal(f, nil, cpuFee, flags, activations...)
}

// NewMethodAndPriceDeferrable works similar to NewMethodAndPrice except that it expects a deferrable
// method handler that processes the result via callback instead of direct return.
func NewMethodAndPriceDeferrable(f interop.DeferrableMethod, cpuFee int64, flags callflag.CallFlag, activations ...config.Hardfork) *interop.MethodAndPrice {
	return newMethodAndPriceInternal(nil, f, cpuFee, flags, activations...)
}

func newMethodAndPriceInternal(f interop.Method, fDeferrable interop.DeferrableMethod, cpuFee int64, flags callflag.CallFlag, activations ...config.Hardfork) *interop.MethodAndPrice {
	md := &interop.MethodAndPrice{
		HFSpecificMethodAndPrice: interop.HFSpecificMethodAndPrice{
			Func:           f,
			DeferrableFunc: fDeferrable,
			CPUFee:         cpuFee,
			RequiredFlags:  flags,
		},
	}
	if len(activations) > 0 {
		if activations[0] != config.HFDefault {
			md.ActiveFrom = &activations[0]
		}
	}
	if len(activations) > 1 {
		md.ActiveTill = &activations[1]
	}
	return md
}

func NewEventDescriptor(name string, ps ...manifest.Parameter) *manifest.Event {
	if len(ps) == 0 {
		ps = []manifest.Parameter{}
	}
	return &manifest.Event{
		Name:       name,
		Parameters: ps,
	}
}

// NewEvent builds event with the provided descriptor and ActiveFrom/ActiveTill hardfork
// values consequently specified via activations.
func NewEvent(desc *manifest.Event, activations ...config.Hardfork) interop.Event {
	md := interop.Event{
		HFSpecificEvent: interop.HFSpecificEvent{
			MD: desc,
		},
	}
	if len(activations) > 0 {
		md.ActiveFrom = &activations[0]
	}
	if len(activations) > 1 {
		md.ActiveTill = &activations[1]
	}
	return md
}
