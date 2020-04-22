package native

import (
	"math/big"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/pkg/errors"
)

// NEO represents NEO native contract.
type NEO struct {
	nep5TokenNative
	GAS *GAS
}

const neoSyscallName = "Neo.Native.Tokens.NEO"

// NEOTotalSupply is the total amount of NEO in the system.
const NEOTotalSupply = 100000000

// NewNEO returns NEO native contract.
func NewNEO() *NEO {
	nep5 := newNEP5Native(neoSyscallName)
	nep5.name = "NEO"
	nep5.symbol = "neo"
	nep5.decimals = 0
	nep5.factor = 1

	n := &NEO{nep5TokenNative: *nep5}

	desc := newDescriptor("unclaimedGas", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("end", smartcontract.IntegerType))
	md := newMethodAndPrice(n.unclaimedGas, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("registerValidator", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.registerValidator, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("vote", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("pubkeys", smartcontract.ArrayType))
	md = newMethodAndPrice(n.vote, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("getRegisteredValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getRegisteredValidators, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("getValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getValidators, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("getNextBlockValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getNextBlockValidators, 1, smartcontract.NoneFlag)
	n.AddMethod(md, desc, true)

	n.onPersist = chainOnPersist(n.onPersist, n.OnPersist)
	n.incBalance = n.increaseBalance

	return n
}

// Initialize initializes NEO contract.
func (n *NEO) Initialize(ic *interop.Context) error {
	if err := n.nep5TokenNative.Initialize(ic); err != nil {
		return err
	}

	if n.nep5TokenNative.getTotalSupply(ic).Sign() != 0 {
		return errors.New("already initialized")
	}

	h, vs, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	n.mint(ic, h, big.NewInt(NEOTotalSupply))

	for i := range vs {
		if err := n.registerValidatorInternal(ic, vs[i]); err != nil {
			return err
		}
	}

	return nil
}

// OnPersist implements Contract interface.
func (n *NEO) OnPersist(ic *interop.Context) error {
	pubs, err := n.GetValidatorsInternal(ic.Chain, ic.DAO)
	if err != nil {
		return err
	}
	if err := ic.DAO.PutNextBlockValidators(pubs); err != nil {
		return err
	}
	return nil
}

func (n *NEO) increaseBalance(ic *interop.Context, acc *state.Account, amount *big.Int) error {
	if sign := amount.Sign(); sign == 0 {
		return nil
	} else if sign == -1 && acc.NEO.Balance.Cmp(new(big.Int).Neg(amount)) == -1 {
		return errors.New("insufficient funds")
	}
	if err := n.distributeGas(ic, acc); err != nil {
		return err
	}
	acc.NEO.Balance.Add(&acc.NEO.Balance, amount)
	return nil
}

func (n *NEO) distributeGas(ic *interop.Context, acc *state.Account) error {
	if ic.Block == nil {
		return nil
	}
	sys, net, err := ic.Chain.CalculateClaimable(util.Fixed8(acc.NEO.Balance.Int64()), acc.NEO.BalanceHeight, ic.Block.Index)
	if err != nil {
		return err
	}
	acc.NEO.BalanceHeight = ic.Block.Index
	n.GAS.mint(ic, acc.ScriptHash, big.NewInt(int64(sys+net)))
	return nil
}

func (n *NEO) unclaimedGas(ic *interop.Context, args []vm.StackItem) vm.StackItem {
	u := toUint160(args[0])
	end := uint32(toBigInt(args[1]).Int64())
	bs, err := ic.DAO.GetNEP5Balances(u)
	if err != nil {
		panic(err)
	}
	tr := bs.Trackers[n.Hash]

	sys, net, err := ic.Chain.CalculateClaimable(util.Fixed8(tr.Balance), tr.LastUpdatedBlock, end)
	if err != nil {
		panic(err)
	}
	return vm.NewBigIntegerItem(big.NewInt(int64(sys.Add(net))))
}

func (n *NEO) registerValidator(ic *interop.Context, args []vm.StackItem) vm.StackItem {
	err := n.registerValidatorInternal(ic, toPublicKey(args[0]))
	return vm.NewBoolItem(err == nil)
}

func (n *NEO) registerValidatorInternal(ic *interop.Context, pub *keys.PublicKey) error {
	_, err := ic.DAO.GetValidatorState(pub)
	if err == nil {
		return err
	}
	return ic.DAO.PutValidatorState(&state.Validator{PublicKey: pub})
}

func (n *NEO) vote(ic *interop.Context, args []vm.StackItem) vm.StackItem {
	acc := toUint160(args[0])
	arr := args[1].Value().([]vm.StackItem)
	var pubs keys.PublicKeys
	for i := range arr {
		pub := new(keys.PublicKey)
		bs, err := arr[i].TryBytes()
		if err != nil {
			panic(err)
		} else if err := pub.DecodeBytes(bs); err != nil {
			panic(err)
		}
		pubs = append(pubs, pub)
	}
	err := n.VoteInternal(ic, acc, pubs)
	return vm.NewBoolItem(err == nil)
}

// VoteInternal votes from account h for validarors specified in pubs.
func (n *NEO) VoteInternal(ic *interop.Context, h util.Uint160, pubs keys.PublicKeys) error {
	ok, err := runtime.CheckHashedWitness(ic, h)
	if err != nil {
		return err
	} else if !ok {
		return errors.New("invalid signature")
	}
	acc, err := ic.DAO.GetAccountState(h)
	if err != nil {
		return err
	}
	balance := util.Fixed8(acc.NEO.Balance.Int64())
	if err := ModifyAccountVotes(acc, ic.DAO, -balance); err != nil {
		return err
	}
	pubs = pubs.Unique()
	var newPubs keys.PublicKeys
	for _, pub := range pubs {
		_, err := ic.DAO.GetValidatorState(pub)
		if err != nil {
			if err == storage.ErrKeyNotFound {
				continue
			}
			return err
		}
		newPubs = append(newPubs, pub)
	}
	if lp, lv := len(newPubs), len(acc.Votes); lp != lv {
		vc, err := ic.DAO.GetValidatorsCount()
		if err != nil {
			return err
		}
		if lv > 0 {
			vc[lv-1] -= balance
		}
		if len(newPubs) > 0 {
			vc[lp-1] += balance
		}
		if err := ic.DAO.PutValidatorsCount(vc); err != nil {
			return err
		}
	}
	acc.Votes = newPubs
	return ModifyAccountVotes(acc, ic.DAO, balance)
}

// ModifyAccountVotes modifies votes of the specified account by value (can be negative).
func ModifyAccountVotes(acc *state.Account, d dao.DAO, value util.Fixed8) error {
	for _, vote := range acc.Votes {
		validator, err := d.GetValidatorStateOrNew(vote)
		if err != nil {
			return err
		}
		validator.Votes += value
		if validator.UnregisteredAndHasNoVotes() {
			if err := d.DeleteValidatorState(validator); err != nil {
				return err
			}
		} else {
			if err := d.PutValidatorState(validator); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *NEO) getRegisteredValidators(ic *interop.Context, _ []vm.StackItem) vm.StackItem {
	vs := ic.DAO.GetValidators()
	arr := make([]vm.StackItem, len(vs))
	for i := range vs {
		arr[i] = vm.NewStructItem([]vm.StackItem{
			vm.NewByteArrayItem(vs[i].PublicKey.Bytes()),
			vm.NewBigIntegerItem(big.NewInt(int64(vs[i].Votes))),
		})
	}
	return vm.NewArrayItem(arr)
}

// GetValidatorsInternal returns a list of current validators.
func (n *NEO) GetValidatorsInternal(bc blockchainer.Blockchainer, d dao.DAO) ([]*keys.PublicKey, error) {
	validatorsCount, err := d.GetValidatorsCount()
	if err != nil {
		return nil, err
	} else if len(validatorsCount) == 0 {
		sb, err := bc.GetStandByValidators()
		if err != nil {
			return nil, err
		}
		return sb, nil
	}

	validators := d.GetValidators()
	sort.Slice(validators, func(i, j int) bool {
		// Unregistered validators go to the end of the list.
		if validators[i].Registered != validators[j].Registered {
			return validators[i].Registered
		}
		// The most-voted validators should end up in the front of the list.
		if validators[i].Votes != validators[j].Votes {
			return validators[i].Votes > validators[j].Votes
		}
		// Ties are broken with public keys.
		return validators[i].PublicKey.Cmp(validators[j].PublicKey) == -1
	})

	count := validatorsCount.GetWeightedAverage()
	standByValidators, err := bc.GetStandByValidators()
	if err != nil {
		return nil, err
	}
	if count < len(standByValidators) {
		count = len(standByValidators)
	}

	uniqueSBValidators := standByValidators.Unique()
	result := keys.PublicKeys{}
	for _, validator := range validators {
		if validator.RegisteredAndHasVotes() || uniqueSBValidators.Contains(validator.PublicKey) {
			result = append(result, validator.PublicKey)
		}
	}

	if result.Len() >= count {
		result = result[:count]
	} else {
		for i := 0; i < uniqueSBValidators.Len() && result.Len() < count; i++ {
			if !result.Contains(uniqueSBValidators[i]) {
				result = append(result, uniqueSBValidators[i])
			}
		}
	}
	sort.Sort(result)
	return result, nil
}

func (n *NEO) getValidators(ic *interop.Context, _ []vm.StackItem) vm.StackItem {
	result, err := n.GetValidatorsInternal(ic.Chain, ic.DAO)
	if err != nil {
		panic(err)
	}
	return pubsToArray(result)
}

func (n *NEO) getNextBlockValidators(ic *interop.Context, _ []vm.StackItem) vm.StackItem {
	result, err := n.GetNextBlockValidatorsInternal(ic.Chain, ic.DAO)
	if err != nil {
		panic(err)
	}
	return pubsToArray(result)
}

// GetNextBlockValidatorsInternal returns next block validators.
func (n *NEO) GetNextBlockValidatorsInternal(bc blockchainer.Blockchainer, d dao.DAO) ([]*keys.PublicKey, error) {
	result, err := d.GetNextBlockValidators()
	if err != nil {
		return nil, err
	} else if result == nil {
		return bc.GetStandByValidators()
	}
	return result, nil
}

func pubsToArray(pubs keys.PublicKeys) vm.StackItem {
	arr := make([]vm.StackItem, len(pubs))
	for i := range pubs {
		arr[i] = vm.NewByteArrayItem(pubs[i].Bytes())
	}
	return vm.NewArrayItem(arr)
}

func toPublicKey(s vm.StackItem) *keys.PublicKey {
	buf, err := s.TryBytes()
	if err != nil {
		panic(err)
	}
	pub := new(keys.PublicKey)
	if err := pub.DecodeBytes(buf); err != nil {
		panic(err)
	}
	return pub
}
