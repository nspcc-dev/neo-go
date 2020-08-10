package native

import (
	"crypto/elliptic"
	"errors"
	"math/big"
	"sort"
	"strings"
	"sync/atomic"

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
)

// NEO represents NEO native contract.
type NEO struct {
	nep5TokenNative
	GAS *GAS

	validators atomic.Value
}

// keyWithVotes is a serialized key with votes balance. It's not deserialized
// because some uses of it imply serialized-only usage and converting to
// PublicKey is quite expensive.
type keyWithVotes struct {
	Key   string
	Votes *big.Int
}

const (
	neoName       = "NEO"
	neoContractID = -1
	// NEOTotalSupply is the total amount of NEO in the system.
	NEOTotalSupply = 100000000
	// prefixCandidate is a prefix used to store validator's data.
	prefixCandidate = 33
	// prefixVotersCount is a prefix for storing total amount of NEO of voters.
	prefixVotersCount = 1
	// effectiveVoterTurnout represents minimal ratio of total supply to total amount voted value
	// which is require to use non-standby validators.
	effectiveVoterTurnout = 5
)

var (
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
	b[0] = prefixCandidate
	return b
}

// NewNEO returns NEO native contract.
func NewNEO() *NEO {
	n := &NEO{}
	nep5 := newNEP5Native(neoName)
	nep5.symbol = "neo"
	nep5.decimals = 0
	nep5.factor = 1
	nep5.onPersist = chainOnPersist(nep5.OnPersist, n.OnPersist)
	nep5.incBalance = n.increaseBalance
	nep5.ContractID = neoContractID

	n.nep5TokenNative = *nep5
	n.validators.Store(keys.PublicKeys(nil))

	onp := n.Methods["onPersist"]
	onp.Func = getOnPersistWrapper(n.onPersist)
	n.Methods["onPersist"] = onp

	desc := newDescriptor("unclaimedGas", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("end", smartcontract.IntegerType))
	md := newMethodAndPrice(n.unclaimedGas, 3000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("registerCandidate", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.registerCandidate, 5000000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("unregisterCandidate", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.unregisterCandidate, 5000000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("vote", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.vote, 500000000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("getCandidates", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getCandidatesCall, 100000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("getСommittee", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getCommittee, 100000000, smartcontract.AllowStates)
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

	if n.nep5TokenNative.getTotalSupply(ic.DAO).Sign() != 0 {
		return errors.New("already initialized")
	}

	h, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	n.mint(ic, h, big.NewInt(NEOTotalSupply))

	err = ic.DAO.PutStorageItem(n.ContractID, []byte{prefixVotersCount}, &state.StorageItem{Value: []byte{}})
	if err != nil {
		return err
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
		si.Value = acc.Bytes()
		return nil
	}
	if err := n.ModifyAccountVotes(acc, ic.DAO, amount, modifyVoteTransfer); err != nil {
		return err
	}
	if acc.VoteTo != nil {
		if err := n.modifyVoterTurnout(ic.DAO, amount); err != nil {
			return err
		}
	}
	acc.Balance.Add(&acc.Balance, amount)
	if acc.Balance.Sign() != 0 {
		si.Value = acc.Bytes()
	} else {
		si.Value = nil
	}
	return nil
}

func (n *NEO) distributeGas(ic *interop.Context, h util.Uint160, acc *state.NEOBalanceState) error {
	if ic.Block == nil || ic.Block.Index == 0 {
		return nil
	}
	gen := ic.Chain.CalculateClaimable(&acc.Balance, acc.BalanceHeight, ic.Block.Index)
	acc.BalanceHeight = ic.Block.Index
	n.GAS.mint(ic, h, gen)
	return nil
}

func (n *NEO) unclaimedGas(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	u := toUint160(args[0])
	end := uint32(toBigInt(args[1]).Int64())
	bs, err := ic.DAO.GetNEP5Balances(u)
	if err != nil {
		panic(err)
	}
	tr := bs.Trackers[n.ContractID]

	gen := ic.Chain.CalculateClaimable(&tr.Balance, tr.LastUpdatedBlock, end)
	return stackitem.NewBigInteger(gen)
}

func (n *NEO) registerCandidate(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	pub := toPublicKey(args[0])
	ok, err := runtime.CheckKeyedWitness(ic, pub)
	if err != nil {
		panic(err)
	} else if !ok {
		return stackitem.NewBool(false)
	}
	err = n.RegisterCandidateInternal(ic, pub)
	return stackitem.NewBool(err == nil)
}

// RegisterCandidateInternal registers pub as a new candidate.
func (n *NEO) RegisterCandidateInternal(ic *interop.Context, pub *keys.PublicKey) error {
	key := makeValidatorKey(pub)
	si := ic.DAO.GetStorageItem(n.ContractID, key)
	if si == nil {
		c := &candidate{Registered: true}
		si = &state.StorageItem{Value: c.Bytes()}
	} else {
		c := new(candidate).FromBytes(si.Value)
		c.Registered = true
		si.Value = c.Bytes()
	}
	return ic.DAO.PutStorageItem(n.ContractID, key, si)
}

func (n *NEO) unregisterCandidate(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	pub := toPublicKey(args[0])
	ok, err := runtime.CheckKeyedWitness(ic, pub)
	if err != nil {
		panic(err)
	} else if !ok {
		return stackitem.NewBool(false)
	}
	err = n.UnregisterCandidateInternal(ic, pub)
	return stackitem.NewBool(err == nil)
}

// UnregisterCandidateInternal unregisters pub as a candidate.
func (n *NEO) UnregisterCandidateInternal(ic *interop.Context, pub *keys.PublicKey) error {
	key := makeValidatorKey(pub)
	si := ic.DAO.GetStorageItem(n.ContractID, key)
	if si == nil {
		return nil
	}
	n.validators.Store(keys.PublicKeys(nil))
	c := new(candidate).FromBytes(si.Value)
	if c.Votes.Sign() == 0 {
		return ic.DAO.DeleteStorageItem(n.ContractID, key)
	}
	c.Registered = false
	si.Value = c.Bytes()
	return ic.DAO.PutStorageItem(n.ContractID, key, si)
}

func (n *NEO) vote(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	acc := toUint160(args[0])
	var pub *keys.PublicKey
	if _, ok := args[1].(stackitem.Null); !ok {
		pub = toPublicKey(args[1])
	}
	err := n.VoteInternal(ic, acc, pub)
	return stackitem.NewBool(err == nil)
}

// VoteInternal votes from account h for validarors specified in pubs.
func (n *NEO) VoteInternal(ic *interop.Context, h util.Uint160, pub *keys.PublicKey) error {
	ok, err := runtime.CheckHashedWitness(ic, h)
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
	if (acc.VoteTo == nil) != (pub == nil) {
		val := &acc.Balance
		if pub == nil {
			val = new(big.Int).Neg(val)
		}
		if err := n.modifyVoterTurnout(ic.DAO, val); err != nil {
			return err
		}
	}
	if err := n.ModifyAccountVotes(acc, ic.DAO, new(big.Int).Neg(&acc.Balance), modifyVoteOld); err != nil {
		return err
	}
	acc.VoteTo = pub
	if err := n.ModifyAccountVotes(acc, ic.DAO, &acc.Balance, modifyVoteNew); err != nil {
		return err
	}
	si.Value = acc.Bytes()
	return ic.DAO.PutStorageItem(n.ContractID, key, si)
}

const (
	modifyVoteTransfer = iota
	modifyVoteOld
	modifyVoteNew
)

// ModifyAccountVotes modifies votes of the specified account by value (can be negative).
// typ specifies if this modify is occuring during transfer or vote (with old or new validator).
func (n *NEO) ModifyAccountVotes(acc *state.NEOBalanceState, d dao.DAO, value *big.Int, typ int) error {
	if acc.VoteTo != nil {
		key := makeValidatorKey(acc.VoteTo)
		si := d.GetStorageItem(n.ContractID, key)
		if si == nil {
			return errors.New("invalid validator")
		}
		cd := new(candidate).FromBytes(si.Value)
		cd.Votes.Add(&cd.Votes, value)
		switch typ {
		case modifyVoteOld:
			if !cd.Registered && cd.Votes.Sign() == 0 {
				return d.DeleteStorageItem(n.ContractID, key)
			}
		case modifyVoteNew:
			if !cd.Registered {
				return errors.New("validator must be registered")
			}
		}
		n.validators.Store(keys.PublicKeys(nil))
		si.Value = cd.Bytes()
		return d.PutStorageItem(n.ContractID, key, si)
	}
	return nil
}

func (n *NEO) getCandidates(d dao.DAO) ([]keyWithVotes, error) {
	siMap, err := d.GetStorageItemsWithPrefix(n.ContractID, []byte{prefixCandidate})
	if err != nil {
		return nil, err
	}
	arr := make([]keyWithVotes, 0, len(siMap))
	for key, si := range siMap {
		c := new(candidate).FromBytes(si.Value)
		if c.Registered {
			arr = append(arr, keyWithVotes{key, &c.Votes})
		}
	}
	sort.Slice(arr, func(i, j int) bool { return strings.Compare(arr[i].Key, arr[j].Key) == -1 })
	return arr, nil
}

// GetCandidates returns current registered validators list with keys
// and votes.
func (n *NEO) GetCandidates(d dao.DAO) ([]state.Validator, error) {
	kvs, err := n.getCandidates(d)
	if err != nil {
		return nil, err
	}
	arr := make([]state.Validator, len(kvs))
	for i := range kvs {
		arr[i].Key, err = keys.NewPublicKeyFromBytes([]byte(kvs[i].Key), elliptic.P256())
		if err != nil {
			return nil, err
		}
		arr[i].Votes = kvs[i].Votes
	}
	return arr, nil
}

func (n *NEO) getCandidatesCall(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	validators, err := n.getCandidates(ic.DAO)
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
	if vals := n.validators.Load().(keys.PublicKeys); vals != nil {
		return vals.Copy(), nil
	}
	result, err := n.getCommitteeMembers(bc, d)
	if err != nil {
		return nil, err
	}
	count := bc.GetConfig().ValidatorsCount
	if len(result) < count {
		count = len(result)
	}
	result = result[:count]
	sort.Sort(result)
	n.validators.Store(result)
	return result, nil
}

func (n *NEO) getValidators(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	result, err := n.GetValidatorsInternal(ic.Chain, ic.DAO)
	if err != nil {
		panic(err)
	}
	return pubsToArray(result)
}

func (n *NEO) getCommittee(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	pubs, err := n.getCommitteeMembers(ic.Chain, ic.DAO)
	if err != nil {
		panic(err)
	}
	sort.Sort(pubs)
	return pubsToArray(pubs)
}

func (n *NEO) modifyVoterTurnout(d dao.DAO, amount *big.Int) error {
	key := []byte{prefixVotersCount}
	si := d.GetStorageItem(n.ContractID, key)
	if si == nil {
		return errors.New("voters count not found")
	}
	votersCount := bigint.FromBytes(si.Value)
	votersCount.Add(votersCount, amount)
	si.Value = bigint.ToBytes(votersCount)
	return d.PutStorageItem(n.ContractID, key, si)
}

func (n *NEO) getCommitteeMembers(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, error) {
	key := []byte{prefixVotersCount}
	si := d.GetStorageItem(n.ContractID, key)
	if si == nil {
		return nil, errors.New("voters count not found")
	}
	votersCount := bigint.FromBytes(si.Value)
	// votersCount / totalSupply must be >= 0.2
	votersCount.Mul(votersCount, big.NewInt(effectiveVoterTurnout))
	voterTurnout := votersCount.Div(votersCount, n.getTotalSupply(d))
	if voterTurnout.Sign() != 1 {
		return bc.GetStandByValidators(), nil
	}
	cs, err := n.getCandidates(d)
	if err != nil {
		return nil, err
	}
	sbVals := bc.GetStandByValidators()
	count := len(sbVals)
	if len(cs) < count {
		return sbVals, nil
	}
	sort.Slice(cs, func(i, j int) bool {
		// The most-voted validators should end up in the front of the list.
		cmp := cs[i].Votes.Cmp(cs[j].Votes)
		if cmp != 0 {
			return cmp > 0
		}
		// Ties are broken with public keys.
		return strings.Compare(cs[i].Key, cs[j].Key) == -1
	})
	pubs := make(keys.PublicKeys, count)
	for i := range pubs {
		pubs[i], err = keys.NewPublicKeyFromBytes([]byte(cs[i].Key), elliptic.P256())
		if err != nil {
			return nil, err
		}
	}
	return pubs, nil
}

func (n *NEO) getNextBlockValidators(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	result, err := n.getNextBlockValidatorsInternal(ic.Chain, ic.DAO)
	if err != nil {
		panic(err)
	}
	return pubsToArray(result)
}

// GetNextBlockValidatorsInternal returns next block validators.
func (n *NEO) GetNextBlockValidatorsInternal(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, error) {
	pubs, err := n.getNextBlockValidatorsInternal(bc, d)
	if err != nil {
		return nil, err
	}
	return pubs.Copy(), nil
}

// getNextBlockValidatorsInternal returns next block validators.
func (n *NEO) getNextBlockValidatorsInternal(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, error) {
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
