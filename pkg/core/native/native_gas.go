package native

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// GAS represents GAS native contract.
type GAS struct {
	nep5TokenNative
	NEO *NEO
}

const gasSyscallName = "Neo.Native.Tokens.GAS"

// NewGAS returns GAS native contract.
func NewGAS() *GAS {
	nep5 := newNEP5Native(gasSyscallName)
	nep5.name = "GAS"
	nep5.symbol = "gas"
	nep5.decimals = 8
	nep5.factor = 100000000

	g := &GAS{nep5TokenNative: *nep5}

	desc := newDescriptor("getSysFeeAmount", smartcontract.IntegerType,
		manifest.NewParameter("index", smartcontract.IntegerType))
	md := newMethodAndPrice(g.getSysFeeAmount, 1, smartcontract.NoneFlag)
	g.AddMethod(md, desc, true)

	g.onPersist = chainOnPersist(g.onPersist, g.OnPersist)
	g.incBalance = g.increaseBalance
	return g
}

// initFromStore initializes variable contract parameters from the store.
func (g *GAS) InitFromStore(data []byte) error {
	g.totalSupply = *emit.BytesToInt(data)
	return nil
}

func (g *GAS) serializeState() []byte {
	return emit.IntToBytes(&g.totalSupply)
}

func (g *GAS) increaseBalance(_ *interop.Context, acc *state.Account, amount *big.Int) error {
	if sign := amount.Sign(); sign == 0 {
		return nil
	} else if sign == -1 && acc.GAS.Balance.Cmp(new(big.Int).Neg(amount)) == -1 {
		return errors.New("insufficient funds")
	}
	acc.GAS.Balance.Add(&acc.GAS.Balance, amount)
	return nil
}

// Initialize initializes GAS contract.
func (g *GAS) Initialize(ic *interop.Context) error {
	data, err := ic.DAO.GetNativeContractState(g.Hash)
	if err == nil {
		return g.InitFromStore(data)
	} else if err != storage.ErrKeyNotFound {
		return err
	}

	if err := g.nep5TokenNative.Initialize(); err != nil {
		return err
	}
	h, _, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	g.mint(ic, h, big.NewInt(30000000*g.factor))
	return ic.DAO.PutNativeContractState(g.Hash, g.serializeState())
}

// OnPersist implements Contract interface.
func (g *GAS) OnPersist(ic *interop.Context) error {
	//for _ ,tx := range ic.block.Transactions {
	//	g.burn(ic, tx.Sender, tx.SystemFee + tx.NetworkFee)
	//}
	//validators := g.NEO.getNextBlockValidators(ic)
	//var netFee util.Fixed8
	//for _, tx := range ic.block.Transactions {
	//	netFee += tx.NetworkFee
	//}
	//g.mint(ic, <primary>, netFee)
	return ic.DAO.PutNativeContractState(g.Hash, g.serializeState())
}

func (g *GAS) getSysFeeAmount(ic *interop.Context, args []vm.StackItem) vm.StackItem {
	index := toBigInt(args[0])
	h := ic.Chain.GetHeaderHash(int(index.Int64()))
	_, sf, err := ic.DAO.GetBlock(h)
	if err != nil {
		panic(err)
	}
	return vm.NewBigIntegerItem(big.NewInt(int64(sf)))
}

func getStandbyValidatorsHash(ic *interop.Context) (util.Uint160, []*keys.PublicKey, error) {
	vs, err := ic.Chain.GetStandByValidators()
	if err != nil {
		return util.Uint160{}, nil, err
	}
	s, err := smartcontract.CreateMultiSigRedeemScript(len(vs)/2+1, vs)
	if err != nil {
		return util.Uint160{}, nil, err
	}
	return hash.Hash160(s), vs, nil
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
