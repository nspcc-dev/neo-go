package native

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// GAS represents GAS native contract.
type GAS struct {
	nep17TokenNative
	NEO *NEO
}

const gasContractID = -4

// GASFactor is a divisor for finding GAS integral value.
const GASFactor = NEOTotalSupply
const initialGAS = 30000000

// newGAS returns GAS native contract.
func newGAS() *GAS {
	g := &GAS{}
	nep17 := newNEP17Native(nativenames.Gas, gasContractID)
	nep17.symbol = "GAS"
	nep17.decimals = 8
	nep17.factor = GASFactor
	nep17.incBalance = g.increaseBalance

	g.nep17TokenNative = *nep17

	return g
}

func (g *GAS) increaseBalance(_ *interop.Context, _ util.Uint160, si *state.StorageItem, amount *big.Int) error {
	acc, err := state.NEP17BalanceStateFromBytes(si.Value)
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
	g.mint(ic, h, big.NewInt(initialGAS*GASFactor), false)
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
	g.mint(ic, primary, big.NewInt(int64(netFee)), false)
	return nil
}

// PostPersist implements Contract interface.
func (g *GAS) PostPersist(ic *interop.Context) error {
	return nil
}

func getStandbyValidatorsHash(ic *interop.Context) (util.Uint160, error) {
	s, err := smartcontract.CreateDefaultMultiSigRedeemScript(ic.Chain.GetStandByValidators())
	if err != nil {
		return util.Uint160{}, err
	}
	return hash.Hash160(s), nil
}
