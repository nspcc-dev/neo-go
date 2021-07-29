package native

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// GAS represents GAS native contract.
type GAS struct {
	nep17TokenNative
	NEO *NEO

	initialSupply int64
}

const gasContractID = -6

// GASFactor is a divisor for finding GAS integral value.
const GASFactor = NEOTotalSupply

// newGAS returns GAS native contract.
func newGAS(init int64) *GAS {
	g := &GAS{initialSupply: init}
	defer g.UpdateHash()

	nep17 := newNEP17Native(nativenames.Gas, gasContractID)
	nep17.symbol = "GAS"
	nep17.decimals = 8
	nep17.factor = GASFactor
	nep17.incBalance = g.increaseBalance
	nep17.balFromBytes = g.balanceFromBytes

	g.nep17TokenNative = *nep17

	desc := newDescriptor("refuel", smartcontract.VoidType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("amount", smartcontract.IntegerType))
	md := newMethodAndPrice(g.refuel, 1<<15, callflag.States|callflag.AllowNotify)
	g.AddMethod(md, desc)

	return g
}

func (g *GAS) increaseBalance(_ *interop.Context, _ util.Uint160, si *state.StorageItem, amount *big.Int) error {
	acc, err := state.NEP17BalanceFromBytes(*si)
	if err != nil {
		return err
	}
	if sign := amount.Sign(); sign == 0 {
		return nil
	} else if sign == -1 && acc.Balance.Cmp(new(big.Int).Neg(amount)) == -1 {
		return errors.New("insufficient funds")
	}
	acc.Balance.Add(&acc.Balance, amount)
	if acc.Balance.Sign() != 0 {
		*si = acc.Bytes()
	} else {
		*si = nil
	}
	return nil
}

func (g *GAS) balanceFromBytes(si *state.StorageItem) (*big.Int, error) {
	acc, err := state.NEP17BalanceFromBytes(*si)
	if err != nil {
		return nil, err
	}
	return &acc.Balance, err
}

func (g *GAS) refuel(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	acc := toUint160(args[0])
	gas := toBigInt(args[1])

	if !gas.IsInt64() || gas.Sign() == -1 {
		panic("invalid GAS value")
	}

	ok, err := runtime.CheckHashedWitness(ic, acc)
	if !ok || err != nil {
		panic(fmt.Errorf("%w: %v", ErrInvalidWitness, err))
	}

	g.burn(ic, acc, gas)
	ic.VM.GasLimit += gas.Int64()
	return stackitem.Null{}
}

// Initialize initializes GAS contract.
func (g *GAS) Initialize(ic *interop.Context) error {
	if err := g.nep17TokenNative.Initialize(ic); err != nil {
		return err
	}
	if g.nep17TokenNative.getTotalSupply(ic.DAO).Sign() != 0 {
		return errors.New("already initialized")
	}
	h, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	g.mint(ic, h, big.NewInt(g.initialSupply), false)
	return nil
}

// OnPersist implements Contract interface.
func (g *GAS) OnPersist(ic *interop.Context) error {
	if len(ic.Block.Transactions) == 0 {
		return nil
	}
	for _, tx := range ic.Block.Transactions {
		absAmount := big.NewInt(tx.SystemFee + tx.NetworkFee)
		g.burn(ic, tx.Sender(), absAmount)
	}
	validators := g.NEO.GetNextBlockValidatorsInternal()
	primary := validators[ic.Block.PrimaryIndex].GetScriptHash()
	var netFee int64
	for _, tx := range ic.Block.Transactions {
		netFee += tx.NetworkFee
	}
	g.mint(ic, primary, big.NewInt(int64(netFee)), false)
	return nil
}

// PostPersist implements Contract interface.
func (g *GAS) PostPersist(ic *interop.Context) error {
	return nil
}

// BalanceOf returns native GAS token balance for the acc.
func (g *GAS) BalanceOf(d dao.DAO, acc util.Uint160) *big.Int {
	return g.balanceOfInternal(d, acc)
}

func getStandbyValidatorsHash(ic *interop.Context) (util.Uint160, error) {
	s, err := smartcontract.CreateDefaultMultiSigRedeemScript(ic.Chain.GetStandByValidators())
	if err != nil {
		return util.Uint160{}, err
	}
	return hash.Hash160(s), nil
}
