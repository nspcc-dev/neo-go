package native

import (
	"crypto/elliptic"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NEO represents NEO native contract.
type NEO struct {
	nep17TokenNative
	GAS *GAS

	// gasPerBlock represents current value of generated gas per block.
	// It is append-only and doesn't need to be copied when used.
	gasPerBlock        atomic.Value
	gasPerBlockChanged atomic.Value

	registerPrice        atomic.Value
	registerPriceChanged atomic.Value

	votesChanged   atomic.Value
	nextValidators atomic.Value
	validators     atomic.Value
	// committee contains cached committee members and their votes.
	// It is updated once in a while depending on committee size
	// (every 28 blocks for mainnet). It's value
	// is always equal to value stored by `prefixCommittee`.
	committee atomic.Value
	// committeeHash contains script hash of the committee.
	committeeHash atomic.Value

	// gasPerVoteCache contains last updated value of GAS per vote reward for candidates.
	// It is set in state-modifying methods only and read in `PostPersist` thus is not protected
	// by any mutex.
	gasPerVoteCache map[string]big.Int
}

const (
	neoContractID = -5
	// NEOTotalSupply is the total amount of NEO in the system.
	NEOTotalSupply = 100000000
	// DefaultRegisterPrice is default price for candidate register.
	DefaultRegisterPrice = 1000 * GASFactor
	// prefixCandidate is a prefix used to store validator's data.
	prefixCandidate = 33
	// prefixVotersCount is a prefix for storing total amount of NEO of voters.
	prefixVotersCount = 1
	// prefixVoterRewardPerCommittee is a prefix for storing committee GAS reward.
	prefixVoterRewardPerCommittee = 23
	// voterRewardFactor is a factor by which voter reward per committee is multiplied
	// to make calculations more precise.
	voterRewardFactor = 100_000_000
	// prefixGASPerBlock is a prefix for storing amount of GAS generated per block.
	prefixGASPerBlock = 29
	// prefixRegisterPrice is a prefix for storing candidate register price.
	prefixRegisterPrice = 13
	// effectiveVoterTurnout represents minimal ratio of total supply to total amount voted value
	// which is require to use non-standby validators.
	effectiveVoterTurnout = 5
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to NEO holders.
	neoHolderRewardRatio = 10
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to committee.
	committeeRewardRatio = 10
	// neoHolderRewardRatio is a percent of generated GAS that is distributed to voters.
	voterRewardRatio = 80
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
	defer n.UpdateHash()

	nep17 := newNEP17Native(nativenames.Neo, neoContractID)
	nep17.symbol = "NEO"
	nep17.decimals = 0
	nep17.factor = 1
	nep17.incBalance = n.increaseBalance
	nep17.balFromBytes = n.balanceFromBytes

	n.nep17TokenNative = *nep17
	n.votesChanged.Store(true)
	n.nextValidators.Store(keys.PublicKeys(nil))
	n.validators.Store(keys.PublicKeys(nil))
	n.committee.Store(keysWithVotes(nil))
	n.committeeHash.Store(util.Uint160{})
	n.registerPriceChanged.Store(true)
	n.gasPerVoteCache = make(map[string]big.Int)

	desc := newDescriptor("unclaimedGas", smartcontract.IntegerType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("end", smartcontract.IntegerType))
	md := newMethodAndPrice(n.unclaimedGas, 1<<17, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("registerCandidate", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.registerCandidate, 0, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("unregisterCandidate", smartcontract.BoolType,
		manifest.NewParameter("pubkey", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.unregisterCandidate, 1<<16, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("vote", smartcontract.BoolType,
		manifest.NewParameter("account", smartcontract.Hash160Type),
		manifest.NewParameter("voteTo", smartcontract.PublicKeyType))
	md = newMethodAndPrice(n.vote, 1<<16, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("getCandidates", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getCandidatesCall, 1<<22, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getAccountState", smartcontract.ArrayType,
		manifest.NewParameter("account", smartcontract.Hash160Type))
	md = newMethodAndPrice(n.getAccountState, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getCommittee", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getCommittee, 1<<16, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getNextBlockValidators", smartcontract.ArrayType)
	md = newMethodAndPrice(n.getNextBlockValidators, 1<<16, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("getGasPerBlock", smartcontract.IntegerType)
	md = newMethodAndPrice(n.getGASPerBlock, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("setGasPerBlock", smartcontract.VoidType,
		manifest.NewParameter("gasPerBlock", smartcontract.IntegerType))
	md = newMethodAndPrice(n.setGASPerBlock, 1<<15, callflag.States)
	n.AddMethod(md, desc)

	desc = newDescriptor("getRegisterPrice", smartcontract.IntegerType)
	md = newMethodAndPrice(n.getRegisterPrice, 1<<15, callflag.ReadStates)
	n.AddMethod(md, desc)

	desc = newDescriptor("setRegisterPrice", smartcontract.VoidType,
		manifest.NewParameter("registerPrice", smartcontract.IntegerType))
	md = newMethodAndPrice(n.setRegisterPrice, 1<<15, callflag.States)
	n.AddMethod(md, desc)

	return n
}

// Initialize initializes NEO contract.
func (n *NEO) Initialize(ic *interop.Context) error {
	if err := n.nep17TokenNative.Initialize(ic); err != nil {
		return err
	}

	if n.nep17TokenNative.getTotalSupply(ic.DAO).Sign() != 0 {
		return errors.New("already initialized")
	}

	committee := ic.Chain.GetStandByCommittee()
	cvs := toKeysWithVotes(committee)
	err := n.updateCache(cvs, ic.Chain)
	if err != nil {
		return err
	}

	err = ic.DAO.PutStorageItem(n.ID, prefixCommittee, cvs.Bytes())
	if err != nil {
		return err
	}

	h, err := getStandbyValidatorsHash(ic)
	if err != nil {
		return err
	}
	n.mint(ic, h, big.NewInt(NEOTotalSupply), false)

	var index uint32 = 0
	value := big.NewInt(5 * GASFactor)
	err = n.putGASRecord(ic.DAO, index, value)
	if err != nil {
		return err
	}
	gr := &gasRecord{{Index: index, GASPerBlock: *value}}
	n.gasPerBlock.Store(*gr)
	n.gasPerBlockChanged.Store(false)
	err = ic.DAO.PutStorageItem(n.ID, []byte{prefixVotersCount}, state.StorageItem{})
	if err != nil {
		return err
	}

	err = setIntWithKey(n.ID, ic.DAO, []byte{prefixRegisterPrice}, DefaultRegisterPrice)
	if err != nil {
		return err
	}
	n.registerPrice.Store(int64(DefaultRegisterPrice))
	n.registerPriceChanged.Store(false)
	return nil
}

// InitializeCache initializes all NEO cache with the proper values from storage.
// Cache initialisation should be done apart from Initialize because Initialize is
// called only when deploying native contracts.
func (n *NEO) InitializeCache(bc blockchainer.Blockchainer, d dao.DAO) error {
	var committee = keysWithVotes{}
	si := d.GetStorageItem(n.ID, prefixCommittee)
	if err := committee.DecodeBytes(si); err != nil {
		return err
	}
	if err := n.updateCache(committee, bc); err != nil {
		return err
	}

	gr, err := n.getSortedGASRecordFromDAO(d)
	if err != nil {
		return err
	}
	n.gasPerBlock.Store(gr)
	n.gasPerBlockChanged.Store(false)

	return nil
}

func (n *NEO) updateCache(cvs keysWithVotes, bc blockchainer.Blockchainer) error {
	n.committee.Store(cvs)

	var committee = n.GetCommitteeMembers()
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
		committee := n.committee.Load().(keysWithVotes)
		return ic.DAO.PutStorageItem(n.ID, prefixCommittee, committee.Bytes())
	}

	_, cvs, err := n.computeCommitteeMembers(ic.Chain, ic.DAO)
	if err != nil {
		return err
	}
	if err := n.updateCache(cvs, ic.Chain); err != nil {
		return err
	}
	n.votesChanged.Store(false)
	return ic.DAO.PutStorageItem(n.ID, prefixCommittee, cvs.Bytes())
}

// ShouldUpdateCommittee returns true if committee is updated at block h.
func ShouldUpdateCommittee(h uint32, bc blockchainer.Blockchainer) bool {
	cfg := bc.GetConfig()
	r := len(cfg.StandbyCommittee)
	return h%uint32(r) == 0
}

// OnPersist implements Contract interface.
func (n *NEO) OnPersist(ic *interop.Context) error {
	if ShouldUpdateCommittee(ic.Block.Index, ic.Chain) {
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
	committeeSize := len(ic.Chain.GetConfig().StandbyCommittee)
	index := int(ic.Block.Index) % committeeSize
	committeeReward := new(big.Int).Mul(gas, big.NewInt(committeeRewardRatio))
	n.GAS.mint(ic, pubs[index].GetScriptHash(), committeeReward.Div(committeeReward, big.NewInt(100)), false)

	if ShouldUpdateCommittee(ic.Block.Index, ic.Chain) {
		var voterReward = big.NewInt(voterRewardRatio)
		voterReward.Mul(voterReward, gas)
		voterReward.Mul(voterReward, big.NewInt(voterRewardFactor*int64(committeeSize)))
		var validatorsCount = ic.Chain.GetConfig().ValidatorsCount
		voterReward.Div(voterReward, big.NewInt(int64(committeeSize+validatorsCount)))
		voterReward.Div(voterReward, big.NewInt(100))

		var cs = n.committee.Load().(keysWithVotes)
		var key = make([]byte, 38)
		for i := range cs {
			if cs[i].Votes.Sign() > 0 {
				tmp := big.NewInt(1)
				if i < validatorsCount {
					tmp = big.NewInt(2)
				}
				tmp.Mul(tmp, voterReward)
				tmp.Div(tmp, cs[i].Votes)

				key = makeVoterKey([]byte(cs[i].Key), key)

				var r *big.Int
				if g, ok := n.gasPerVoteCache[cs[i].Key]; ok {
					r = &g
				} else {
					reward := n.getGASPerVote(ic.DAO, key[:34], ic.Block.Index+1)
					r = &reward[0]
				}
				tmp.Add(tmp, r)

				binary.BigEndian.PutUint32(key[34:], ic.Block.Index+1)
				n.gasPerVoteCache[cs[i].Key] = *tmp

				if err := ic.DAO.PutStorageItem(n.ID, key, bigint.ToBytes(tmp)); err != nil {
					return err
				}
			}
		}
	}
	if n.gasPerBlockChanged.Load().(bool) {
		gr, err := n.getSortedGASRecordFromDAO(ic.DAO)
		if err != nil {
			panic(err)
		}
		n.gasPerBlock.Store(gr)
		n.gasPerBlockChanged.Store(false)
	}

	if n.registerPriceChanged.Load().(bool) {
		p := getIntWithKey(n.ID, ic.DAO, []byte{prefixRegisterPrice})
		n.registerPrice.Store(p)
		n.registerPriceChanged.Store(false)
	}
	return nil
}

func (n *NEO) getGASPerVote(d dao.DAO, key []byte, index ...uint32) []big.Int {
	var max = make([]uint32, len(index))
	var reward = make([]big.Int, len(index))
	d.Seek(n.ID, key, func(k, v []byte) {
		if len(k) == 4 {
			num := binary.BigEndian.Uint32(k)
			for i, ind := range index {
				if max[i] < num && num <= ind {
					max[i] = num
					reward[i] = *bigint.FromBytes(v)
				}
			}
		}
	})
	return reward
}

func (n *NEO) increaseBalance(ic *interop.Context, h util.Uint160, si *state.StorageItem, amount *big.Int) error {
	acc, err := state.NEOBalanceStateFromBytes(*si)
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
		*si = acc.Bytes()
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
		*si = acc.Bytes()
	} else {
		*si = nil
	}
	return nil
}

func (n *NEO) balanceFromBytes(si *state.StorageItem) (*big.Int, error) {
	acc, err := state.NEOBalanceStateFromBytes(*si)
	if err != nil {
		return nil, err
	}
	return &acc.Balance, err
}

func (n *NEO) distributeGas(ic *interop.Context, h util.Uint160, acc *state.NEOBalanceState) error {
	if ic.Block == nil || ic.Block.Index == 0 {
		return nil
	}
	gen, err := n.calculateBonus(ic.DAO, acc.VoteTo, &acc.Balance, acc.BalanceHeight, ic.Block.Index)
	if err != nil {
		return err
	}
	acc.BalanceHeight = ic.Block.Index
	n.GAS.mint(ic, h, gen, true)
	return nil
}

func (n *NEO) unclaimedGas(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	u := toUint160(args[0])
	end := uint32(toBigInt(args[1]).Int64())
	gen, err := n.CalculateBonus(ic.DAO, u, end)
	if err != nil {
		panic(err)
	}
	return stackitem.NewBigInteger(gen)
}

func (n *NEO) getGASPerBlock(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	gas := n.GetGASPerBlock(ic.DAO, ic.Block.Index)
	return stackitem.NewBigInteger(gas)
}

func (n *NEO) getSortedGASRecordFromDAO(d dao.DAO) (gasRecord, error) {
	grMap, err := d.GetStorageItemsWithPrefix(n.ID, []byte{prefixGASPerBlock})
	if err != nil {
		return gasRecord{}, fmt.Errorf("failed to get gas records from storage: %w", err)
	}
	var (
		i  int
		gr = make(gasRecord, len(grMap))
	)
	for indexBytes, gasValue := range grMap {
		gr[i] = gasIndexPair{
			Index:       binary.BigEndian.Uint32([]byte(indexBytes)),
			GASPerBlock: *bigint.FromBytes(gasValue),
		}
		i++
	}
	sort.Slice(gr, func(i, j int) bool {
		return gr[i].Index < gr[j].Index
	})
	return gr, nil
}

// GetGASPerBlock returns gas generated for block with provided index.
func (n *NEO) GetGASPerBlock(d dao.DAO, index uint32) *big.Int {
	var (
		gr  gasRecord
		err error
	)
	if n.gasPerBlockChanged.Load().(bool) {
		gr, err = n.getSortedGASRecordFromDAO(d)
		if err != nil {
			panic(err)
		}
	} else {
		gr = n.gasPerBlock.Load().(gasRecord)
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

func (n *NEO) checkCommittee(ic *interop.Context) bool {
	ok, err := runtime.CheckHashedWitness(ic, n.GetCommitteeAddress())
	if err != nil {
		panic(err)
	}
	return ok
}

func (n *NEO) setGASPerBlock(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	gas := toBigInt(args[0])
	err := n.SetGASPerBlock(ic, ic.Block.Index+1, gas)
	if err != nil {
		panic(err)
	}
	return stackitem.Null{}
}

// SetGASPerBlock sets gas generated for blocks after index.
func (n *NEO) SetGASPerBlock(ic *interop.Context, index uint32, gas *big.Int) error {
	if gas.Sign() == -1 || gas.Cmp(big.NewInt(10*GASFactor)) == 1 {
		return errors.New("invalid value for GASPerBlock")
	}
	if !n.checkCommittee(ic) {
		return errors.New("invalid committee signature")
	}
	n.gasPerBlockChanged.Store(true)
	return n.putGASRecord(ic.DAO, index, gas)
}

func (n *NEO) getRegisterPrice(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	return stackitem.NewBigInteger(big.NewInt(n.getRegisterPriceInternal(ic.DAO)))
}

func (n *NEO) getRegisterPriceInternal(d dao.DAO) int64 {
	if !n.registerPriceChanged.Load().(bool) {
		return n.registerPrice.Load().(int64)
	}
	return getIntWithKey(n.ID, d, []byte{prefixRegisterPrice})
}

func (n *NEO) setRegisterPrice(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	price := toBigInt(args[0])
	if price.Sign() <= 0 || !price.IsInt64() {
		panic("invalid register price")
	}
	if !n.checkCommittee(ic) {
		panic("invalid committee signature")
	}

	err := setIntWithKey(n.ID, ic.DAO, []byte{prefixRegisterPrice}, price.Int64())
	if err != nil {
		panic(err)
	}
	n.registerPriceChanged.Store(true)
	return stackitem.Null{}
}

func (n *NEO) dropCandidateIfZero(d dao.DAO, pub *keys.PublicKey, c *candidate) (bool, error) {
	if c.Registered || c.Votes.Sign() != 0 {
		return false, nil
	}
	if err := d.DeleteStorageItem(n.ID, makeValidatorKey(pub)); err != nil {
		return true, err
	}

	var toRemove []string
	voterKey := makeVoterKey(pub.Bytes())
	d.Seek(n.ID, voterKey, func(k, v []byte) {
		toRemove = append(toRemove, string(k))
	})
	for i := range toRemove {
		if err := d.DeleteStorageItem(n.ID, []byte(toRemove[i])); err != nil {
			return true, err
		}
	}
	delete(n.gasPerVoteCache, string(voterKey))

	return true, nil
}

func makeVoterKey(pub []byte, prealloc ...[]byte) []byte {
	var key []byte
	if len(prealloc) != 0 {
		key = prealloc[0]
	} else {
		key = make([]byte, 34, 38)
	}
	key[0] = prefixVoterRewardPerCommittee
	copy(key[1:], pub)
	return key
}

// CalculateBonus calculates amount of gas generated for holding value NEO from start to end block
// and having voted for active committee member.
func (n *NEO) CalculateBonus(d dao.DAO, acc util.Uint160, end uint32) (*big.Int, error) {
	key := makeAccountKey(acc)
	si := d.GetStorageItem(n.ID, key)
	if si == nil {
		return nil, storage.ErrKeyNotFound
	}
	st, err := state.NEOBalanceStateFromBytes(si)
	if err != nil {
		return nil, err
	}
	return n.calculateBonus(d, st.VoteTo, &st.Balance, st.BalanceHeight, end)
}

func (n *NEO) calculateBonus(d dao.DAO, vote *keys.PublicKey, value *big.Int, start, end uint32) (*big.Int, error) {
	r, err := n.CalculateNEOHolderReward(d, value, start, end)
	if err != nil || vote == nil {
		return r, err
	}

	var key = makeVoterKey(vote.Bytes())
	var reward = n.getGASPerVote(d, key, start, end)
	var tmp = new(big.Int).Sub(&reward[1], &reward[0])
	tmp.Mul(tmp, value)
	tmp.Div(tmp, big.NewInt(voterRewardFactor))
	tmp.Add(tmp, r)
	return tmp, nil
}

// CalculateNEOHolderReward return GAS reward for holding `value` of NEO from start to end block.
func (n *NEO) CalculateNEOHolderReward(d dao.DAO, value *big.Int, start, end uint32) (*big.Int, error) {
	if value.Sign() == 0 || start >= end {
		return big.NewInt(0), nil
	} else if value.Sign() < 0 {
		return nil, errors.New("negative value")
	}
	var (
		gr  gasRecord
		err error
	)
	if !n.gasPerBlockChanged.Load().(bool) {
		gr = n.gasPerBlock.Load().(gasRecord)
	} else {
		gr, err = n.getSortedGASRecordFromDAO(d)
		if err != nil {
			return nil, err
		}
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
	if !ic.VM.AddGas(n.getRegisterPriceInternal(ic.DAO)) {
		panic("insufficient gas")
	}
	err = n.RegisterCandidateInternal(ic, pub)
	return stackitem.NewBool(err == nil)
}

// RegisterCandidateInternal registers pub as a new candidate.
func (n *NEO) RegisterCandidateInternal(ic *interop.Context, pub *keys.PublicKey) error {
	key := makeValidatorKey(pub)
	si := ic.DAO.GetStorageItem(n.ID, key)
	var c *candidate
	if si == nil {
		c = &candidate{Registered: true}
	} else {
		c = new(candidate).FromBytes(si)
		c.Registered = true
	}
	return ic.DAO.PutStorageItem(n.ID, key, c.Bytes())
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
	si := ic.DAO.GetStorageItem(n.ID, key)
	if si == nil {
		return nil
	}
	n.validators.Store(keys.PublicKeys(nil))
	c := new(candidate).FromBytes(si)
	c.Registered = false
	ok, err := n.dropCandidateIfZero(ic.DAO, pub, c)
	if ok {
		return err
	}
	return ic.DAO.PutStorageItem(n.ID, key, c.Bytes())
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
	si := ic.DAO.GetStorageItem(n.ID, key)
	if si == nil {
		return errors.New("invalid account")
	}
	acc, err := state.NEOBalanceStateFromBytes(si)
	if err != nil {
		return err
	}
	// we should put it in storage anyway as it affects dumps
	err = ic.DAO.PutStorageItem(n.ID, key, si)
	if err != nil {
		return err
	}
	if pub != nil {
		valKey := makeValidatorKey(pub)
		valSi := ic.DAO.GetStorageItem(n.ID, valKey)
		if valSi == nil {
			return errors.New("unknown validator")
		}
		cd := new(candidate).FromBytes(valSi)
		// we should put it in storage anyway as it affects dumps
		err = ic.DAO.PutStorageItem(n.ID, valKey, valSi)
		if err != nil {
			return err
		}
		if !cd.Registered {
			return errors.New("validator must be registered")
		}
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
	if err := n.distributeGas(ic, h, acc); err != nil {
		return err
	}
	if err := n.ModifyAccountVotes(acc, ic.DAO, new(big.Int).Neg(&acc.Balance), false); err != nil {
		return err
	}
	acc.VoteTo = pub
	if err := n.ModifyAccountVotes(acc, ic.DAO, &acc.Balance, true); err != nil {
		return err
	}
	return ic.DAO.PutStorageItem(n.ID, key, acc.Bytes())
}

// ModifyAccountVotes modifies votes of the specified account by value (can be negative).
// typ specifies if this modify is occurring during transfer or vote (with old or new validator).
func (n *NEO) ModifyAccountVotes(acc *state.NEOBalanceState, d dao.DAO, value *big.Int, isNewVote bool) error {
	n.votesChanged.Store(true)
	if acc.VoteTo != nil {
		key := makeValidatorKey(acc.VoteTo)
		si := d.GetStorageItem(n.ID, key)
		if si == nil {
			return errors.New("invalid validator")
		}
		cd := new(candidate).FromBytes(si)
		cd.Votes.Add(&cd.Votes, value)
		if !isNewVote {
			ok, err := n.dropCandidateIfZero(d, acc.VoteTo, cd)
			if ok {
				return err
			}
		}
		n.validators.Store(keys.PublicKeys(nil))
		return d.PutStorageItem(n.ID, key, cd.Bytes())
	}
	return nil
}

func (n *NEO) getCandidates(d dao.DAO, sortByKey bool) ([]keyWithVotes, error) {
	siMap, err := d.GetStorageItemsWithPrefix(n.ID, []byte{prefixCandidate})
	if err != nil {
		return nil, err
	}
	arr := make([]keyWithVotes, 0, len(siMap))
	for key, si := range siMap {
		c := new(candidate).FromBytes(si)
		if c.Registered {
			arr = append(arr, keyWithVotes{Key: key, Votes: &c.Votes})
		}
	}
	if sortByKey {
		// Sort by serialized key bytes (that's the way keys are stored and retrieved from the storage by default).
		sort.Slice(arr, func(i, j int) bool { return strings.Compare(arr[i].Key, arr[j].Key) == -1 })
	} else {
		sort.Slice(arr, func(i, j int) bool {
			// The most-voted validators should end up in the front of the list.
			cmp := arr[i].Votes.Cmp(arr[j].Votes)
			if cmp != 0 {
				return cmp > 0
			}
			// Ties are broken with deserialized public keys.
			// Sort by ECPoint's (X, Y) components: compare X first, and then compare Y.
			cmpX := strings.Compare(arr[i].Key[1:], arr[j].Key[1:])
			if cmpX != 0 {
				return cmpX == -1
			}
			// The case when X components are the same is extremely rare, thus we perform
			// key deserialization only if needed. No error can occur.
			ki, _ := keys.NewPublicKeyFromBytes([]byte(arr[i].Key), elliptic.P256())
			kj, _ := keys.NewPublicKeyFromBytes([]byte(arr[j].Key), elliptic.P256())
			return ki.Y.Cmp(kj.Y) == -1
		})
	}
	return arr, nil
}

// GetCandidates returns current registered validators list with keys
// and votes.
func (n *NEO) GetCandidates(d dao.DAO) ([]state.Validator, error) {
	kvs, err := n.getCandidates(d, true)
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
	validators, err := n.getCandidates(ic.DAO, true)
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

func (n *NEO) getAccountState(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := makeAccountKey(toUint160(args[0]))
	si := ic.DAO.GetStorageItem(n.ID, key)
	if len(si) == 0 {
		return stackitem.Null{}
	}

	item, err := stackitem.Deserialize(si)
	if err != nil {
		panic(err) // no errors are expected but we better be sure
	}
	return item
}

// ComputeNextBlockValidators returns an actual list of current validators.
func (n *NEO) ComputeNextBlockValidators(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, error) {
	if vals := n.validators.Load().(keys.PublicKeys); vals != nil {
		return vals.Copy(), nil
	}
	result, _, err := n.computeCommitteeMembers(bc, d)
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
	si := d.GetStorageItem(n.ID, key)
	if si == nil {
		return errors.New("voters count not found")
	}
	votersCount := bigint.FromBytes(si)
	votersCount.Add(votersCount, amount)
	si = bigint.ToPreallocatedBytes(votersCount, si)
	return d.PutStorageItem(n.ID, key, si)
}

// GetCommitteeMembers returns public keys of nodes in committee using cached value.
func (n *NEO) GetCommitteeMembers() keys.PublicKeys {
	var cvs = n.committee.Load().(keysWithVotes)
	var committee = make(keys.PublicKeys, len(cvs))
	var err error
	for i := range committee {
		committee[i], err = cvs[i].PublicKey()
		if err != nil {
			panic(err)
		}
	}
	return committee
}

func toKeysWithVotes(pubs keys.PublicKeys) keysWithVotes {
	ks := make(keysWithVotes, len(pubs))
	for i := range pubs {
		ks[i].UnmarshaledKey = pubs[i]
		ks[i].Key = string(pubs[i].Bytes())
		ks[i].Votes = big.NewInt(0)
	}
	return ks
}

// computeCommitteeMembers returns public keys of nodes in committee.
func (n *NEO) computeCommitteeMembers(bc blockchainer.Blockchainer, d dao.DAO) (keys.PublicKeys, keysWithVotes, error) {
	key := []byte{prefixVotersCount}
	si := d.GetStorageItem(n.ID, key)
	if si == nil {
		return nil, nil, errors.New("voters count not found")
	}
	votersCount := bigint.FromBytes(si)
	// votersCount / totalSupply must be >= 0.2
	votersCount.Mul(votersCount, big.NewInt(effectiveVoterTurnout))
	voterTurnout := votersCount.Div(votersCount, n.getTotalSupply(d))

	sbVals := bc.GetStandByCommittee()
	count := len(sbVals)
	cs, err := n.getCandidates(d, false)
	if err != nil {
		return nil, nil, err
	}
	if voterTurnout.Sign() != 1 || len(cs) < count {
		kvs := make(keysWithVotes, count)
		for i := range kvs {
			kvs[i].UnmarshaledKey = sbVals[i]
			kvs[i].Key = string(sbVals[i].Bytes())
			votes := big.NewInt(0)
			for j := range cs {
				if cs[j].Key == kvs[i].Key {
					votes = cs[j].Votes
					break
				}
			}
			kvs[i].Votes = votes
		}
		return sbVals, kvs, nil
	}
	pubs := make(keys.PublicKeys, count)
	for i := range pubs {
		pubs[i], err = cs[i].PublicKey()
		if err != nil {
			return nil, nil, err
		}
	}
	return pubs, cs[:count], nil
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

// putGASRecord is a helper which creates key and puts GASPerBlock value into the storage.
func (n *NEO) putGASRecord(dao dao.DAO, index uint32, value *big.Int) error {
	key := make([]byte, 5)
	key[0] = prefixGASPerBlock
	binary.BigEndian.PutUint32(key[1:], index)
	return dao.PutStorageItem(n.ID, key, bigint.ToBytes(value))
}
