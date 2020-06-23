package native

import (
	"math/big"
	"sort"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/pkg/errors"
)

// NEO represents NEO native contract.
type NEO struct {
	nep5TokenNative
	GAS *GAS
}

// keyWithVotes is a serialized key with votes balance. It's not deserialized
// because some uses of it imply serialized-only usage and converting to
// PublicKey is quite expensive.
type keyWithVotes struct {
	Key   string
	Votes *big.Int
}

const (
	neoSyscallName = "Neo.Native.Tokens.NEO"
	neoContractID  = -1
	// NEOTotalSupply is the total amount of NEO in the system.
	NEOTotalSupply = 100000000
	// prefixValidator is a prefix used to store validator's data.
	prefixValidator = 33
)

var (
	// validatorsCountKey is a key used to store validators count
	// used to determine the real number of validators.
	validatorsCountKey = []byte{15}
	// nextValidatorsKey is a key used to store validators for the
	// next block.
	nextValidatorsKey = []byte{14}
)

// makeValidatorKey creates a key from account script hash.
func makeValidatorKey(key *keys.PublicKey) []byte {
	b := key.Bytes()
	// Don't create a new buffer.
	b = append(b, 0)
	copy(b[1:], b[0:])
	b[0] = prefixValidator
	return b
}

// NewNEO returns NEO native contract.
func NewNEO() *NEO {
	n := &NEO{}
	nep5 := newNEP5Native(neoSyscallName)
	nep5.name = "NEO"
	nep5.symbol = "neo"
	nep5.decimals = 0
	nep5.factor = 1
	nep5.onPersist = chainOnPersist(nep5.OnPersist, n.OnPersist)
	nep5.incBalance = n.increaseBalance
	nep5.ContractID = neoContractID

	n.nep5TokenNative = *nep5

	onp := n.Methods["onPersist"]
	onp.Func = getOnPersistWrapper(n.onPersist)
	n.Methods["onPersist"] = onp

	desc := newDescriptor("unclaimedGas", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("end", smartcontract.IntegerType))
	md := newMethodAndPrice(n.unclaimedGas, 3000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("registerValidator", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.registerValidator, 5000000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("vote", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("pubkeys", smartcontract.ArrayType))
	md = newMethodAndPrice(n.vote, 500000000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("getRegisteredValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getRegisteredValidatorsCall, 100000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("getValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getValidators, 100000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("getNextBlockValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getNextBlockValidators, 100000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

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
	si := new(state.StorageItem)
	si.Value = pubs.Bytes()
	return ic.DAO.PutStorageItem(n.ContractID, nextValidatorsKey, si)
}

func (n *NEO) increaseBalance(ic *interop.Context, h util.Uint160, si *state.StorageItem, amount *big.Int) error {
	acc, err := state.NEOBalanceStateFromBytes(si.Value)
	if err != nil {
		return err
	}
	if amount.Sign() == -1 && acc.Balance.Cmp(new(big.Int).Neg(amount)) == -1 {
		return errors.New("insufficient funds")
	}
	if err := n.distributeGas(ic, h, acc); err != nil {
		return err
	}
	if amount.Sign() == 0 {
		return nil
	}
	if len(acc.Votes) > 0 {
		if err := n.ModifyAccountVotes(acc, ic.DAO, amount); err != nil {
			return err
		}
		siVC := ic.DAO.GetStorageItem(n.ContractID, validatorsCountKey)
		if siVC == nil {
			return errors.New("validators count uninitialized")
		}
		vc, err := ValidatorsCountFromBytes(siVC.Value)
		if err != nil {
			return err
		}
		vc[len(acc.Votes)-1].Add(&vc[len(acc.Votes)-1], amount)
		siVC.Value = vc.Bytes()
		if err := ic.DAO.PutStorageItem(n.ContractID, validatorsCountKey, siVC); err != nil {
			return err
		}
	}
	acc.Balance.Add(&acc.Balance, amount)
	si.Value = acc.Bytes()
	return nil
}

func (n *NEO) distributeGas(ic *interop.Context, h util.Uint160, acc *state.NEOBalanceState) error {
	if ic.Block == nil || ic.Block.Index == 0 {
		return nil
	}
	gen := ic.Chain.CalculateClaimable(acc.Balance.Int64(), acc.BalanceHeight, ic.Block.Index)
	acc.BalanceHeight = ic.Block.Index
	n.GAS.mint(ic, h, big.NewInt(int64(gen)))
	return nil
}

func (n *NEO) unclaimedGas(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	u := toUint160(args[0])
	end := uint32(toBigInt(args[1]).Int64())
	bs, err := ic.DAO.GetNEP5Balances(u)
	if err != nil {
		panic(err)
	}
	tr := bs.Trackers[n.Hash]

	gen := ic.Chain.CalculateClaimable(tr.Balance, tr.LastUpdatedBlock, end)
	return stackitem.NewBigInteger(big.NewInt(int64(gen)))
}

func (n *NEO) registerValidator(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	err := n.registerValidatorInternal(ic, toPublicKey(args[0]))
	return stackitem.NewBool(err == nil)
}

func (n *NEO) registerValidatorInternal(ic *interop.Context, pub *keys.PublicKey) error {
	key := makeValidatorKey(pub)
	si := ic.DAO.GetStorageItem(n.ContractID, key)
	if si != nil {
		return errors.New("already registered")
	}
	si = new(state.StorageItem)
	// Zero value.
	si.Value = []byte{}
	return ic.DAO.PutStorageItem(n.ContractID, key, si)
}

func (n *NEO) vote(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	acc := toUint160(args[0])
	arr := args[1].Value().([]stackitem.Item)
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
	return stackitem.NewBool(err == nil)
}

// VoteInternal votes from account h for validarors specified in pubs.
func (n *NEO) VoteInternal(ic *interop.Context, h util.Uint160, pubs keys.PublicKeys) error {
	ok, err := runtime.CheckHashedWitness(ic, nep5ScriptHash{
		callingScriptHash: util.Uint160{},
		entryScriptHash:   n.Hash,
		currentScriptHash: n.Hash,
	}, h)
	if err != nil {
		return err
	} else if !ok {
		return errors.New("invalid signature")
	}
	key := makeAccountKey(h)
	si := ic.DAO.GetStorageItem(n.ContractID, key)
	if si == nil {
		return errors.New("invalid account")
	}
	acc, err := state.NEOBalanceStateFromBytes(si.Value)
	if err != nil {
		return err
	}
	if err := n.ModifyAccountVotes(acc, ic.DAO, new(big.Int).Neg(&acc.Balance)); err != nil {
		return err
	}
	pubs = pubs.Unique()
	// Check validators registration.
	var newPubs keys.PublicKeys
	for _, pub := range pubs {
		if ic.DAO.GetStorageItem(n.ContractID, makeValidatorKey(pub)) == nil {
			continue
		}
		newPubs = append(newPubs, pub)
	}
	if lp, lv := len(newPubs), len(acc.Votes); lp != lv {
		var si *state.StorageItem
		var vc *ValidatorsCount
		var err error

		si = ic.DAO.GetStorageItem(n.ContractID, validatorsCountKey)
		if si == nil {
			// The first voter.
			si = new(state.StorageItem)
			vc = new(ValidatorsCount)
		} else {
			vc, err = ValidatorsCountFromBytes(si.Value)
			if err != nil {
				return err
			}
		}
		if lv > 0 {
			vc[lv-1].Sub(&vc[lv-1], &acc.Balance)
		}
		if len(newPubs) > 0 {
			vc[lp-1].Add(&vc[lp-1], &acc.Balance)
		}
		si.Value = vc.Bytes()
		if err := ic.DAO.PutStorageItem(n.ContractID, validatorsCountKey, si); err != nil {
			return err
		}
	}
	acc.Votes = newPubs
	if err := n.ModifyAccountVotes(acc, ic.DAO, &acc.Balance); err != nil {
		return err
	}
	si.Value = acc.Bytes()
	return ic.DAO.PutStorageItem(n.ContractID, key, si)
}

// ModifyAccountVotes modifies votes of the specified account by value (can be negative).
func (n *NEO) ModifyAccountVotes(acc *state.NEOBalanceState, d dao.DAO, value *big.Int) error {
	for _, vote := range acc.Votes {
		key := makeValidatorKey(vote)
		si := d.GetStorageItem(n.ContractID, key)
		if si == nil {
			return errors.New("invalid validator")
		}
		votes := bigint.FromBytes(si.Value)
		votes.Add(votes, value)
		si.Value = bigint.ToPreallocatedBytes(votes, si.Value[:0])
		if err := d.PutStorageItem(n.ContractID, key, si); err != nil {
			return err
		}
	}
	return nil
}

func (n *NEO) getRegisteredValidators(d dao.DAO) ([]keyWithVotes, error) {
	siMap, err := d.GetStorageItemsWithPrefix(n.ContractID, []byte{prefixValidator})
	if err != nil {
		return nil, err
	}
	arr := make([]keyWithVotes, 0, len(siMap))
	for key, si := range siMap {
		votes := bigint.FromBytes(si.Value)
		arr = append(arr, keyWithVotes{key, votes})
	}
	sort.Slice(arr, func(i, j int) bool { return strings.Compare(arr[i].Key, arr[j].Key) == -1 })
	return arr, nil
}

// GetRegisteredValidators returns current registered validators list with keys
// and votes.
func (n *NEO) GetRegisteredValidators(d dao.DAO) ([]state.Validator, error) {
	kvs, err := n.getRegisteredValidators(d)
	if err != nil {
		return nil, err
	}
	arr := make([]state.Validator, len(kvs))
	for i := range kvs {
		arr[i].Key, err = keys.NewPublicKeyFromBytes([]byte(kvs[i].Key))
		if err != nil {
			return nil, err
		}
		arr[i].Votes = kvs[i].Votes
	}
	return arr, nil
}

func (n *NEO) getRegisteredValidatorsCall(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	validators, err := n.getRegisteredValidators(ic.DAO)
	if err != nil {
		panic(err)
	}
	arr := make([]stackitem.Item, len(validators))
	for i := range validators {
		arr[i] = stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray([]byte(validators[i].Key)),
			stackitem.NewBigInteger(validators[i].Votes),
		})
	}
	return stackitem.NewArray(arr)
}

// GetValidatorsInternal returns a list of current validators.
func (n *NEO) GetValidatorsInternal(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, error) {
	standByValidators := bc.GetStandByValidators()
	si := d.GetStorageItem(n.ContractID, validatorsCountKey)
	if si == nil {
		return standByValidators, nil
	}
	validatorsCount, err := ValidatorsCountFromBytes(si.Value)
	if err != nil {
		return nil, err
	}
	validators, err := n.GetRegisteredValidators(d)
	if err != nil {
		return nil, err
	}
	sort.Slice(validators, func(i, j int) bool {
		// The most-voted validators should end up in the front of the list.
		cmp := validators[i].Votes.Cmp(validators[j].Votes)
		if cmp != 0 {
			return cmp > 0
		}
		// Ties are broken with public keys.
		return validators[i].Key.Cmp(validators[j].Key) == -1
	})

	count := validatorsCount.GetWeightedAverage()
	if count < len(standByValidators) {
		count = len(standByValidators)
	}

	uniqueSBValidators := standByValidators.Unique()
	result := keys.PublicKeys{}
	for _, validator := range validators {
		if validator.Votes.Sign() > 0 || uniqueSBValidators.Contains(validator.Key) {
			result = append(result, validator.Key)
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

func (n *NEO) getValidators(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	result, err := n.GetValidatorsInternal(ic.Chain, ic.DAO)
	if err != nil {
		panic(err)
	}
	return pubsToArray(result)
}

func (n *NEO) getNextBlockValidators(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	result, err := n.GetNextBlockValidatorsInternal(ic.Chain, ic.DAO)
	if err != nil {
		panic(err)
	}
	return pubsToArray(result)
}

// GetNextBlockValidatorsInternal returns next block validators.
func (n *NEO) GetNextBlockValidatorsInternal(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, error) {
	si := d.GetStorageItem(n.ContractID, nextValidatorsKey)
	if si == nil {
		return n.GetValidatorsInternal(bc, d)
	}
	pubs := keys.PublicKeys{}
	err := pubs.DecodeBytes(si.Value)
	if err != nil {
		return nil, err
	}
	return pubs, nil
}

func pubsToArray(pubs keys.PublicKeys) stackitem.Item {
	arr := make([]stackitem.Item, len(pubs))
	for i := range pubs {
		arr[i] = stackitem.NewByteArray(pubs[i].Bytes())
	}
	return stackitem.NewArray(arr)
}

func toPublicKey(s stackitem.Item) *keys.PublicKey {
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
