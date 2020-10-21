package native

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// GAS represents GAS native contract.
type GAS struct {
	nep5TokenNative
	NEO *NEO
}

const gasName = "GAS"
const gasContractID = -2

// GASFactor is a divisor for finding GAS integral value.
const GASFactor = NEOTotalSupply
const initialGAS = 30000000

// newGAS returns GAS native contract.
func newGAS() *GAS {
	g := &GAS{}
	nep5 := newNEP5Native(gasName)
	nep5.symbol = "gas"
	nep5.decimals = 8
	nep5.factor = GASFactor
	nep5.onPersist = chainOnPersist(nep5.OnPersist, g.OnPersist)
	nep5.incBalance = g.increaseBalance
	nep5.ContractID = gasContractID

	g.nep5TokenNative = *nep5

	onp := g.Methods["onPersist"]
	onp.Func = getOnPersistWrapper(g.onPersist)
	g.Methods["onPersist"] = onp

	return g
}

func (g *GAS) increaseBalance(_ *interop.Context, _ util.Uint160, si *state.StorageItem, amount *big.Int) error {
	acc, err := state.NEP5BalanceStateFromBytes(si.Value)
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
		si.Value = acc.Bytes()
	} else {
		si.Value = nil
	}
	return nil
}

// Initialize initializes GAS contract.
func (g *GAS) Initialize(ic *interop.Context) error {
	if err := g.nep5TokenNative.Initialize(ic); err != nil {
		return err
	}
	if g.nep5TokenNative.getTotalSupply(ic.DAO).Sign() != 0 {
		return errors.New("already initialized")
	}
	h, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	g.mint(ic, h, big.NewInt(initialGAS*GASFactor))
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
	primary := validators[ic.Block.ConsensusData.PrimaryIndex].GetScriptHash()
	var netFee int64
	for _, tx := range ic.Block.Transactions {
		netFee += tx.NetworkFee
	}
	g.mint(ic, primary, big.NewInt(int64(netFee)))
	return nil
}

func getStandbyValidatorsHash(ic *interop.Context) (util.Uint160, error) {
	s, err := smartcontract.CreateDefaultMultiSigRedeemScript(ic.Chain.GetStandByValidators())
	if err != nil {
		return util.Uint160{}, err
	}
	return hash.Hash160(s), nil
}

func chainOnPersist(fs ...func(*interop.Context) error) func(*interop.Context) error {
	return func(ic *interop.Context) error {
		for i := range fs {
			if fs[i] != nil {
				if err := fs[i](ic); err != nil {
					return err
				}
			}
		}
		return nil
	}
}
