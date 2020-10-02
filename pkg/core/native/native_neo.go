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
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
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

	// gasPerBlock represents current value of generated gas per block.
	// It is append-only and doesn't need to be copied when used.
	gasPerBlock        atomic.Value
	gasPerBlockChanged atomic.Value

	votesChanged   atomic.Value
	nextValidators atomic.Value
	validators     atomic.Value
	// committee contains cached committee members and
	// is updated once in a while depending on committee size
	// (every 28 blocks for mainnet). It's value
	// is always equal to value stored by `prefixCommittee`.
	committee atomic.Value
	// committeeHash contains script hash of the committee.
	committeeHash atomic.Value
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
	// prefixGasPerBlock is a prefix for storing amount of GAS generated per block.
	prefixGASPerBlock = 29
	// effectiveVoterTurnout represents minimal ratio of total supply to total amount voted value
	// which is require to use non-standby validators.
	effectiveVoterTurnout = 5
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to NEO holders.
	neoHolderRewardRatio = 10
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to committee.
	committeeRewardRatio = 5
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to voters.
	voterRewardRatio = 85
)

var (
	// prefixCommittee is a key used to store committee.
	prefixCommittee = []byte{14}
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

// newNEO returns NEO native contract.
func newNEO() *NEO {
	n := &NEO{}
	nep5 := newNEP5Native(neoName)
	nep5.symbol = "neo"
	nep5.decimals = 0
	nep5.factor = 1
	nep5.onPersist = chainOnPersist(nep5.OnPersist, n.OnPersist)
	nep5.postPersist = chainOnPersist(nep5.postPersist, n.PostPersist)
	nep5.incBalance = n.increaseBalance
	nep5.ContractID = neoContractID

	n.nep5TokenNative = *nep5
	n.votesChanged.Store(true)
	n.nextValidators.Store(keys.PublicKeys(nil))
	n.validators.Store(keys.PublicKeys(nil))
	n.committee.Store(keys.PublicKeys(nil))
	n.committeeHash.Store(util.Uint160{})

	onp := n.Methods["onPersist"]
	onp.Func = getOnPersistWrapper(n.onPersist)
	n.Methods["onPersist"] = onp

	pp := n.Methods["postPersist"]
	pp.Func = getOnPersistWrapper(n.postPersist)
	n.Methods["postPersist"] = pp

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

	desc = newDescriptor("getNextBlockValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getNextBlockValidators, 100000000, smartcontract.AllowStates)
	n.AddMethod(md, desc, true)

	desc = newDescriptor("getGasPerBlock", smartcontract.IntegerType)
	md = newMethodAndPrice(n.getGASPerBlock, 100_0000, smartcontract.AllowStates)
	n.AddMethod(md, desc, false)

	desc = newDescriptor("setGasPerBlock", smartcontract.BoolType,
		manifest.NewParameter("gasPerBlock", smartcontract.IntegerType))
	md = newMethodAndPrice(n.setGASPerBlock, 500_0000, smartcontract.AllowModifyStates)
	n.AddMethod(md, desc, false)

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

	committee := ic.Chain.GetStandByCommittee()
	err := n.updateCache(committee, ic.Chain)
	if err != nil {
		return err
	}

	err = ic.DAO.PutStorageItem(n.ContractID, prefixCommittee, &state.StorageItem{Value: committee.Bytes()})
	if err != nil {
		return err
	}

	h, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	n.mint(ic, h, big.NewInt(NEOTotalSupply))

	gr := &state.GASRecord{{Index: 0, GASPerBlock: *big.NewInt(5 * GASFactor)}}
	n.gasPerBlock.Store(*gr)
	n.gasPerBlockChanged.Store(false)
	err = ic.DAO.PutStorageItem(n.ContractID, []byte{prefixGASPerBlock}, &state.StorageItem{Value: gr.Bytes()})
	if err != nil {
		return err
	}
	err = ic.DAO.PutStorageItem(n.ContractID, []byte{prefixVotersCount}, &state.StorageItem{Value: []byte{}})
	if err != nil {
		return err
	}

	return nil
}

func (n *NEO) updateCache(committee keys.PublicKeys, bc blockchainer.Blockchainer) error {
	n.committee.Store(committee)
	script, err := smartcontract.CreateMajorityMultiSigRedeemScript(committee.Copy())
	if err != nil {
		return err
	}
	n.committeeHash.Store(hash.Hash160(script))

	nextVals := committee[:bc.GetConfig().ValidatorsCount].Copy()
	sort.Sort(nextVals)
	n.nextValidators.Store(nextVals)
	return nil
}

func (n *NEO) updateCommittee(ic *interop.Context) error {
	votesChanged := n.votesChanged.Load().(bool)
	if !votesChanged {
		// We need to put in storage anyway, as it affects dumps
		committee := n.committee.Load().(keys.PublicKeys)
		si := &state.StorageItem{Value: committee.Bytes()}
		return ic.DAO.PutStorageItem(n.ContractID, prefixCommittee, si)
	}

	committee, err := n.ComputeCommitteeMembers(ic.Chain, ic.DAO)
	if err != nil {
		return err
	}
	if err := n.updateCache(committee, ic.Chain); err != nil {
		return err
	}
	n.votesChanged.Store(false)
	si := &state.StorageItem{Value: committee.Bytes()}
	return ic.DAO.PutStorageItem(n.ContractID, prefixCommittee, si)
}

func shouldUpdateCommittee(h uint32, bc blockchainer.Blockchainer) bool {
	cfg := bc.GetConfig()
	r := cfg.ValidatorsCount + len(cfg.StandbyCommittee)
	return h%uint32(r) == 0
}

// OnPersist implements Contract interface.
func (n *NEO) OnPersist(ic *interop.Context) error {
	gpb := n.gasPerBlockChanged.Load()
	if gpb == nil {
		committee := keys.PublicKeys{}
		si := ic.DAO.GetStorageItem(n.ContractID, prefixCommittee)
		if err := committee.DecodeBytes(si.Value); err != nil {
			return err
		}
		if err := n.updateCache(committee, ic.Chain); err != nil {
			return err
		}

		var gr state.GASRecord
		si = ic.DAO.GetStorageItem(n.ContractID, []byte{prefixGASPerBlock})
		if err := gr.FromBytes(si.Value); err != nil {
			return err
		}
		n.gasPerBlock.Store(gr)
		n.gasPerBlockChanged.Store(false)
	}
	if shouldUpdateCommittee(ic.Block.Index, ic.Chain) {
		if err := n.updateCommittee(ic); err != nil {
			return err
		}
	}
	return nil
}

// PostPersist implements Contract interface.
func (n *NEO) PostPersist(ic *interop.Context) error {
	gas := n.GetGASPerBlock(ic.DAO, ic.Block.Index)
	pubs := n.GetCommitteeMembers()
	index := int(ic.Block.Index) % len(ic.Chain.GetConfig().StandbyCommittee)
	gas.Mul(gas, big.NewInt(committeeRewardRatio))
	n.GAS.mint(ic, pubs[index].GetScriptHash(), gas.Div(gas, big.NewInt(100)))
	n.OnPersistEnd(ic.DAO)
	return nil
}

// OnPersistEnd updates cached values if they've been changed.
func (n *NEO) OnPersistEnd(d dao.DAO) {
	if n.gasPerBlockChanged.Load().(bool) {
		gr := n.getGASRecordFromDAO(d)
		n.gasPerBlock.Store(gr)
		n.gasPerBlockChanged.Store(false)
	}
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
	if err := n.ModifyAccountVotes(acc, ic.DAO, amount, false); err != nil {
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
	gen, err := n.CalculateBonus(ic, &acc.Balance, acc.BalanceHeight, ic.Block.Index)
	if err != nil {
		return err
	}
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

	gen, err := n.CalculateBonus(ic, &tr.Balance, tr.LastUpdatedBlock, end)
	if err != nil {
		panic(err)
	}
	return stackitem.NewBigInteger(gen)
}

func (n *NEO) getGASPerBlock(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	gas := n.GetGASPerBlock(ic.DAO, ic.Block.Index)
	return stackitem.NewBigInteger(gas)
}

func (n *NEO) getGASRecordFromDAO(d dao.DAO) state.GASRecord {
	var gr state.GASRecord
	si := d.GetStorageItem(n.ContractID, []byte{prefixGASPerBlock})
	_ = gr.FromBytes(si.Value)
	return gr
}

// GetGASPerBlock returns gas generated for block with provided index.
func (n *NEO) GetGASPerBlock(d dao.DAO, index uint32) *big.Int {
	var gr state.GASRecord
	if n.gasPerBlockChanged.Load().(bool) {
		gr = n.getGASRecordFromDAO(d)
	} else {
		gr = n.gasPerBlock.Load().(state.GASRecord)
	}
	for i := len(gr) - 1; i >= 0; i-- {
		if gr[i].Index <= index {
			g := gr[i].GASPerBlock
			return &g
		}
	}
	panic("contract not initialized")
}

// GetCommitteeAddress returns address of the committee.
func (n *NEO) GetCommitteeAddress() util.Uint160 {
	return n.committeeHash.Load().(util.Uint160)
}

func (n *NEO) setGASPerBlock(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	gas := toBigInt(args[0])
	ok, err := n.SetGASPerBlock(ic, ic.Block.Index+1, gas)
	if err != nil {
		panic(err)
	}
	return stackitem.NewBool(ok)
}

// SetGASPerBlock sets gas generated for blocks after index.
func (n *NEO) SetGASPerBlock(ic *interop.Context, index uint32, gas *big.Int) (bool, error) {
	if gas.Sign() == -1 || gas.Cmp(big.NewInt(10*GASFactor)) == 1 {
		return false, errors.New("invalid value for GASPerBlock")
	}
	h := n.GetCommitteeAddress()
	ok, err := runtime.CheckHashedWitness(ic, h)
	if err != nil || !ok {
		return ok, err
	}
	si := ic.DAO.GetStorageItem(n.ContractID, []byte{prefixGASPerBlock})
	var gr state.GASRecord
	if err := gr.FromBytes(si.Value); err != nil {
		return false, err
	}
	if len(gr) > 0 && gr[len(gr)-1].Index == index {
		gr[len(gr)-1].GASPerBlock = *gas
	} else {
		gr = append(gr, state.GASIndexPair{
			Index:       index,
			GASPerBlock: *gas,
		})
	}
	n.gasPerBlockChanged.Store(true)
	return true, ic.DAO.PutStorageItem(n.ContractID, []byte{prefixGASPerBlock}, &state.StorageItem{Value: gr.Bytes()})
}

// CalculateBonus calculates amount of gas generated for holding `value` NEO from start to end block.
func (n *NEO) CalculateBonus(ic *interop.Context, value *big.Int, start, end uint32) (*big.Int, error) {
	if value.Sign() == 0 || start >= end {
		return big.NewInt(0), nil
	} else if value.Sign() < 0 {
		return nil, errors.New("negative value")
	}
	si := ic.DAO.GetStorageItem(n.ContractID, []byte{prefixGASPerBlock})
	var gr state.GASRecord
	if err := gr.FromBytes(si.Value); err != nil {
		return nil, err
	}
	var sum, tmp big.Int
	for i := len(gr) - 1; i >= 0; i-- {
		if gr[i].Index >= end {
			continue
		} else if gr[i].Index <= start {
			tmp.SetInt64(int64(end - start))
			tmp.Mul(&tmp, &gr[i].GASPerBlock)
			sum.Add(&sum, &tmp)
			break
		}
		tmp.SetInt64(int64(end - gr[i].Index))
		tmp.Mul(&tmp, &gr[i].GASPerBlock)
		sum.Add(&sum, &tmp)
		end = gr[i].Index
	}
	res := new(big.Int).Mul(value, &sum)
	res.Mul(res, tmp.SetInt64(neoHolderRewardRatio))
	res.Div(res, tmp.SetInt64(100*NEOTotalSupply))
	return res, nil
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
	if err := n.ModifyAccountVotes(acc, ic.DAO, new(big.Int).Neg(&acc.Balance), false); err != nil {
		return err
	}
	acc.VoteTo = pub
	if err := n.ModifyAccountVotes(acc, ic.DAO, &acc.Balance, true); err != nil {
		return err
	}
	si.Value = acc.Bytes()
	return ic.DAO.PutStorageItem(n.ContractID, key, si)
}

// ModifyAccountVotes modifies votes of the specified account by value (can be negative).
// typ specifies if this modify is occurring during transfer or vote (with old or new validator).
func (n *NEO) ModifyAccountVotes(acc *state.NEOBalanceState, d dao.DAO, value *big.Int, isNewVote bool) error {
	n.votesChanged.Store(true)
	if acc.VoteTo != nil {
		key := makeValidatorKey(acc.VoteTo)
		si := d.GetStorageItem(n.ContractID, key)
		if si == nil {
			return errors.New("invalid validator")
		}
		cd := new(candidate).FromBytes(si.Value)
		cd.Votes.Add(&cd.Votes, value)
		if !isNewVote {
			if !cd.Registered && cd.Votes.Sign() == 0 {
				return d.DeleteStorageItem(n.ContractID, key)
			}
		} else if !cd.Registered {
			return errors.New("validator must be registered")
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

// ComputeNextBlockValidators returns an actual list of current validators.
func (n *NEO) ComputeNextBlockValidators(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, error) {
	if vals := n.validators.Load().(keys.PublicKeys); vals != nil {
		return vals.Copy(), nil
	}
	result, err := n.ComputeCommitteeMembers(bc, d)
	if err != nil {
		return nil, err
	}
	result = result[:bc.GetConfig().ValidatorsCount]
	sort.Sort(result)
	n.validators.Store(result)
	return result, nil
}

func (n *NEO) getCommittee(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	pubs := n.GetCommitteeMembers()
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

// GetCommitteeMembers returns public keys of nodes in committee using cached value.
func (n *NEO) GetCommitteeMembers() keys.PublicKeys {
	return n.committee.Load().(keys.PublicKeys).Copy()
}

// ComputeCommitteeMembers returns public keys of nodes in committee.
func (n *NEO) ComputeCommitteeMembers(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, error) {
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
		pubs := bc.GetStandByCommittee()
		return pubs, nil
	}
	cs, err := n.getCandidates(d)
	if err != nil {
		return nil, err
	}
	sbVals := bc.GetStandByCommittee()
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
	result := n.GetNextBlockValidatorsInternal()
	return pubsToArray(result)
}

// GetNextBlockValidatorsInternal returns next block validators.
func (n *NEO) GetNextBlockValidatorsInternal() keys.PublicKeys {
	return n.nextValidators.Load().(keys.PublicKeys).Copy()
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
