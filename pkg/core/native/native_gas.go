package native

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// GAS represents GAS native contract.
type GAS struct {
	nep17TokenNative
	NEO    *NEO
	Policy *Policy

	initialSupply           int64
	p2pSigExtensionsEnabled bool
}

const gasContractID = -6

// GASFactor is a divisor for finding GAS integral value.
const GASFactor = NEOTotalSupply

// newGAS returns GAS native contract.
func newGAS(init int64, p2pSigExtensionsEnabled bool) *GAS {
	g := &GAS{
		initialSupply:           init,
		p2pSigExtensionsEnabled: p2pSigExtensionsEnabled,
	}
	defer g.BuildHFSpecificMD(g.ActiveIn())

	nep17 := newNEP17Native(nativenames.Gas, gasContractID, nil)
	nep17.symbol = "GAS"
	nep17.decimals = 8
	nep17.factor = GASFactor
	nep17.incBalance = g.increaseBalance
	nep17.balFromBytes = g.balanceFromBytes

	g.nep17TokenNative = *nep17

	return g
}

func (g *GAS) increaseBalance(_ *interop.Context, _ util.Uint160, si *state.StorageItem, amount *big.Int, checkBal *big.Int) (func(), error) {
	acc, err := state.NEP17BalanceFromBytes(*si)
	if err != nil {
		return nil, err
	}
	if sign := amount.Sign(); sign == 0 {
		// Requested self-transfer amount can be higher than actual balance.
		if checkBal != nil && acc.Balance.Cmp(checkBal) < 0 {
			err = errors.New("insufficient funds")
		}
		return nil, err
	} else if sign == -1 && acc.Balance.CmpAbs(amount) == -1 {
		return nil, errors.New("insufficient funds")
	}
	acc.Balance.Add(&acc.Balance, amount)
	if acc.Balance.Sign() != 0 {
		*si = acc.Bytes(nil)
	} else {
		*si = nil
	}
	return nil, nil
}

func (g *GAS) balanceFromBytes(si *state.StorageItem) (*big.Int, error) {
	acc, err := state.NEP17BalanceFromBytes(*si)
	if err != nil {
		return nil, err
	}
	return &acc.Balance, err
}

// Initialize initializes a GAS contract.
func (g *GAS) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	if hf != g.ActiveIn() {
		return nil
	}

	if err := g.nep17TokenNative.Initialize(ic); err != nil {
		return err
	}
	_, totalSupply := g.nep17TokenNative.getTotalSupply(ic.DAO)
	if totalSupply.Sign() != 0 {
		return errors.New("already initialized")
	}
	h, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	g.mint(ic, h, big.NewInt(g.initialSupply), false)
	return nil
}

// InitializeCache implements the Contract interface.
func (g *GAS) InitializeCache(blockHeight uint32, d *dao.Simple) error {
	return nil
}

// OnPersist implements the Contract interface.
func (g *GAS) OnPersist(ic *interop.Context) error {
	if len(ic.Block.Transactions) == 0 {
		return nil
	}
	for _, tx := range ic.Block.Transactions {
		absAmount := big.NewInt(tx.SystemFee + tx.NetworkFee)
		g.burn(ic, tx.Sender(), absAmount)
	}
	validators := g.NEO.GetNextBlockValidatorsInternal(ic.DAO)
	primary := validators[ic.Block.PrimaryIndex].GetScriptHash()
	var netFee int64
	for _, tx := range ic.Block.Transactions {
		netFee += tx.NetworkFee
		if g.p2pSigExtensionsEnabled {
			// Reward for NotaryAssisted attribute will be minted to designated notary nodes
			// by Notary contract.
			attrs := tx.GetAttributes(transaction.NotaryAssistedT)
			if len(attrs) != 0 {
				na := attrs[0].Value.(*transaction.NotaryAssisted)
				netFee -= (int64(na.NKeys) + 1) * g.Policy.GetAttributeFeeInternal(ic.DAO, transaction.NotaryAssistedT)
			}
		}
	}
	g.mint(ic, primary, big.NewInt(int64(netFee)), false)
	return nil
}

// PostPersist implements the Contract interface.
func (g *GAS) PostPersist(ic *interop.Context) error {
	return nil
}

// ActiveIn implements the Contract interface.
func (g *GAS) ActiveIn() *config.Hardfork {
	return nil
}

// BalanceOf returns native GAS token balance for the acc.
func (g *GAS) BalanceOf(d *dao.Simple, acc util.Uint160) *big.Int {
	return g.balanceOfInternal(d, acc)
}

func getStandbyValidatorsHash(ic *interop.Context) (util.Uint160, error) {
	cfg := ic.Chain.GetConfig()
	committee, err := keys.NewPublicKeysFromStrings(cfg.StandbyCommittee)
	if err != nil {
		return util.Uint160{}, err
	}
	s, err := smartcontract.CreateDefaultMultiSigRedeemScript(committee[:cfg.GetNumOfCNs(0)])
	if err != nil {
		return util.Uint160{}, err
	}
	return hash.Hash160(s), nil
}
